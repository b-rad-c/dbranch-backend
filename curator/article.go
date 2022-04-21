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

const indexFile = "index.json"

type ArticleMetadata struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	SubTitle string `json:"sub_title"`
	Author   string `json:"author"`
}

type Article struct {
	Metadata ArticleMetadata        `json:"metadata"`
	Contents map[string]interface{} `json:"contents"`
}

type ArticleListItem struct {
	Name     string          `json:"name"`
	Size     uint64          `json:"size"`
	Hash     string          `json:"hash"`
	Metadata ArticleMetadata `json:"metadata"`
}

type ArticleList struct {
	Items []*ArticleListItem `json:"items"`
}

type ArticleIndexItem struct {
	Name          string          `json:"name"`
	Size          uint64          `json:"size"`
	Hash          string          `json:"hash"`
	TransactionID string          `json:"transaction_id,omitempty"`
	Date          time.Time       `json:"date"` // publish date in UTC
	Metadata      ArticleMetadata `json:"metadata"`
}

type ArticleIndex struct {
	Items []*ArticleIndexItem `json:"items"`
}

type IncomingArticle struct {
	Name          string `json:"name"`
	CID           string `json:"cid"`
	TransactionID string `json:"transaction_id,omitempty"`
}

//
// curator article actions
//

func (c *Curator) LoadArticleIndex() (ArticleIndex, error) {
	index := ArticleIndex{}

	indexPath := path.Join(c.Config.CuratedDir, indexFile)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	content, err := c.Shell.FilesRead(ctx, indexPath)
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

	indexPath := path.Join(c.Config.CuratedDir, indexFile)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	output := new(bytes.Buffer)
	err := json.NewEncoder(output).Encode(index)
	if err != nil {
		return errors.New("failed to encode article index: " + err.Error())
	}

	err = c.Shell.FilesWrite(ctx, indexPath, output, ipfs.FilesWrite.Create(true), ipfs.FilesWrite.Truncate(true))
	if err != nil {
		return errors.New("failed to write article index: " + err.Error())
	}

	return nil
}

func (c *Curator) addToCuratedIndex(in *IncomingArticle) error {
	current_index, err := c.LoadArticleIndex()
	if err != nil {
		// ignore not found error, will create a blank index
		if err.Error() != "files/read: file does not exist" {
			return err
		}
	}

	// filter index to prevent duplicate entries (this incoming article may replace an existing article version)
	new_index := ArticleIndex{}
	for _, article := range current_index.Items {
		if article.Name != in.Name {
			new_index.Items = append(new_index.Items, article)
		}
	}

	// load and stat article to get metadata
	articlePath := path.Join(c.Config.CuratedDir, in.Name)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stat, err := c.Shell.FilesStat(ctx, articlePath)

	if err != nil {
		return err
	}

	articleContents, err := c.GetArticle(in.Name)
	if err != nil {
		return err
	}

	// add new article to index
	local_date := time.Now()
	utc_date := local_date.UTC()
	log.Printf("local date: %v, utc date: %v", local_date, utc_date)

	new_index.Items = append(new_index.Items, &ArticleIndexItem{
		Name:          in.Name,
		Size:          stat.Size,
		Hash:          in.CID,
		Date:          utc_date,
		TransactionID: in.TransactionID,
		Metadata:      articleContents.Metadata,
	})

	return c.WriteArticleIndex(new_index)
}

// process incoming article on wire
func (c *Curator) AddToCurated(in *IncomingArticle) error {
	// init
	ipfsPath := path.Join("/ipfs", in.CID)
	localPath := path.Join(c.Config.CuratedDir, in.Name)

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
		log.Printf("stat hash: %s\n article cid: %s", stat.Hash, in.CID)

		if stat.Hash == in.CID {
			// if we already have the same hash in our curated dir, don't copy
			log.Printf("already have article: %s with hash %s\n", in.Name, in.CID)
			return nil
		} else {
			// hash is different, replace existing article
			log.Printf("replacing article: %s with newer hash %s\n", in.Name, in.CID)
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
	if err = c.Shell.Pin(in.CID); err != nil {
		return err
	}

	// update article index
	log.Println("adding article to curated index")
	err = c.addToCuratedIndex(in)
	if err != nil {
		return err
	}

	log.Println("article successfully added to curated list")
	return nil
}

func (c *Curator) RemoveFromCurated(name string) error {
	// remove article from curated index
	index, err := c.LoadArticleIndex()
	if err != nil {
		return errors.New("failed to load curated index: " + err.Error())
	}

	for i, article := range index.Items {
		if article.Name == name {
			index.Items = append(index.Items[:i], index.Items[i+1:]...)
			break
		}
	}

	err = c.WriteArticleIndex(index)
	if err != nil {
		return errors.New("failed to write curated index: " + err.Error())
	}

	// delete local copy
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	articlePath := path.Join(c.Config.CuratedDir, name)
	return c.Shell.FilesRm(ctx, articlePath, true)
}

func (c *Curator) ListArticles() (*ArticleList, error) {
	// init
	list := &ArticleList{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// get listing
	ls, err := c.Shell.FilesLs(ctx, c.Config.CuratedDir, ipfs.FilesLs.Stat(true))
	if err != nil {
		return list, err
	}

	// get metadata for each listing
	for _, entry := range ls {
		if entry.Name == indexFile {
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

		list.Items = append(list.Items, &ArticleListItem{
			Name:     entry.Name,
			Size:     entry.Size,
			Hash:     entry.Hash,
			Metadata: article.Metadata,
		})
	}

	return list, nil
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
