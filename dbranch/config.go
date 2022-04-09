package dbranch

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

type Config struct {
	AllowEmptyPeerList bool     `json:"allow_empty_peer_list"`
	CuratedDir         string   `json:"curated_dir"`
	IpfsHost           string   `json:"ipfs_host"`
	LogPath            string   `json:"log_path"`
	AllowedPeers       []string `json:"allowed_peers"`
	WireChannel        string   `json:"wire_channel"`
}

func DefaultConfig() *Config {
	return &Config{
		AllowEmptyPeerList: false,
		CuratedDir:         "/dBranch/curated",
		IpfsHost:           "localhost:5001",
		LogPath:            "-",
		AllowedPeers:       []string{},
		WireChannel:        "dbranch-wire",
	}
}

func WriteConfig(path string, config *Config) error {
	f, err := os.Create(path)
	defer f.Close()

	if err != nil {
		return err
	}

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "    ")
	return encoder.Encode(config)
}

func LoadConfig(path string) (*Config, error) {
	var config Config
	f, err := os.Open(path)
	defer f.Close()

	// if config file doesn't exist, create default config
	if errors.Is(err, fs.ErrNotExist) {
		config = *DefaultConfig()

		err = WriteConfig(path, &config)
		if err != nil {
			return &config, errors.New("error creating default config file: " + err.Error())
		}

		fmt.Printf("created default config file at: %s\n", path)
		return &config, nil

	} else if err != nil {
		return &config, errors.New("error loading config file: " + err.Error())
	}

	err = json.NewDecoder(f).Decode(&config)
	if err != nil {
		return &config, err
	}

	return &config, nil
}

func (config *Config) PeerIsAllowed(peerId string) bool {
	if len(config.AllowedPeers) == 0 {
		// empty list implies all peers are allowed
		return true
	}

	for _, p := range config.AllowedPeers {
		if p == peerId {
			return true
		}
	}
	return false
}
