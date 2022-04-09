package main

import (
	"errors"
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
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "The path to the dbranch config file",
				Value:   "~/.dbranch/config.json",
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
				Name:  "run",
				Usage: "Run the curator daemon",
				Action: func(cli *cli.Context) error {
					return runCuratorService(cli)
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

	config, err := dbranch.LoadConfig(configPath)
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
		dbranch.WriteConfig(configPath, config)

		fmt.Printf("added %d peer(s)\n", len(newPeers))

	} else {
		return errors.New("invalid argument: " + command)
	}

	return nil
}

func runCuratorService(cli *cli.Context) error {

	config, err := dbranch.LoadConfig(cli.String("config"))
	if err != nil {
		return err
	}

	wire := dbranch.NewCuratorService(config)

	err = wire.Setup()
	if err != nil {
		return err
	}

	wire.WaitForIPFS()

	wire.SubscribeLoop()

	return nil
}
