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

func (a *Article) addToCurated(wire *WireSub) {
	ipfsPath := path.Join("/ipfs", a.CID)
	localPath := path.Join(wire.CuratedDir, a.Name)
	log.Printf("got new article, copying from: %s to: %s\n", ipfsPath, localPath)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := wire.sh.FilesCp(ctx, ipfsPath, localPath)
	if err != nil {
		log.Printf("error copying: %s\n", err)
		return
	}

	log.Println("article copy complete")

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

	subscription, err := wire.sh.PubSubSubscribe(wire.WireChannel)
	if err != nil {
		panic(err)
	}

	defer subscription.Cancel()

	log.Printf("subscribed to wire channel: %s\n", wire.WireChannel)

	for {
		msg, err := subscription.Next()
		if err != nil {
			panic(err)
		}

		article := Article{}
		err = json.Unmarshal([]byte(msg.Data), &article)
		if err == nil {
			article.addToCurated(wire)
		} else {
			// not a new article, log incoming msg
			log.Printf("error decoding incoming msg: %s\n", err)
			log.Printf("raw msg: %s\n", msg.Data)
		}

	}
}

func main() {
	wire := WireSubFromEnv()

	wire.WaitForService()

	wire.SubscribeLoop()

	log.Println("exiting 0")
}
