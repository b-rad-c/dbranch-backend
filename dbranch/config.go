package dbranch

import "github.com/urfave/cli/v2"

type Config struct {
	AllowEmptyPeerList bool
	CuratedDir         string
	IpfsHost           string
	LogPath            string
	PeerFilePath       string
	WireChannel        string
}

func ConfigFromCLI(cli *cli.Context) *Config {
	return &Config{
		AllowEmptyPeerList: cli.Bool("allow-empty-peer-list"),
		CuratedDir:         cli.String("curated-dir"),
		IpfsHost:           cli.String("ipfs-host"),
		LogPath:            cli.String("log-path"),
		PeerFilePath:       cli.String("peer-file"),
		WireChannel:        cli.String("wire-channel"),
	}
}
