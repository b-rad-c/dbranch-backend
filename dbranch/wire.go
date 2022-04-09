package dbranch

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	ipfs "github.com/ipfs/go-ipfs-api"
)

type WireSub struct {
	IpfsHost           string
	WireChannel        string
	CuratedDir         string
	Peers              *PeerAllowList
	AllowEmptyPeerList bool
	Shell              *ipfs.Shell
}

func NewWireSub(host, wire, dir, peerAllowFile string, allowEmptyPeerList bool) *WireSub {
	peers, err := LoadPeerAllowFile(peerAllowFile)
	if err != nil {
		log.Printf("error loading peer allow list: %s\n", err)
	} else {
		log.Printf("loaded %d allowed peer(s) from: %s\n", len(peers.AllowedPeers), peerAllowFile)
	}

	return &WireSub{
		IpfsHost:           host,
		WireChannel:        wire,
		CuratedDir:         dir,
		Peers:              peers,
		AllowEmptyPeerList: allowEmptyPeerList,
		Shell:              ipfs.NewShell(host),
	}
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

	peerAllowFile := os.Getenv("DBRANCH_PEER_ALLOW_LIST")
	if peerAllowFile == "" {
		peerAllowFile = "./peer-allow-list.json"
	}

	allowEmptyPeerList := false
	if os.Getenv("DBRANCH_ALLOW_EMPTY_PEER_LIST") == "true" {
		allowEmptyPeerList = true
	}

	return NewWireSub(host, wire, dir, peerAllowFile, allowEmptyPeerList)
}

func (wire *WireSub) VerifyCanRun() {
	if !wire.AllowEmptyPeerList && len(wire.Peers.AllowedPeers) == 0 {
		log.Panic("empty peer list is not allowed, set env variable DBRANCH_ALLOW_EMPTY_PEER_LIST='true' to allow")
	}
}

func (wire *WireSub) WaitForService() {
	// wait for ipfs service to come online
	for {
		log.Printf("checking if ipfs is up: %s\n", wire.IpfsHost)
		if wire.Shell.IsUp() {
			log.Println("ready to go!")
			break
		}
		time.Sleep(time.Second * 5)
	}

	// ensure directory exists - ipfs client doesn't take option to mkdir when calling FilesCp yet
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	err := wire.Shell.FilesMkdir(ctx, wire.CuratedDir, ipfs.FilesMkdir.Parents(true))
	if err != nil {
		log.Printf("error creating curated dir: %s\n", err)
	}

	log.Printf("curated dir created: %s\n", wire.CuratedDir)
}

func (wire *WireSub) SubscribeLoop() {

	// setup pubsub subscription
	subscription, err := wire.Shell.PubSubSubscribe(wire.WireChannel)
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

		peer := msg.From.String()

		log.Printf("processing new article: %s from peer: %s\n", article.CID, peer)

		// check if peer is allowed to publish this article
		if wire.Peers.PeerIsAllowed(peer) {
			// attempt to add article to local curated dir
			err = article.addToCurated(wire)
			if err != nil {
				log.Printf("error adding article to curated list: %s\n", err)
			}
		} else {
			log.Printf("peer: %s is not in allowed peers list\n", msg.From)
		}

	}
}
