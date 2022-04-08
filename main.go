package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path"
	"time"

	ipfs "github.com/ipfs/go-ipfs-api"
)

//
// article
//

type Article struct {
	Name string `json:"name"`
	CID  string `json:"cid"`
}

func (a *Article) addToCurated(wire *WireSub) error {
	ipfsPath := path.Join("/ipfs", a.CID)
	localPath := path.Join(wire.CuratedDir, a.Name)
	log.Printf("processing new article: %s\n", a.CID)

	//
	// stat file to determine if we have it already and how to proceed
	//

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stat, err := wire.sh.FilesStat(ctx, localPath)

	if err != nil && err.Error() != "files/stat: file does not exist" {
		return err
	}

	if stat != nil {
		if stat.Hash == a.CID {
			// if we already have the same hash in our curated dir, don't copy
			log.Printf("already have article: %s with hash %s\n", a.Name, a.CID)
			return nil
		} else {
			// hash is different, replace existing article
			log.Printf("replacing article: %s with newer hash %s\n", a.Name, a.CID)
			ctx, cancel = context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			err = wire.sh.FilesRm(ctx, localPath, true)
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

	if err = wire.sh.FilesCp(ctx, ipfsPath, localPath); err != nil {
		return err
	}

	// pin article, this is necessary to store the entire file contents locally
	// because FilesCp does not copy the entire contents of the file, just the root node of the DAG
	log.Println("pinning article to local node")
	if err = wire.sh.Pin(a.CID); err != nil {
		return err
	}

	log.Println("article successfully added to curated list")
	return nil
}

//
// wire
//

type WireSub struct {
	IpfsHost    string
	WireChannel string
	CuratedDir  string
	sh          *ipfs.Shell
}

func WireSubFromEnv() *WireSub {
	host := os.Getenv("IPFS_HOST")
	if host == "" {
		host = "localhost:5001"
	}

	wire := os.Getenv("DBRANCH_WIRE_CHANNEL")
	if wire == "" {
		wire = "dbranch-wire"
	}

	dir := os.Getenv("DBRANCH_CURATED_DIRECTORY")
	if dir == "" {
		dir = "/dBranch/curated"
	}

	return &WireSub{
		IpfsHost:    host,
		WireChannel: wire,
		CuratedDir:  dir,
		sh:          ipfs.NewShell(host),
	}
}

func (wire *WireSub) WaitForService() {
	// wait for ipfs service to come online
	for {
		log.Printf("checking if ipfs is up: %s\n", wire.IpfsHost)
		if wire.sh.IsUp() {
			log.Println("ready to go!")
			break
		}
		time.Sleep(time.Second * 5)
	}

	// ensure directory exists - ipfs client doesn't take option to mkdir when calling FilesCp yet
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := wire.sh.FilesMkdir(ctx, wire.CuratedDir, ipfs.FilesMkdir.Parents(true))
	if err != nil {
		log.Printf("error creating curated dir: %s\n", err)
	}

	log.Printf("curated dir created: %s\n", wire.CuratedDir)
}

func (wire *WireSub) SubscribeLoop() {

	// setup pubsub subscription
	subscription, err := wire.sh.PubSubSubscribe(wire.WireChannel)
	if err != nil {
		panic(err)
	}

	defer subscription.Cancel()
	log.Printf("subscribed to wire channel: %s\n", wire.WireChannel)

	// enter infinite loop
	for {
		// wait for new message &  attempt to decode json
		msg, err := subscription.Next()
		if err != nil {
			log.Panic(err)
		}

		article := Article{}
		err = json.Unmarshal([]byte(msg.Data), &article)

		// cannot decode json, not a new article, log incoming msg and move on
		if err != nil {
			log.Printf("error decoding incoming msg: %s\n", err)
			log.Printf("raw msg: %s\n", msg.Data)
			continue
		}

		// attempt to add article to local curated dir
		err = article.addToCurated(wire)
		if err != nil {
			log.Printf("error adding article to curated list: %s\n", err)
		}

	}
}

func main() {
	wire := WireSubFromEnv()

	wire.WaitForService()

	wire.SubscribeLoop()

	log.Println("exiting 0")
}
