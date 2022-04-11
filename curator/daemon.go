package curator

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path"
	"time"

	ipfs "github.com/ipfs/go-ipfs-api"
)

//
// incoming article from wire
//

type IncomingArticle struct {
	Name string `json:"name"`
	CID  string `json:"cid"`
}

func (a *IncomingArticle) AddToCurated(wire *CuratorDaemon) error {
	ipfsPath := path.Join("/ipfs", a.CID)
	localPath := path.Join(wire.Config.CuratedDir, a.Name)

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

//
// curator wire daemon
//

type CuratorDaemon struct {
	Config *Config
	Shell  *ipfs.Shell
}

func NewCuratorDaemon(config *Config) *CuratorDaemon {
	return &CuratorDaemon{
		Config: config,
		Shell:  ipfs.NewShell(config.IpfsHost),
	}
}

func (wire *CuratorDaemon) Setup() error {
	if wire.Config.LogPath != "-" {
		logFile, err := os.OpenFile(wire.Config.LogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		defer logFile.Close()

		log.SetOutput(logFile)
	}

	log.Printf("allow empty: %v, num peers: %d\n", wire.Config.AllowEmptyPeerList, len(wire.Config.AllowedPeers))
	if !wire.Config.AllowEmptyPeerList && len(wire.Config.AllowedPeers) == 0 {
		return errors.New("empty peer list is not allowed, set config value 'allow_empty_peer_list' to 'true' or add peers")
	}

	return nil
}

func (wire *CuratorDaemon) WaitForIPFS() {
	// wait for ipfs service to come online
	for {
		log.Printf("checking if ipfs is up: %s\n", wire.Config.IpfsHost)
		if wire.Shell.IsUp() {
			log.Println("ready to go!")
			break
		}
		time.Sleep(time.Second * 5)
	}

	// ensure directory exists - ipfs client doesn't take option to mkdir when calling FilesCp yet
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := wire.Shell.FilesMkdir(ctx, wire.Config.CuratedDir, ipfs.FilesMkdir.Parents(true))
	if err != nil {
		log.Printf("error creating curated dir: %s\n", err)
	}

	log.Printf("curated dir created: %s\n", wire.Config.CuratedDir)
}

func (wire *CuratorDaemon) SubscribeLoop() {

	// setup pubsub subscription
	subscription, err := wire.Shell.PubSubSubscribe(wire.Config.WireChannel)
	if err != nil {
		panic(err)
	}

	defer subscription.Cancel()
	log.Printf("subscribed to wire channel: %s\n", wire.Config.WireChannel)

	// enter infinite loop
	for {
		// wait for new message &  attempt to decode json
		msg, err := subscription.Next()
		if err != nil {
			log.Println("error getting next message: ", err)
			log.Panic(err)
		}

		article := IncomingArticle{}
		err = json.Unmarshal([]byte(msg.Data), &article)

		// cannot decode json, not a new article, log incoming msg and move on
		if err != nil {
			log.Printf("error decoding incoming msg: %s\n", err)
			log.Printf("raw msg: %s\n", msg.Data)
			continue
		}

		peer := msg.From.String()

		log.Printf("processing new article: %s from peer: %s\n", article.CID, peer)

		// check if peer is allowed to publish this article
		if wire.Config.PeerIsAllowed(peer) {
			// attempt to add article to local curated dir
			err = article.AddToCurated(wire)
			if err != nil {
				log.Printf("error adding article to curated list: %s\n", err)
			}
		} else {
			log.Printf("peer: %s is not in allowed peers list\n", msg.From)
		}

	}
}
