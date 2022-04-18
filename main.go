package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	curator "github.com/b-rad-c/dbranch-backend/curator"
	"github.com/urfave/cli/v2"
)

//
// cli interface
//

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
				Name:  "article",
				Usage: "interact with articles, run with no arguments to see available sub commands",
				Subcommands: []*cli.Command{
					{
						Name:  "add",
						Usage: "article add <article_name> <article cid> // add article to curated list",
						Action: func(cli *cli.Context) error {
							return articleCommand(cli, "add")
						},
					},
					{
						Name:  "remove",
						Usage: "article remove <article_name>            // remove an article from curated list",
						Action: func(cli *cli.Context) error {
							return articleCommand(cli, "remove")
						},
					},
					{
						Name:  "get",
						Usage: "article get <article_name>               // get an article as json",
						Action: func(cli *cli.Context) error {
							return articleCommand(cli, "get")
						},
					},
					{
						Name:  "list",
						Usage: "article list                             // list curated articles",
						Action: func(cli *cli.Context) error {
							return articleCommand(cli, "list")
						},
					},
				},
			},
			{
				Name:  "cardano",
				Usage: "interact with cardano blockchain",
				Subcommands: []*cli.Command{
					{
						Name:  "network",
						Usage: "cardano network                          // show cardano node network information",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "network")
						},
					},
					{
						Name:  "articles",
						Usage: "cardano articles <wallet id>             // list articles for a given wallet",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "articles")
						},
					},
					{
						Name:  "transactions",
						Usage: "cardano transactions <wallet id>             // list wallets for a given wallet",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "transactions")
						},
					},
					{
						Name:  "wallets",
						Usage: "cardano wallets                          // list wallets",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "wallets")
						},
					},
				},
			},
			{
				Name:  "peers",
				Usage: "show or edit the allowed peers list, run with args 'peers help' for more info",
				Subcommands: []*cli.Command{
					{
						Name:  "show",
						Usage: "peers show                           // show the allowed peers list",
						Action: func(cli *cli.Context) error {
							return peerCommand(cli, "show")
						},
					},
					{
						Name:  "add",
						Usage: "peers add <peer_id> <peer_id> ...    // add one or more peers to the list",
						Action: func(cli *cli.Context) error {
							return peerCommand(cli, "add")
						},
					},
				},
			},
			{
				Name:  "daemon",
				Usage: "Run the curator daemon",
				Action: func(cli *cli.Context) error {
					return daemonCommand(cli)
				},
			},
			{
				Name:  "serve",
				Usage: "Serve curated articles over HTTP",
				Action: func(cli *cli.Context) error {
					return serverCommand(cli)
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}

//
// command logic
//

func articleCommand(cli *cli.Context, sub_cmd string) error {
	config, err := curator.LoadConfig(cli.String("config"))
	if err != nil {
		return err
	}

	app := curator.NewCurator(config)

	args := cli.Args().Slice()

	if sub_cmd == "add" {

		if len(args) < 2 {
			return errors.New("did not supply article name and cid")
		}

		err = app.AddToCurated(&curator.IncomingArticle{Name: args[0], CID: args[1]})
		if err != nil {
			return err
		}

		fmt.Printf("added %s to curated list from cid: %s\n", args[0], args[1])

	} else if sub_cmd == "remove" {

		if len(args) < 1 {
			return errors.New("did not supply article name")
		}

		err = app.RemoveFromCurated(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("removed %s from curated list\n", args[0])

	} else if sub_cmd == "list" {
		list, err := app.ListArticles()
		if err != nil {
			return err
		}
		for index, article := range list.Items {
			fmt.Printf("%d) %s\n", index+1, article.Name)
		}

	} else if sub_cmd == "get" {

		if len(args) < 1 {
			return errors.New("did not supply article name")
		}

		article, err := app.GetArticle(args[0])
		if err != nil {
			return err
		}

		printJSON(article)

	}

	return nil
}

func cardanoCommand(cli *cli.Context, sub_cmd string) error {
	config, err := curator.LoadConfig(cli.String("config"))
	if err != nil {
		return err
	}

	app := curator.NewCurator(config)
	args := cli.Args().Slice()

	if sub_cmd == "network" {
		network, err := app.NetworkInformation()
		if err != nil {
			return err
		}

		printJSON(network)

	} else if sub_cmd == "wallets" {
		wallets, err := app.WalletIds()
		if err != nil {
			return err
		}

		for index, wallet := range wallets {
			fmt.Printf("%d) %s\n", index+1, wallet)
		}
	} else if sub_cmd == "transactions" {
		if len(args) < 1 {
			return errors.New("did not supply wallet id")
		}

		transactions, err := app.WalletTransactions(args[0])
		if err != nil {
			return err
		}
		printJSON(transactions)
	} else if sub_cmd == "articles" {
		if len(args) < 1 {
			return errors.New("did not supply wallet id")
		}

		articles, err := app.WalletArticles(args[0])
		if err != nil {
			return err
		}
		printJSON(articles)
	}

	return nil
}

func peerCommand(cli *cli.Context, sub_cmd string) error {
	configPath := cli.String("config")

	config, err := curator.LoadConfig(configPath)
	if err != nil {
		return err
	}

	if sub_cmd == "show" {
		// if no command, print peers
		fmt.Printf("showing %d peer(s)\n", len(config.AllowedPeers))

		for index, peerId := range config.AllowedPeers {
			fmt.Printf("%d) %s\n", index+1, peerId)
		}

		return nil

	} else if sub_cmd == "add" {
		numAdded := config.AddPeers(cli.Args().Slice()...)
		err = config.WriteConfig(configPath)
		if err != nil {
			return err
		}

		fmt.Printf("added %d peer(s)\n", numAdded)

	}

	return nil
}

func daemonCommand(cli *cli.Context) error {

	config, err := curator.LoadConfig(cli.String("config"))
	if err != nil {
		return err
	}

	daemon, err := curator.NewCuratorDaemon(config)
	if err != nil {
		return err
	}

	daemon.SubscribeLoop()

	return nil
}

func serverCommand(cli *cli.Context) error {

	config, err := curator.LoadConfig(cli.String("config"))
	if err != nil {
		return err
	}

	server := curator.NewCuratorServer(config)
	err = server.Start(":1323")
	server.Logger.Fatal(err)
	return err
}

//
// misc
//

func printJSON(data interface{}) {
	indented, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(indented))
}
