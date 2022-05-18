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
					{
						Name:  "index",
						Usage: "article index                             // show curated article index",
						Action: func(cli *cli.Context) error {
							return articleCommand(cli, "index")
						},
					},
				},
			},
			{
				Name:  "cardano",
				Usage: "interact with cardano blockchain",
				Subcommands: []*cli.Command{
					{
						Name:  "status",
						Usage: "cardano status                                        // show cardano node network status",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "status")
						},
					},
					{
						Name:  "wait",
						Usage: "cardano wait                                          // wait for network to become ready",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "wait")
						},
					},
					{
						Name:  "articles",
						Usage: "cardano articles <wallet id>                          // list articles for a given wallet",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "articles")
						},
					},
					{
						Name:  "sign",
						Usage: "cardano sign <wallet id> <address> <name> <location>  // sign an article by sending a transaction to your own wallet with metadata about the article",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "sign")
						},
					},
					{
						Name:  "transactions",
						Usage: "cardano transactions <wallet id>                      // list wallets for a given wallet",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "transactions")
						},
					},
					{
						Name:  "wallets",
						Usage: "cardano wallets                                       // list wallets",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "wallets")
						},
					},
					{
						Name:  "addresses",
						Usage: "cardano addresses <wallet id>                         // list addresses for a given wallet",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "addresses")
						},
					},
					{
						Name:  "db-ping",
						Usage: "cardano db-ping                                      // ping the db",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "db-ping")
						},
					},
					{
						Name:  "db-meta",
						Usage: "cardano db-meta                                      // show cardano db metadata",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "db-meta")
						},
					},
					{
						Name:  "db-records",
						Usage: "cardano db-records                                      // list all cardano article transactions",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "db-records")
						},
					},
					{
						Name:  "db-address",
						Usage: "cardano db-address                                      // list cardano article transactions by wallet address",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "db-address")
						},
					},
					{
						Name:  "db-tx",
						Usage: "cardano db-tx                                          // lookup cardano article by tx hash",
						Action: func(cli *cli.Context) error {
							return cardanoCommand(cli, "db-tx")
						},
					},
				},
			},
			{
				Name:  "index",
				Usage: "generate or view the article index which lists curated and published (signed w cardano) articles",
				Subcommands: []*cli.Command{
					{
						Name:  "show",
						Usage: "index show                           // show the article index",
						Action: func(cli *cli.Context) error {
							return indexCommand(cli, "show")
						},
					},
					{
						Name:  "generate",
						Usage: "index generate                       // generate the article index",
						Action: func(cli *cli.Context) error {
							return indexCommand(cli, "generate")
						},
					},
				},
			},
			{
				Name:  "peers",
				Usage: "show or edit the allowed peers list, run with args 'peers help' for more info",
				Subcommands: []*cli.Command{
					{
						Name:  "list",
						Usage: "peers list                           // show the allowed peers list",
						Action: func(cli *cli.Context) error {
							return peerCommand(cli, "list")
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

func getConfigPath(cli *cli.Context) string {
	/*
		the config path is chosen in the following order
			- env var: DBRANCH_CURATOR_CONFIG
			- cli arg: --config
			- curator.DefaultConfigPath()
	*/
	config_path := cli.String("config")

	env_path := os.Getenv("DBRANCH_CURATOR_CONFIG")
	if env_path != "" {
		config_path = env_path
	}

	return config_path
}

//
// command logic
//

func articleCommand(cli *cli.Context, sub_cmd string) error {
	config, err := curator.LoadConfig(getConfigPath(cli))
	if err != nil {
		return err
	}

	app := curator.NewCurator(config)

	args := cli.Args().Slice()

	if sub_cmd == "add" {

		if len(args) < 2 {
			return errors.New("did not supply article name and cid")
		}

		err = app.AddRecordToLocal(&curator.ArticleRecord{Name: args[0], CID: args[1]}, curator.Curated)
		if err != nil {
			return err
		}

		fmt.Printf("added %s to curated list from cid: %s\n", args[0], args[1])

	} else if sub_cmd == "remove" {

		if len(args) < 1 {
			return errors.New("did not supply article name")
		}

		err = app.RemoveRecordFromLocal(args[0], curator.Curated)
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
			fmt.Printf("%2d) %s\n", index+1, article.Name)
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
	config, err := curator.LoadConfig(getConfigPath(cli))
	if err != nil {
		return err
	}

	app := curator.NewCurator(config)
	args := cli.Args().Slice()

	if sub_cmd == "status" {
		status, err := app.Status()
		if err != nil {
			return err
		}

		fmt.Printf("status: %s\n", status)

	} else if sub_cmd == "wait" {
		app.WaitForCardano()

	} else if sub_cmd == "sign" {
		if len(args) < 4 {
			return errors.New("incorrect arguments")
		}

		transaction_id, err := app.SignArticle(args[0], args[1], args[2], args[3])
		if err != nil {
			return err
		}

		fmt.Printf("transaction id: %s\n", transaction_id)

	} else if sub_cmd == "wallets" {
		wallets, err := app.WalletIds()
		if err != nil {
			return err
		}

		for index, wallet := range wallets {
			fmt.Printf("%d) %s\n", index+1, wallet)
		}

	} else if sub_cmd == "addresses" {
		if len(args) < 1 {
			return errors.New("did not supply wallet id")
		}

		addresses, err := app.WalletAddresses(args[0])
		if err != nil {
			return err
		}

		for index, address := range addresses {
			fmt.Printf("%2d) %6s - %s\n", index+1, address.State, address.ID)
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

		articles, err := app.ListSignedArticles(args[0])
		if err != nil {
			return err
		}
		printJSON(articles)
	} else if sub_cmd == "db-ping" {
		err := curator.CardanoDBPing()
		if err != nil {
			return err
		} else {
			fmt.Println("success!")
		}
	} else if sub_cmd == "db-meta" {
		db_meta, err := curator.CardanoDBMeta()
		if err != nil {
			return err
		}
		printJSON(db_meta)
	} else if sub_cmd == "db-records" {
		records, err := curator.CardanoRecords()
		if err != nil {
			return err
		}
		printJSON(records)
	} else if sub_cmd == "db-address" {
		if len(args) < 1 {
			return errors.New("did not supply address")
		}
		records, err := curator.CardanoRecordsByAddress(args[0])
		if err != nil {
			return err
		}
		printJSON(records)
	} else if sub_cmd == "db-tx" {
		if len(args) < 1 {
			return errors.New("did not supply tx hash")
		}
		records, err := curator.CardanoRecordsByTxHash(args[0])
		if err != nil {
			return err
		}
		printJSON(records)
	}

	return nil
}

func indexCommand(cli *cli.Context, sub_cmd string) error {
	configPath := getConfigPath(cli)
	config, err := curator.LoadConfig(configPath)
	if err != nil {
		return err
	}

	app := curator.NewCurator(config)

	if sub_cmd == "show" {
		index, err := app.LoadArticleIndex()
		if err != nil {
			return err
		}

		printJSON(index)

		return nil

	} else if sub_cmd == "generate" {
		return app.GenerateArticleIndex()
	}

	return nil
}

func peerCommand(cli *cli.Context, sub_cmd string) error {
	configPath := getConfigPath(cli)
	config, err := curator.LoadConfig(configPath)
	if err != nil {
		return err
	}

	if sub_cmd == "list" {
		// if no command, print peers
		fmt.Printf("showing %d peer(s)\n", len(config.AllowedPeers))

		for index, peerId := range config.AllowedPeers {
			fmt.Printf("%2d) %s\n", index+1, peerId)
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
	config, err := curator.LoadConfig(getConfigPath(cli))
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
	config, err := curator.LoadConfig(getConfigPath(cli))
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
