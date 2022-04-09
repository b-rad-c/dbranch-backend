package dbranch

import (
	"context"
	"log"
	"path"
	"time"
)

type Article struct {
	Name string `json:"name"`
	CID  string `json:"cid"`
}

func (a *Article) addToCurated(wire *WireSub) error {
	ipfsPath := path.Join("/ipfs", a.CID)
	localPath := path.Join(wire.CuratedDir, a.Name)

	//
	// stat file to determine if we have it already and how to proceed
	//

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stat, err := wire.Shell.FilesStat(ctx, localPath)

	if err != nil && err.Error() != "files/stat: file does not exist" {
		return err
	}

	if stat != nil {
		log.Printf("stat hash: %s\n article cid: %s", stat.Hash, a.CID)

		if stat.Hash == a.CID {
			// if we already have the same hash in our curated dir, don't copy
			log.Printf("already have article: %s with hash %s\n", a.Name, a.CID)
			return nil
		} else {
			// hash is different, replace existing article
			log.Printf("replacing article: %s with newer hash %s\n", a.Name, a.CID)
			ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			err = wire.Shell.FilesRm(ctx, localPath, true)
			if err != nil {
				return err
			}
		}
	}

	//
	// copy article to ipfs files (mfs)
	//

	log.Printf("copying article from: %s to: %s\n", ipfsPath, localPath)
	ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err = wire.Shell.FilesCp(ctx, ipfsPath, localPath); err != nil {
		return err
	}

	// pin article, this is necessary to store the entire file contents locally
	// because FilesCp does not copy the entire contents of the file, just the root node of the DAG
	log.Println("pinning article to local node")
	if err = wire.Shell.Pin(a.CID); err != nil {
		return err
	}

	log.Println("article successfully added to curated list")
	return nil
}
