package curator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"path"
	"time"

	ipfs "github.com/ipfs/go-ipfs-api"
)

//
// article models
//

type Article struct {
	Metadata ArticleMetadata        `json:"metadata"`
	Contents map[string]interface{} `json:"contents"`
}

type ArticleMetadata struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	SubTitle string `json:"sub_title"`
	Author   string `json:"author"`
}

type ArticleRecord struct {
	Name          string          `json:"name"`
	Size          uint64          `json:"size"`
	CID           string          `json:"cid"`
	DateAdded     time.Time       `json:"date_added"`               // date curated in UTC
	DatePublished time.Time       `json:"date_published"`           // publish date in UTC
	TransactionID string          `json:"transaction_id,omitempty"` // cardano transaction id
	Metadata      ArticleMetadata `json:"metadata"`                 // article metadata (cached from .news file)
}

type ArticleRecordType int

const (
	Curated ArticleRecordType = iota
	Published
)

type ArticleRecordList struct {
	Items []*ArticleRecord `json:"records"`
}

type ArticleIndex struct {
	Curated   []*ArticleRecord `json:"curated"`
	Published []*ArticleRecord `json:"published"`
}

func (c *Curator) GetArticle(name string) (*Article, error) {
	// init
	article := &Article{}
	articlePath := path.Join(c.Config.CuratedDir, name)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// read
	content, err := c.Shell.FilesRead(ctx, articlePath, ipfs.FilesLs.Stat(true))
	if err != nil {
		return article, err
	}

	// decode
	err = json.NewDecoder(content).Decode(&article)
	if err != nil {
		return article, err
	}

	return article, nil
}

func (c *Curator) ListArticles() (*ArticleRecordList, error) {
	// init
	list := &ArticleRecordList{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// get listing
	ls, err := c.Shell.FilesLs(ctx, c.Config.CuratedDir, ipfs.FilesLs.Stat(true))
	if err != nil {
		return list, err
	}

	// get metadata for each listing
	for _, entry := range ls {
		if entry.Name == "index.json" {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		articlePath := path.Join(c.Config.CuratedDir, entry.Name)
		content, err := c.Shell.FilesRead(ctx, articlePath)
		if err != nil {
			return list, err
		}

		article := Article{}
		err = json.NewDecoder(content).Decode(&article)
		if err != nil {
			return list, err
		}

		list.Items = append(list.Items, &ArticleRecord{
			Name:     entry.Name,
			Size:     entry.Size,
			CID:      entry.Hash,
			Metadata: article.Metadata,
		})
	}

	return list, nil
}

func (c *Curator) AddRecordToLocal(record *ArticleRecord, record_type ArticleRecordType) error {
	// init
	ipfsPath := path.Join("/ipfs", record.CID)

	var localPath string
	if record_type == Curated {
		localPath = path.Join(c.Config.CuratedDir, record.Name)
	} else {
		localPath = path.Join(c.Config.PublishedDir, record.Name)
	}

	// stat file to determine if we have it already and how to proceed
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stat, err := c.Shell.FilesStat(ctx, localPath)

	// not found error is ok, we will add the article
	if err != nil && err.Error() != "files/stat: file does not exist" {
		return err
	}

	// we already have a matching article name, determine how to proceed
	if stat != nil {
		log.Printf("stat hash: %s\n article cid: %s", stat.Hash, record.CID)

		if stat.Hash == record.CID {
			// if we already have the same hash in our curated dir, don't copy
			log.Printf("already have article: %s with hash %s\n", record.Name, record.CID)
			return nil
		} else {
			// hash is different, replace existing article
			log.Printf("replacing article: %s with newer hash %s\n", record.Name, record.CID)
			ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			err = c.Shell.FilesRm(ctx, localPath, true)
			if err != nil {
				return err
			}
		}
	}

	// copy article to ipfs files (mfs)
	log.Printf("copying article from: %s to: %s\n", ipfsPath, localPath)
	ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err = c.Shell.FilesCp(ctx, ipfsPath, localPath); err != nil {
		return err
	}

	// pin article, this is necessary to store the entire file contents locally
	// because FilesCp does not copy the entire contents of the file, just the root node of the DAG
	log.Println("pinning article to local node")
	if err = c.Shell.Pin(record.CID); err != nil {
		return err
	}

	// update article index
	log.Println("adding article to index")
	err = c.addToIndex(record, record_type)
	if err != nil {
		return err
	}

	log.Println("article successfully added")
	return nil
}

func (c *Curator) RemoveRecordFromLocal(name string, record_type ArticleRecordType) error {
	// remove an article from the index and the ipfs mfs

	//
	// init
	//

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	index, err := c.LoadArticleIndex()
	if err != nil {
		return errors.New("failed to load curated index: " + err.Error())
	}

	//
	// filter index
	//

	var source_list []*ArticleRecord
	if record_type == Curated {
		source_list = index.Curated
	} else {
		source_list = index.Published
	}

	for i, article := range source_list {
		if article.Name == name {
			source_list = append(index.Curated[:i], index.Curated[i+1:]...)
			break
		}
	}

	var articlePath string
	if record_type == Curated {
		index.Curated = source_list
		articlePath = path.Join(c.Config.CuratedDir, name)
	} else {
		index.Published = source_list
		articlePath = path.Join(c.Config.PublishedDir, name)
	}

	err = c.WriteArticleIndex(index)
	if err != nil {
		return errors.New("failed to write modified index: " + err.Error())
	}

	return c.Shell.FilesRm(ctx, articlePath, true)

}

//
// index
//

func (c *Curator) GenerateArticleIndex() error {
	/*
		generate index from all wallet transactions + all articles in curated folder
	*/
	return nil
}

func (c *Curator) LoadArticleIndex() (ArticleIndex, error) {
	index := ArticleIndex{}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	content, err := c.Shell.FilesRead(ctx, c.Config.Index)
	if err != nil {
		return index, err
	}

	err = json.NewDecoder(content).Decode(&index)
	if err != nil {
		return index, err
	}

	return index, nil
}

func (c *Curator) WriteArticleIndex(index ArticleIndex) error {

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	output := new(bytes.Buffer)
	err := json.NewEncoder(output).Encode(index)
	if err != nil {
		return errors.New("failed to encode article index: " + err.Error())
	}

	err = c.Shell.FilesWrite(ctx, c.Config.Index, output, ipfs.FilesWrite.Create(true), ipfs.FilesWrite.Truncate(true))
	if err != nil {
		return errors.New("failed to write article index: " + err.Error())
	}

	return nil
}

func (c *Curator) addToIndex(record *ArticleRecord, record_type ArticleRecordType) error {
	index, err := c.LoadArticleIndex()
	if err != nil {
		// ignore not found error, will create a blank index
		if err.Error() != "files/read: file does not exist" {
			return err
		}
	}

	// filter index to prevent duplicate entries (this incoming article may replace an existing article version)
	var source_list []*ArticleRecord
	var articlePath string
	if record_type == Curated {
		source_list = index.Curated
		articlePath = path.Join(c.Config.CuratedDir, record.Name)
	} else {
		source_list = index.Published
		articlePath = path.Join(c.Config.PublishedDir, record.Name)
	}

	new_index := ArticleIndex{}
	for _, article := range source_list {
		if article.Name != record.Name {
			source_list = append(source_list, article)
		}
	}

	// load and stat article to get metadata
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stat, err := c.Shell.FilesStat(ctx, articlePath)

	if err != nil {
		return err
	}

	articleContents, err := c.GetArticle(record.Name)
	if err != nil {
		return err
	}

	// add new article to index
	local_date := time.Now()
	utc_date := local_date.UTC()
	log.Printf("local date: %v, utc date: %v", local_date, utc_date)

	new_item := &ArticleRecord{
		Name:          record.Name,
		Size:          stat.Size,
		CID:           record.CID,
		DateAdded:     utc_date,
		TransactionID: record.TransactionID,
		Metadata:      articleContents.Metadata,
	}

	if record_type == Curated {
		index.Curated = append(source_list, new_item)

	} else {
		index.Published = append(source_list, new_item)

	}

	return c.WriteArticleIndex(new_index)
}
