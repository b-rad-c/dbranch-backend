package dbranch

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
// config
//

type Config struct {
	CuratedDir        string `json:"curated_dir"`
	Index             string `json:"index"`
	IpfsHost          string `json:"ipfs_host"`
	CardanoWalletHost string `json:"cardano_wallet_host"`
}

func DefaultConfig() *Config {
	return &Config{
		CuratedDir:        "/dBranch/curated",
		Index:             "/dBranch/index.json",
		IpfsHost:          "localhost:5001",
		CardanoWalletHost: "http://localhost:8090",
	}
}

func DefaultConfigPath() string {
	user, _ := user.Current()
	return path.Join(user.HomeDir, ".dbranch/curator.json")
}

func LoadConfig(configPath string) (*Config, error) {
	fmt.Printf("Loading config from: %s\n", configPath)

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

func (c *Config) WriteConfig(configPath string) error {

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
	return encoder.Encode(c)
}

func (c *Config) ipfsShell() *ipfs.Shell {
	return ipfs.NewShell(c.IpfsHost)
}
