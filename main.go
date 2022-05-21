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

func main() {

	app := &cli.App{
		Name:    "dBranch Backend",
		Usage:   "Curate articles from the dBranch news protocol!",
		Version: "0.1.0",
		Commands: []*cli.Command{
			{
				Name:  "article",
				Usage: "Interact with curated articles",
				Subcommands: []*cli.Command{
					{
						Name:  "get",
						Usage: "get a curated article",
						Action: func(cli *cli.Context) error {
							name := cli.Args().First()
							if name == "" {
								return errors.New("missing article name")
							}
							record, article, err := dbranch.GetArticle(name)
							if err != nil {
								return err
							}
							printJSON(&dbranch.FullArticle{Article: article, Record: record})
							return nil
						},
					},
					{
						Name:  "list",
						Usage: "list curated articles",
						Action: func(cli *cli.Context) error {
							names, err := dbranch.ListArticles()
							if err != nil {
								return err
							}
							printJSON(names)
							return nil
						},
					},
					{
						Name:  "index",
						Usage: "refresh or view the article index which lists curated and published (signed w cardano) articles",
						Subcommands: []*cli.Command{
							{
								Name:  "show",
								Usage: "show the article index",
								Action: func(cli *cli.Context) error {
									index, err := dbranch.LoadArticleIndex()
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
									return dbranch.RefreshArticleIndex()
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
							db_sync, err := dbranch.CardanoDBSyncStatus()
							if err != nil {
								return err
							}
							fmt.Printf("{\"sync_progress\": %f}\n", *db_sync)
							return nil
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
								Name:    "tx_hash",
								Aliases: []string{"tx"},
								Usage:   "filter records by transaction hash",
							},
						},
						Action: func(cli *cli.Context) error {
							address := cli.String("address")
							tx_hash := cli.String("tx_hash")
							var err error
							var record dbranch.CardanoArticleRecord
							var records []dbranch.CardanoArticleRecord

							if address != "" && tx_hash != "" {
								return errors.New("cannot specify both address and tx_hash")
							} else if address != "" {
								records, err = dbranch.CardanoRecordsByAddress(address)
							} else if tx_hash != "" {
								record, err = dbranch.CardanoRecordsByTxHash(tx_hash)
								records = []dbranch.CardanoArticleRecord{record}
							} else {
								records, err = dbranch.CardanoRecords()
							}

							if err != nil {
								return err
							}
							printJSON(records)
							return nil
						},
					},
					{
						Name:      "add-tx",
						Usage:     "add article to curated list by cardano tx hash",
						ArgsUsage: "add-tx [tx_hash]",
						Action: func(cli *cli.Context) error {
							tx_hash := cli.Args().First()
							if tx_hash == "" {
								return fmt.Errorf("missing tx_hash")
							}
							return dbranch.AddRecordByCardanoTxHash(tx_hash)
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
							status, err := dbranch.Status()
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
							dbranch.WaitForCardano()
							return nil
						},
					},
					{
						Name:  "list",
						Usage: "list available wallets by id",
						Action: func(cli *cli.Context) error {
							wallets, err := dbranch.WalletIds()
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
							wallet_id := cli.Args().First()
							if wallet_id == "" {
								return fmt.Errorf("missing wallet id")
							}
							addresses, err := dbranch.WalletAddresses(wallet_id)
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
							transaction_id, err := dbranch.SignArticle(args[0], args[1], args[2], args[3])
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
							articles, err := dbranch.ListSignedArticles(cli.Args().First())
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
						Action: func(cli *cli.Context) error {
							return dbranch.CuratorServer()
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
