package dbranch

import (
	"encoding/json"
	"os"
)

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
