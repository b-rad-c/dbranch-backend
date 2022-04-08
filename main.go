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
// peer allow list
//

type PeerAllowList struct {
	AllowedPeers []string `json:"allowed_peers"`
}

func (peers *PeerAllowList) PeerIsAllowed(peerId string) bool {
	if len(peers.AllowedPeers) == 0 {
		// empty list implies all peers are allowed
		return true
	}

	for _, p := range peers.AllowedPeers {
		if p == peerId {
			return true
		}
	}
	return false
}

func LoadPeerAllowFile(path string) (*PeerAllowList, error) {
	var peers PeerAllowList
	f, err := os.Open(path)
	defer f.Close()

	if err != nil {
		return &peers, err
	}

	err = json.NewDecoder(f).Decode(&peers)
	if err != nil {
		return &peers, err
	}

	return &peers, nil
}

//
// wire
//

type WireSub struct {
	IpfsHost           string
	WireChannel        string
	CuratedDir         string
	Peers              *PeerAllowList
	AllowEmptyPeerList bool
	sh                 *ipfs.Shell
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
		sh:                 ipfs.NewShell(host),
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

func main() {
	wire := WireSubFromEnv()

	wire.VerifyCanRun()

	wire.WaitForService()

	wire.SubscribeLoop()

	log.Println("exiting 0")
}
