package dbranch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	ipfs "github.com/ipfs/go-ipfs-api"
)

var shell *ipfs.Shell

const CuratedDir = "/dBranch/curated"
const PublishedDir = "/dBranch/published"
const IndexFile = "/dBranch/index.json"

func init() {
	host := os.Getenv("IPFS_HOST")
	if host == "" {
		host = "localhost:5001"
	}
	shell = ipfs.NewShell(host)
	shell.SetTimeout(15 * time.Second)
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
	CuratedArticles   []*ArticleIndexItem `json:"curated"`
	PublishedArticles []*ArticleIndexItem `json:"published"`
}

func statIpfsPath(path string) (*ipfs.FilesStatObject, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return shell.FilesStat(ctx, path)
}

func loadArticleRecord(path string) (*ArticleRecord, error) {
	// init
	record := &ArticleRecord{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// read and decode record
	record_raw, err := shell.FilesRead(ctx, path, ipfs.FilesLs.Stat(true))
	if err != nil {
		return record, err
	}

	err = json.NewDecoder(record_raw).Decode(&record)
	if err != nil {
		return record, err
	}

	return record, nil
}

func loadArticle(path string) (*Article, error) {
	// init
	article := &Article{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// read and decode article
	article_raw, err := shell.FilesRead(ctx, path, ipfs.FilesLs.Stat(true))
	if err != nil {
		return article, err
	}

	err = json.NewDecoder(article_raw).Decode(&article)
	return article, err
}

func GetArticleByMFSPath(path string) (*Article, error) {
	/* get an article and record by MFS path */
	article, err := loadArticle(path)
	if err != nil {
		return nil, err
	}

	record, err := loadArticleRecord(path + ".json")
	if err != nil {
		return nil, err
	}

	article.Record = record
	return article, nil
}

func cidIsPinned(cid string) (bool, error) {
	if strings.HasPrefix(cid, "/ipfs/") {
		cid = cid[6:]
	}

	pins, err := shell.Pins()
	if err != nil {
		return false, err
	}

	_, pinned := pins[cid]

	return pinned, nil
}

func GetArticleByCID(ipfs_path string) (*Article, error) {
	/*
		get an article (w/o record) by IPFS path
			examples:
				/ipfs/<cid>
				<cid>
	*/

	// check if pinned because the Cat command will search the network if it is not local, potentially taking a while
	pinned, err := cidIsPinned(ipfs_path)
	if err != nil {
		return nil, err
	}

	if !pinned {
		return nil, errors.New("article not found")
	}

	if !strings.HasPrefix(ipfs_path, "/ipfs/") {
		ipfs_path = "/ipfs/" + ipfs_path
	}

	resp, err := shell.Cat(ipfs_path)
	if err != nil {
		return nil, err
	}
	defer resp.Close()

	article := &Article{}
	err = json.NewDecoder(resp).Decode(&article)

	return article, nil
}

func listArticles(path string) ([]string, error) {
	// init
	list := []string{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// get listing
	ls, err := shell.FilesLs(ctx, path)
	if err != nil {
		return list, err
	}

	// filter non .news files (ex: record files w .json extension)
	for _, entry := range ls {
		if strings.HasSuffix(entry.Name, ".news") {
			list = append(list, entry.Name)
		}
	}

	return list, nil
}

func AddRecordToLocal(directory string, record *ArticleRecord, copy_article bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ipfs_source := path.Join("/ipfs", record.CID)
	article_path := path.Join(directory, record.Name)
	record_path := article_path + ".json"

	//
	// optionally copy article to ipfs files (mfs)
	//

	if copy_article {

		ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := shell.FilesCp(ctx, ipfs_source, article_path)
		if err != nil {
			return err
		}

		log.Printf("copied article: %s to: %s\n", ipfs_source, article_path)

		// pin article because FilesCp does not copy the entire contents of the file, just the root node of the DAG
		err = shell.Pin(record.CID)
		if err != nil {
			return err
		}

		log.Printf("pinned CID: %s\n", record.CID)
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
	err = shell.FilesWrite(ctx, record_path, json_reader, ipfs.FilesWrite.Create(true))
	if err != nil {
		return errors.New("Error writing article record to: " + record_path + ": " + err.Error())
	}

	log.Printf("wrote artricle record to: %s\n", record_path)

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

func NewArticleIndex() *ArticleIndex {
	return &ArticleIndex{CuratedArticles: []*ArticleIndexItem{}, PublishedArticles: []*ArticleIndexItem{}}
}

func GenerateArticleIndex() (*ArticleIndex, error) {

	index := NewArticleIndex()

	paths := []string{CuratedDir, PublishedDir}
	for _, directory := range paths {
		names, err := listArticles(directory)
		if err != nil {
			return nil, err
		}

		for _, name := range names {

			article, err := GetArticleByMFSPath(path.Join(directory, name))
			if err != nil {
				if err.Error() == "files/read: file does not exist" {
					// record does not exist for a published article (ie. is hasn't been signed yet)
					continue
				} else {
					return nil, err
				}
			}

			item := &ArticleIndexItem{Record: article.Record, Metadata: article.Metadata}

			if directory == CuratedDir {
				index.CuratedArticles = append(index.CuratedArticles, item)
			} else {
				index.PublishedArticles = append(index.PublishedArticles, item)
			}
		}
	}

	return index, nil
}

func LoadArticleIndex() (*ArticleIndex, error) {
	fmt.Printf("loading article index\n")
	index := NewArticleIndex()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	fmt.Printf("reading article index\n")
	content, err := shell.FilesRead(ctx, IndexFile)
	fmt.Printf("read article index\n")

	if err != nil {
		if err.Error() == "files/read: file does not exist" {
			return index, nil
		} else {
			return index, err
		}
	}

	fmt.Printf("decoding article index\n")
	err = json.NewDecoder(content).Decode(&index)
	if err != nil {
		return index, err
	}
	fmt.Printf("done loading article index\n")
	return index, nil
}

func writeArticleIndex(index *ArticleIndex) error {

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

	err = writeArticleIndex(index)
	if err != nil {
		return err
	}

	log.Println("refreshed article index")
	return nil
}
