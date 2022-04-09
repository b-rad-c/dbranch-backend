package main

import (
	"fmt"
	"log"
	"os"

	dbranch "github.com/b-rad-c/dbranch-backend/dbranch"
	"github.com/urfave/cli/v2"
)

func main() {

	app := &cli.App{
		Name:    "dBranch Backend",
		Usage:   "Curate articles from the dBranch news protocol",
		Version: "1.0.0",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "allow-empty-peer-list",
				Usage: "If true curator will curate all articles received over wire",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "curated-dir",
				Usage: "The ipfs mfs path to copy curated articles into",
				Value: "/dBranch/curated",
			},
			&cli.StringFlag{
				Name:  "ipfs-host",
				Usage: "The address for the local ipfs node",
				Value: "localhost:5001",
			},
			&cli.StringFlag{
				Name:    "log-path",
				Aliases: []string{"log"},
				Usage:   "The log path to use or '-' for stdout",
				Value:   "-",
			},
			&cli.StringFlag{
				Name:  "peer-file",
				Usage: "The json file to parse for allowed peers",
				Value: "./peer-allow-list.json",
			},
			&cli.StringFlag{
				Name:  "wire-channel",
				Usage: "The ipfs pubsub topic subscribe to for new articles",
				Value: "dbranch-wire",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "peers",
				Usage: "See/edit peer list",
				Action: func(c *cli.Context) error {
					return peers(dbranch.ConfigFromCLI(c))
				},
			},
			{
				Name:  "run",
				Usage: "Run the curator daemon",
				Action: func(c *cli.Context) error {
					return runCurator(dbranch.ConfigFromCLI(c))
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}

func peers(config *dbranch.Config) error {
	peers, err := dbranch.LoadPeerAllowFile(config.PeerFilePath)
	if err != nil {
		return err
	}
	fmt.Printf("showing %d peer(s) from file: %s\n", len(peers.AllowedPeers), config.PeerFilePath)

	for index, peerId := range peers.AllowedPeers {
		fmt.Printf("%d) %s\n", index+1, peerId)
	}
	return nil
}

func runCurator(config *dbranch.Config) error {
	if config.LogPath != "-" {
		logFile, err := os.OpenFile(config.LogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		defer logFile.Close()

		log.SetOutput(logFile)
	}

	wire := dbranch.NewWireSub(config)

	wire.VerifyCanRun()

	wire.WaitForService()

	wire.SubscribeLoop()

	log.Println("exiting 0")

	return nil
}
