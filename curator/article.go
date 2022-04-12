package curator

import (
	"context"
	"encoding/json"
	"log"
	"path"
	"time"

	ipfs "github.com/ipfs/go-ipfs-api"
)

//
// curator
//

type Curator struct {
	Config *Config
	Shell  *ipfs.Shell
}

func NewCurator(config *Config) *Curator {
	return &Curator{
		Config: config,
		Shell:  ipfs.NewShell(config.IpfsHost),
	}
}

//
// article models
//

type ArticleMetadata struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	SubTitle string `json:"subTitle"`
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
	Items []*ArticleListItem `json:"entries"`
}

type IncomingArticle struct {
	Name string `json:"name"`
	CID  string `json:"cid"`
}

//
// curator article actions
//

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

	log.Println("article successfully added to curated list")
	return nil
}

func (c *Curator) RemoveFromCurated(name string) error {
	articlePath := path.Join(c.Config.CuratedDir, name)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
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
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		articlePath := path.Join(c.Config.CuratedDir, entry.Name)
		content, err := c.Shell.FilesRead(ctx, articlePath, ipfs.FilesLs.Stat(true))
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
