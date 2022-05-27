package dbranch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path"
	"strings"
	"time"

	ipfs "github.com/ipfs/go-ipfs-api"
)

var shell *ipfs.Shell

const CuratedDir = "/dBranch/curated"
const IndexFile = "/dBranch/index.json"

func init() {
	host := os.Getenv("IPFS_HOST")
	if host == "" {
		host = "localhost:5001"
	}
	shell = ipfs.NewShell(host)

}

//
// article models
//

type Article struct {
	Metadata *ArticleMetadata       `json:"metadata"`
	Contents map[string]interface{} `json:"contents"`
	Record   *ArticleRecord         `json:"record,omitempty"`
}

type ArticleMetadata struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	SubTitle string `json:"sub_title"`
	Author   string `json:"author"`
}

type ArticleRecord struct {
	Name          string    `json:"name"`
	Size          uint64    `json:"size"`
	CID           string    `json:"cid"`
	DateAdded     time.Time `json:"date_added"`                // date curated in UTC
	DatePublished time.Time `json:"date_published"`            // publish date in UTC
	CardanoTxHash string    `json:"cardano_tx_hash,omitempty"` // cardano transaction id
}

type ArticleIndexItem struct {
	Record   *ArticleRecord   `json:"record"`
	Metadata *ArticleMetadata `json:"metadata"`
}

type ArticleIndex struct {
	Articles []*ArticleIndexItem `json:"articles"`
}

func loadArticleRecord(name string) (*ArticleRecord, error) {
	// init
	record := &ArticleRecord{}
	record_path := path.Join(CuratedDir, record.Name) + ".json"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// read and decode record
	record_raw, err := shell.FilesRead(ctx, record_path, ipfs.FilesLs.Stat(true))
	if err != nil {
		return record, err
	}

	err = json.NewDecoder(record_raw).Decode(&record)
	if err != nil {
		return record, err
	}

	return record, nil
}

func loadArticle(name string) (*Article, error) {
	// init
	article := &Article{}
	article_path := path.Join(CuratedDir, name)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// read and decode article
	article_raw, err := shell.FilesRead(ctx, article_path, ipfs.FilesLs.Stat(true))
	if err != nil {
		return article, err
	}

	err = json.NewDecoder(article_raw).Decode(&article)
	return article, err
}

func GetArticle(name string) (*Article, error) {
	article, err := loadArticle(name)
	if err != nil {
		return nil, err
	}

	record, err := loadArticleRecord(name)
	if err != nil {
		return nil, err
	}

	article.Record = record
	return article, nil
}

func ListArticles() ([]string, error) {
	// init
	list := []string{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// get listing
	ls, err := shell.FilesLs(ctx, CuratedDir)
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

func AddRecordToLocal(record *ArticleRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ipfs_source := path.Join("/ipfs", record.CID)
	article_path := path.Join(CuratedDir, record.Name)
	record_path := article_path + ".json"

	//
	// copy article to ipfs files (mfs)
	//

	ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := shell.FilesCp(ctx, ipfs_source, article_path)
	if err != nil {
		return err
	}

	// pin article because FilesCp does not copy the entire contents of the file, just the root node of the DAG
	err = shell.Pin(record.CID)
	if err != nil {
		return err
	}

	//
	// set meteadata
	//

	stat, err := shell.FilesStat(ctx, article_path)
	if err != nil {
		return err
	}

	record.Size = stat.Size
	record.DateAdded = time.Now().UTC()

	//
	// write article record / metadata
	//

	mashalled_record, err := json.Marshal(record)
	if err != nil {
		return err
	}

	json_reader := bytes.NewReader(mashalled_record)
	err = shell.FilesWrite(ctx, record_path, json_reader)
	if err != nil {
		return err
	}

	// refresh article index
	err = RefreshArticleIndex()
	if err != nil {
		return err
	}

	log.Println("article successfully added: " + record.Name)
	return nil
}

func RemoveRecordFromLocal(name string) error {
	log.Printf("removing article: %s\n", name)

	// init
	article_path := path.Join(CuratedDir, name)
	record_path := article_path + ".json"

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// delete files
	err := shell.FilesRm(ctx, record_path, true)
	if err != nil {
		return err
	}

	err = shell.FilesRm(ctx, article_path, true)
	if err != nil {
		return err
	}

	log.Printf("removed article: %s\n", name)

	return RefreshArticleIndex()
}

//
// index
//

func GenerateArticleIndex() (*ArticleIndex, error) {
	// init
	names, err := ListArticles()
	if err != nil {
		return nil, err
	}

	index := &ArticleIndex{}

	for _, name := range names {
		article, err := GetArticle(name)
		if err != nil {
			return index, err
		}
		item := &ArticleIndexItem{Record: article.Record, Metadata: article.Metadata}
		index.Articles = append(index.Articles, item)
	}

	return index, nil
}

func LoadArticleIndex() (*ArticleIndex, error) {
	index := &ArticleIndex{}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	content, err := shell.FilesRead(ctx, IndexFile)
	if err != nil {
		return index, err
	}

	err = json.NewDecoder(content).Decode(&index)
	if err != nil {
		return index, err
	}

	return index, nil
}

func WriteArticleIndex(index *ArticleIndex) error {

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	output := new(bytes.Buffer)
	err := json.NewEncoder(output).Encode(index)
	if err != nil {
		return errors.New("failed to encode article index: " + err.Error())
	}

	err = shell.FilesWrite(ctx, IndexFile, output, ipfs.FilesWrite.Create(true), ipfs.FilesWrite.Truncate(true))
	if err != nil {
		return errors.New("failed to write article index: " + err.Error())
	}

	return nil
}

func RefreshArticleIndex() error {
	index, err := GenerateArticleIndex()
	if err != nil {
		return err
	}

	err = WriteArticleIndex(index)
	if err != nil {
		return err
	}

	log.Println("refreshed article index")
	return nil
}
