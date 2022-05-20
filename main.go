package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	dbranch "github.com/b-rad-c/dbranch-backend/dbranch"
	"github.com/urfave/cli/v2"
)

//
// cli interface
//

var config *dbranch.Config

func loadConfig(cli *cli.Context) error {

	/*
		the config path is chosen in the following order
			- env var: DBRANCH_CURATOR_CONFIG
			- cli arg: --config
			- curator.DefaultConfigPath()
	*/
	var err error
	config_path := cli.String("config")

	env_path := os.Getenv("DBRANCH_CURATOR_CONFIG")
	if env_path != "" {
		config_path = env_path
	}

	config, err = dbranch.LoadConfig(config_path)
	return err
}

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
				Value:   dbranch.DefaultConfigPath(),
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "article",
				Usage: "Interact with curated articles",
				Subcommands: []*cli.Command{
					{
						Name:  "get",
						Usage: "get a curated article",
						Action: func(cli *cli.Context) error {
							record, article, err := config.GetArticle(cli.Args().First())
							if err != nil {
								return err
							}
							printJSON(&dbranch.FullArticle{Article: article, Record: record})
							return nil
						},
					},
					{
						Name:  "index",
						Usage: "generate or view the article index which lists curated and published (signed w cardano) articles",
						Subcommands: []*cli.Command{
							{
								Name:  "show",
								Usage: "show the article index",
								Action: func(cli *cli.Context) error {
									index, err := config.LoadArticleIndex()
									if err != nil {
										return err
									}

									printJSON(index)
									return nil
								},
							},
							{
								Name:  "refresh",
								Usage: "refresh the article index",
								Action: func(cli *cli.Context) error {
									return config.RefreshArticleIndex()
								},
							},
						},
					},
				},
			},
			{
				Name:    "cardano-db",
				Aliases: []string{"db"},
				Usage:   "Interact with cardano sql db",
				Subcommands: []*cli.Command{
					{
						Name:  "ping",
						Usage: "ping postgres db",
						Action: func(cli *cli.Context) error {
							err := dbranch.CardanoDBPing()
							if err != nil {
								return err
							} else {
								fmt.Println("success!")
							}
							return nil
						},
					},
					{
						Name:  "meta",
						Usage: "show db metadata",
						Action: func(cli *cli.Context) error {
							db_meta, err := dbranch.CardanoDBMeta()
							if err != nil {
								return err
							}
							printJSON(db_meta)
							return nil
						},
					},
					{
						Name:  "status",
						Usage: "show db sync status",
						Action: func(cli *cli.Context) error {
							return errors.New("not implemented")
						},
					},
					{
						Name:  "records",
						Usage: "show cardano article records",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "address",
								Aliases: []string{"a", "addr"},
								Usage:   "filter records by public address",
							},
							&cli.StringFlag{
								Name:  "hash",
								Usage: "filter records by transaction hash",
							},
						},
						Action: func(cli *cli.Context) error {
							return errors.New("not implemented")
						},
					},
					{
						Name:      "add-tx",
						Usage:     "add article to curated list by cardano tx hash",
						ArgsUsage: "add-tx [tx_hash]",
						Action: func(cli *cli.Context) error {
							return errors.New("not implemented")
						},
					},
				},
			},

			{
				Name:    "cardano-wallet",
				Aliases: []string{"wallet"},
				Usage:   "interact with cardano wallet",
				Subcommands: []*cli.Command{
					{
						Name:  "status",
						Usage: "show cardano node network status",
						Action: func(cli *cli.Context) error {
							status, err := config.Status()
							if err != nil {
								return err
							}

							fmt.Printf("status: %s\n", status)
							return nil
						},
					},
					{
						Name:  "wait",
						Usage: "wait for network to become ready",
						Action: func(cli *cli.Context) error {
							config.WaitForCardano()
							return nil
						},
					},
					{
						Name:  "list",
						Usage: "list available wallets by id",
						Action: func(cli *cli.Context) error {
							wallets, err := config.WalletIds()
							if err != nil {
								return err
							}

							for index, wallet := range wallets {
								fmt.Printf("%2d) %s\n", index+1, wallet)
							}
							return nil
						},
					},
					{
						Name:  "addresses",
						Usage: "list addresses for a given wallet id",
						Action: func(cli *cli.Context) error {
							addresses, err := config.WalletAddresses(cli.Args().First())
							if err != nil {
								return err
							}

							for index, address := range addresses {
								fmt.Printf("%2d) %6s - %s\n", index+1, address.State, address.ID)
							}
							return nil
						},
					},
					{
						Name:  "sign",
						Usage: "sign an article by sending a transaction to your own wallet with metadata about the article",
						Action: func(cli *cli.Context) error {
							args := cli.Args().Slice()
							transaction_id, err := config.SignArticle(args[0], args[1], args[2], args[3])
							if err != nil {
								return err
							}

							fmt.Printf("transaction id: %s\n", transaction_id)
							return nil
						},
					},
					{
						Name: "articles",
						Usage: `list published articles for a wallet id; list may be incomplete as it will only store articles published singed by this wallet instance
						use "cardano-db records" or "article index show" to see all articles`,
						Action: func(cli *cli.Context) error {
							articles, err := config.ListSignedArticles(cli.Args().First())
							if err != nil {
								return err
							}
							printJSON(articles)
							return nil
						},
					},
				},
			},
			{
				Name:  "run",
				Usage: "Run the curator or web services",
				Subcommands: []*cli.Command{
					{
						Name:  "daemon",
						Usage: "run the curator daemon which pulls articles from the cardano blockchain",
						Action: func(cli *cli.Context) error {
							return errors.New("not implemented")
						},
					},
					{
						Name:  "server",
						Usage: "run curator web server",
						Flags: []cli.Flag{
							&cli.IntFlag{
								Name:    "port",
								Aliases: []string{"p"},
								Usage:   "The port to serve on",
								Value:   1323,
							},
						},
						Action: func(cli *cli.Context) error {
							server := dbranch.NewCuratorServer(config)
							err := server.Start(fmt.Sprintf(":%d", cli.Int("port")))
							server.Logger.Fatal(err)
							return err
						},
					},
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}

}

func printJSON(data interface{}) {
	indented, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(indented))
}
