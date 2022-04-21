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

func (c *Curator) processIncomingMessage(msg *ipfs.Message) {

	//
	// init
	//

	peer := msg.From.String()
	incomingArticle := IncomingArticle{}
	err := json.Unmarshal([]byte(msg.Data), &incomingArticle)

	if err == nil {

		//
		// message is an article, check if peer is allowed to publish
		//

		if c.Config.PeerIsAllowed(peer) {
			err = c.AddToCurated(&incomingArticle)

			if err == nil {
				log.Printf("added new article: %s from peer: %s\n", incomingArticle.CID, peer)
			} else {
				log.Printf("error adding article to curated list: %s\n", err)
			}

		} else {
			log.Printf("peer: %s is not in allowed peers list\n", msg.From)
		}

	} else {

		//
		// message is not an article, log ping or invalid message
		//

		if string(msg.Data) == "ping" {
			log.Printf("received ping from peer: %s\n", peer)
		} else {
			log.Printf("error decoding incoming msg: %s\n", err)
			log.Printf(" -> raw msg: %s\n", msg.Data)
		}
	}
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

		msg, err := subscription.Next()
		if err != nil {
			log.Println("error getting next message: ", err)
			log.Panic(err)
		}

		c.processIncomingMessage(msg)

	}
}
