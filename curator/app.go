package curator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path"

	ipfs "github.com/ipfs/go-ipfs-api"
)

//
// curator app
//

type Curator struct {
	Config *Config
	Shell  *ipfs.Shell
}

func NewCurator(config *Config) *Curator {
	return &Curator{
		Config: config,
		Shell:  ipfs.NewShell(config.IpfsHost),
	}
}

//
// config
//

type Config struct {
	AllowEmptyPeerList bool     `json:"allow_empty_peer_list"`
	CuratedDir         string   `json:"curated_dir"`
	IpfsHost           string   `json:"ipfs_host"`
	CardanoWalletHost  string   `json:"cardano_wallet_host"`
	LogPath            string   `json:"log_path"`
	AllowedPeers       []string `json:"allowed_peers"`
	WireChannel        string   `json:"wire_channel"`
}

func DefaultConfig() *Config {
	return &Config{
		AllowEmptyPeerList: false,
		CuratedDir:         "/dBranch/curated",
		IpfsHost:           "localhost:5001",
		CardanoWalletHost:  "http://localhost:8090",
		LogPath:            "-",
		AllowedPeers:       []string{},
		WireChannel:        "dbranch-wire",
	}
}

func DefaultConfigPath() string {
	user, _ := user.Current()
	return path.Join(user.HomeDir, ".dbranch/curator.json")
}

func LoadConfig(configPath string) (*Config, error) {
	var config Config
	f, err := os.Open(configPath)
	defer f.Close()

	// if config file doesn't exist, create default config
	if errors.Is(err, fs.ErrNotExist) {
		config = *DefaultConfig()

		err = config.WriteConfig(configPath)
		if err != nil {
			return &config, errors.New("error creating default config file: " + err.Error())
		}

		fmt.Printf("created default config file at: %s\n", configPath)
		return &config, nil

	} else if err != nil {
		return &config, errors.New("error loading config file: " + err.Error())
	}

	err = json.NewDecoder(f).Decode(&config)
	if err != nil {
		return &config, errors.New("error decoding config file: " + err.Error())
	}

	return &config, nil
}

func (config *Config) WriteConfig(configPath string) error {

	if configPath == DefaultConfigPath() {
		dir, _ := path.Split(configPath)
		os.MkdirAll(dir, 0755)
	}

	f, err := os.Create(configPath)
	defer f.Close()

	if err != nil {
		return err
	}

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "    ")
	return encoder.Encode(config)
}

//
// config peer ops
//

func (config *Config) AddPeers(peerIds ...string) int {
	config.AllowedPeers = append(config.AllowedPeers, peerIds...)
	return len(peerIds)
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
