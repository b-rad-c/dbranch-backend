package curator

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"time"

	ipfs "github.com/ipfs/go-ipfs-api"
)

//
// daemon to handle incoming articles on ipfs pubsub wire channel
//

func NewCuratorDaemon(config *Config) (*Curator, error) {
	daemon := NewCurator(config)

	err := daemon.setup()
	if err != nil {
		return daemon, err
	}

	daemon.waitForIPFS()

	return daemon, nil
}

func (c *Curator) setup() error {
	if c.Config.LogPath != "-" {
		logFile, err := os.OpenFile(c.Config.LogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		defer logFile.Close()

		log.SetOutput(logFile)
	}

	log.Printf("allow empty: %v, num peers: %d\n", c.Config.AllowEmptyPeerList, len(c.Config.AllowedPeers))
	if !c.Config.AllowEmptyPeerList && len(c.Config.AllowedPeers) == 0 {
		return errors.New("empty peer list is not allowed, set config value 'allow_empty_peer_list' to 'true' or add peers")
	}

	return nil
}

func (c *Curator) waitForIPFS() {
	// wait for ipfs service to come online
	for {
		log.Printf("checking if ipfs is up: %s\n", c.Config.IpfsHost)
		if c.Shell.IsUp() {
			log.Println("ready to go!")
			break
		}
		time.Sleep(time.Second * 5)
	}

	// ensure directory exists - ipfs client doesn't take option to mkdir when calling FilesCp yet
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := c.Shell.FilesMkdir(ctx, c.Config.CuratedDir, ipfs.FilesMkdir.Parents(true))
	if err != nil {
		log.Printf("error creating curated dir: %s\n", err)
	}

	log.Printf("curated dir created: %s\n", c.Config.CuratedDir)
}

func (c *Curator) SubscribeLoop() {
	// init
	subscription, err := c.Shell.PubSubSubscribe(c.Config.WireChannel)
	if err != nil {
		panic(err)
	}

	defer subscription.Cancel()
	log.Printf("subscribed to wire channel: %s\n", c.Config.WireChannel)

	// enter infinite loop
	for {
		// wait for new message & attempt to decode json
		msg, err := subscription.Next()
		if err != nil {
			log.Println("error getting next message: ", err)
			log.Panic(err)
		}

		incomingArticle := IncomingArticle{}
		err = json.Unmarshal([]byte(msg.Data), &incomingArticle)

		// cannot decode json, not a new article, log incoming msg and move on
		if err != nil {
			log.Printf("error decoding incoming msg: %s\n", err)
			log.Printf("raw msg: %s\n", msg.Data)
			continue
		}

		peer := msg.From.String()

		log.Printf("processing new article: %s from peer: %s\n", incomingArticle.CID, peer)

		// check if peer is allowed to publish this article
		if c.Config.PeerIsAllowed(peer) {
			// attempt to add article to local curated dir
			err = c.AddToCurated(&incomingArticle)
			if err != nil {
				log.Printf("error adding article to curated list: %s\n", err)
			}
		} else {
			log.Printf("peer: %s is not in allowed peers list\n", msg.From)
		}

	}
}
