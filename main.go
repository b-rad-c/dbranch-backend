package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	curator "github.com/b-rad-c/dbranch-backend/curator"
	"github.com/urfave/cli/v2"
)

func main() {

	app := &cli.App{
		Name:    "dBranch Backend",
		Usage:   "Curate articles from the dBranch news protocol",
		Version: "0.1.0",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "The path to the dbranch config file",
				Value:   curator.DefaultConfigPath(),
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "peers",
				Usage: "show or edit the allowed peers list, run with args 'peers help' for more info",
				Subcommands: []*cli.Command{
					{
						Name:  "show",
						Usage: "print the peer list",
						Action: func(cli *cli.Context) error {
							return peers(cli, "show")
						},
					},
					{
						Name:  "add",
						Usage: "add one or more peers to the list",
						Action: func(cli *cli.Context) error {
							return peers(cli, "add")
						},
					},
				},
			},
			{
				Name:  "daemon",
				Usage: "Run the curator daemon",
				Action: func(cli *cli.Context) error {
					return runCuratorDaemon(cli)
				},
			},
			{
				Name:  "serve",
				Usage: "Serve curated articles over HTTP",
				Action: func(cli *cli.Context) error {
					return runCuratorServer(cli)
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}

func peers(cli *cli.Context, command string) error {
	configPath := cli.String("config")

	config, err := curator.LoadConfig(configPath)
	if err != nil {
		return err
	}

	if command == "show" {
		// if no command, print peers
		fmt.Printf("showing %d peer(s)\n", len(config.AllowedPeers))

		for index, peerId := range config.AllowedPeers {
			fmt.Printf("%d) %s\n", index+1, peerId)
		}

		return nil

	} else if command == "add" {
		newPeers := cli.Args().Slice()
		if len(newPeers) == 0 {
			return errors.New("no peers specified")
		}

		config.AllowedPeers = append(config.AllowedPeers, newPeers...)
		curator.WriteConfig(configPath, config)

		fmt.Printf("added %d peer(s)\n", len(newPeers))

	} else {
		return errors.New("invalid argument: " + command)
	}

	return nil
}

func runCuratorDaemon(cli *cli.Context) error {

	config, err := curator.LoadConfig(cli.String("config"))
	if err != nil {
		return err
	}

	wire := curator.NewCuratorDaemon(config)

	err = wire.Setup()
	if err != nil {
		return err
	}

	wire.WaitForIPFS()

	wire.SubscribeLoop()

	return nil
}

//
// server
//

func runCuratorServer(cli *cli.Context) error {

	config, err := curator.LoadConfig(cli.String("config"))
	if err != nil {
		return err
	}

	server := curator.NewCuratorServer(config)
	err = server.Start(":1323")
	server.Logger.Fatal(err)
	return err
}
