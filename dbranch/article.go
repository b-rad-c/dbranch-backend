package dbranch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"path"
	"strings"
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

type FullArticle struct {
	Article *Article       `json:"article"`
	Record  *ArticleRecord `json:"record"`
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
	DateAdded     time.Time       `json:"date_added"`                // date curated in UTC
	DatePublished time.Time       `json:"date_published"`            // publish date in UTC
	CardanoTxHash string          `json:"cardano_tx_hash,omitempty"` // cardano transaction id
	Metadata      ArticleMetadata `json:"metadata"`                  // article metadata (cached from .news file)
}

type ArticleIndex struct {
	Articles []*ArticleRecord `json:"articles"`
}

func (c *Config) loadArticleRecord(name string) (*ArticleRecord, error) {
	// init
	record := &ArticleRecord{}
	record_path := path.Join(c.CuratedDir, record.Name) + ".json"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// read and decode record
	record_raw, err := c.ipfsShell().FilesRead(ctx, record_path, ipfs.FilesLs.Stat(true))
	if err != nil {
		return record, err
	}

	err = json.NewDecoder(record_raw).Decode(&record)
	if err != nil {
		return record, err
	}

	return record, nil
}

func (c *Config) loadArticle(name string) (*Article, error) {
	// init
	article := &Article{}
	article_path := path.Join(c.CuratedDir, name)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// read and decode article
	article_raw, err := c.ipfsShell().FilesRead(ctx, article_path, ipfs.FilesLs.Stat(true))
	if err != nil {
		return article, err
	}

	err = json.NewDecoder(article_raw).Decode(&article)
	return article, err
}

func (c *Config) GetArticle(name string) (*ArticleRecord, *Article, error) {
	record, err := c.loadArticleRecord(name)
	if err != nil {
		return nil, nil, err
	}

	article, err := c.loadArticle(name)
	if err != nil {
		return nil, nil, err
	}

	return record, article, nil
}

func (c *Config) ListArticles() ([]string, error) {
	// init
	list := []string{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// get listing
	ls, err := c.ipfsShell().FilesLs(ctx, c.CuratedDir)
	if err != nil {
		return list, err
	}

	// get metadata for each listing
	for _, entry := range ls {
		if strings.HasSuffix(entry.Name, ".news") {
			list = append(list, entry.Name)
		}
	}

	return list, nil
}

func (c *Config) AddRecordToLocal(record *ArticleRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ipfs_source := path.Join("/ipfs", record.CID)
	article_path := path.Join(c.CuratedDir, record.Name)
	record_path := article_path + ".json"

	//
	// copy article to ipfs files (mfs)
	//

	ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := c.ipfsShell().FilesCp(ctx, ipfs_source, article_path)
	if err != nil {
		return err
	}

	// pin article because FilesCp does not copy the entire contents of the file, just the root node of the DAG
	err = c.ipfsShell().Pin(record.CID)
	if err != nil {
		return err
	}

	//
	// get meteadata
	//

	stat, err := c.ipfsShell().FilesStat(ctx, article_path)
	if err != nil {
		return err
	}

	article, err := c.loadArticle(record.Name)
	if err != nil {
		return err
	}

	record.Size = stat.Size
	record.DateAdded = time.Now().UTC()
	record.Metadata = article.Metadata

	//
	// write article record / metadata
	//

	mashalled_record, err := json.Marshal(record)
	if err != nil {
		return err
	}

	json_reader := bytes.NewReader(mashalled_record)
	err = c.ipfsShell().FilesWrite(ctx, record_path, json_reader)
	if err != nil {
		return err
	}

	// refresh article index
	err = c.RefreshArticleIndex()
	if err != nil {
		return err
	}

	log.Println("article successfully added: " + record.Name)
	return nil
}

func (c *Config) RemoveRecordFromLocal(name string) error {
	log.Printf("removing article: %s\n", name)

	// init
	article_path := path.Join(c.CuratedDir, name)
	record_path := article_path + ".json"

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// delete files
	err := c.ipfsShell().FilesRm(ctx, record_path, true)
	if err != nil {
		return err
	}

	err = c.ipfsShell().FilesRm(ctx, article_path, true)
	if err != nil {
		return err
	}

	log.Printf("removed article: %s\n", name)

	return c.RefreshArticleIndex()
}

//
// index
//

func (c *Config) GenerateArticleIndex() (*ArticleIndex, error) {
	// init
	names, err := c.ListArticles()
	if err != nil {
		return nil, err
	}

	index := &ArticleIndex{}

	for _, name := range names {
		record, err := c.loadArticleRecord(name)
		if err != nil {
			return index, err
		}
		index.Articles = append(index.Articles, record)
	}

	return index, nil
}

func (c *Config) LoadArticleIndex() (*ArticleIndex, error) {
	index := &ArticleIndex{}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	content, err := c.ipfsShell().FilesRead(ctx, c.Index)
	if err != nil {
		return index, err
	}

	err = json.NewDecoder(content).Decode(&index)
	if err != nil {
		return index, err
	}

	return index, nil
}

func (c *Config) WriteArticleIndex(index *ArticleIndex) error {

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	output := new(bytes.Buffer)
	err := json.NewEncoder(output).Encode(index)
	if err != nil {
		return errors.New("failed to encode article index: " + err.Error())
	}

	err = c.ipfsShell().FilesWrite(ctx, c.Index, output, ipfs.FilesWrite.Create(true), ipfs.FilesWrite.Truncate(true))
	if err != nil {
		return errors.New("failed to write article index: " + err.Error())
	}

	return nil
}

func (c *Config) RefreshArticleIndex() error {
	index, err := c.GenerateArticleIndex()
	if err != nil {
		return err
	}

	err = c.WriteArticleIndex(index)
	if err != nil {
		return err
	}

	log.Println("refreshed article index")
	return nil
}
