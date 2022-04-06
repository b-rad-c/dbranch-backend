package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
)

const curatedDir = "/dBranch/curated"

func getShell() *shell.Shell {
	host := os.Getenv("IPFS_HOST")

	if host == "" {
		host = "localhost:5001"
	}

	log.Printf("creating shell for %s\n", host)

	return shell.NewShell(host)
}

func waitForService(sh *shell.Shell) {
	// wait for ipfs service to come online
	for {
		log.Println("checking if ipfs is up")
		if sh.IsUp() {
			log.Println("ready to go!")
			break
		}
		time.Sleep(time.Second * 5)
	}

	// ensure directory exists - ipfs client doesn't take option to mkdir when calling FilesCp yet
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := sh.FilesMkdir(ctx, curatedDir, shell.FilesMkdir.Parents(true))
	if err != nil {
		log.Printf("error creating curated dir: %s\n", err)
	}

	log.Printf("curated dir created: %s\n", curatedDir)
}

type newArticle struct {
	Name string `json:"name"`
	CID  string `json:"cid"`
}

func curateArticle(sh *shell.Shell, article newArticle) {
	ipfsPath := path.Join("/ipfs", article.CID)
	localPath := path.Join(curatedDir, article.Name)
	log.Printf("got new article, copying from: %s to: %s\n", ipfsPath, localPath)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := sh.FilesCp(ctx, ipfsPath, localPath)
	if err != nil {
		log.Printf("error copying: %s\n", err)
		return
	}

	log.Println("article copy complete")

}

func wireSubscribeLoop(sh *shell.Shell) {
	wire_topic := os.Getenv("DBRANCH_WIRE")

	if wire_topic == "" {
		wire_topic = "dbranch-wire"
	}

	subscription, err := sh.PubSubSubscribe(wire_topic)
	if err != nil {
		panic(err)
	}

	defer subscription.Cancel()

	log.Printf("subscribed to wire channel: %s\n", wire_topic)

	for {
		msg, err := subscription.Next()
		if err != nil {
			panic(err)
		}

		article := newArticle{}
		err = json.Unmarshal([]byte(msg.Data), &article)
		if err == nil {
			curateArticle(sh, article)
		} else {
			// not a new article, log incoming msg
			log.Printf("error decoding incoming msg: %s\n", err)
			log.Printf("raw msg: %s\n", msg.Data)
		}

	}
}

func main() {
	sh := getShell()

	waitForService(sh)

	wireSubscribeLoop(sh)

	log.Println("exiting 0")
}
