package curator

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

var client = &http.Client{}

func init() {
	client = &http.Client{Timeout: 30 * time.Second}
}

//
// models
//

type CardanoString struct {
	String string `json:"string"`
}

type CardanoKeyValue struct {
	Key   CardanoString `json:"k"`
	Value CardanoString `json:"v"`
}

type CardanoMap struct {
	Map []CardanoKeyValue `json:"map"`
}

type CardanoArticleMetadata struct {
	Label CardanoMap `json:"451"`
}

type CardanoArticle struct {
	TransactionID string `json:"transaction_id"`
	Name          string `json:"name"`
	Location      string `json:"location"`
}

type Transaction struct {
	ID        string                 `json:"id"`
	Direction string                 `json:"direction"`
	Status    string                 `json:"status"`
	Metadata  CardanoArticleMetadata `json:"metadata"`
}

func (c *Curator) getRequest(endpoint string) (interface{}, error) {
	url := c.Config.CardanoWalletHost + endpoint

	var data interface{}

	resp, err := client.Get(url)
	if err != nil {
		return data, errors.New(url + " returned error: " + err.Error())
	}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return data, errors.New("error decoding config file: " + err.Error())
	}

	return data, nil
}

func (c *Curator) WalletStatus() (string, error) {
	type SyncProgress struct {
		Status string `json:"status"`
	}

	type NetworkInformation struct {
		SyncProgress SyncProgress `json:"sync_progress"`
	}

	info := &NetworkInformation{}
	url := c.Config.CardanoWalletHost + "/v2/network/information"

	resp, err := client.Get(url)
	if err != nil {
		return "", errors.New(url + " returned error: " + err.Error())
	}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&info)
	if err != nil {
		return "", errors.New("error decoding config file: " + err.Error())
	}

	return info.SyncProgress.Status, nil
}

func (c *Curator) WalletIds() ([]string, error) {
	var wallet_ids []string

	resp, err := c.getRequest("/v2/wallets")
	if err != nil {
		return wallet_ids, err
	}

	for _, wallet := range resp.([]interface{}) {
		wallet_ids = append(wallet_ids, wallet.(map[string]interface{})["id"].(string))
	}

	return wallet_ids, nil
}

func (c *Curator) WalletTransactions(wallet_id string) ([]Transaction, error) {
	transactions := make([]Transaction, 0)

	url := c.Config.CardanoWalletHost + "/v2/wallets/" + wallet_id + "/transactions"

	resp, err := client.Get(url)
	if err != nil {
		return transactions, errors.New(url + " returned error: " + err.Error())
	}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&transactions)
	if err != nil {
		return transactions, errors.New("error decoding config file: " + err.Error())
	}

	return transactions, nil
}

func (c *Curator) WalletArticles(wallet_id string) ([]CardanoArticle, error) {
	transactions := make([]Transaction, 0)
	articles := make([]CardanoArticle, 0)

	url := c.Config.CardanoWalletHost + "/v2/wallets/" + wallet_id + "/transactions"

	resp, err := client.Get(url)
	if err != nil {
		return articles, errors.New(url + " returned error: " + err.Error())
	}

	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&transactions)
	if err != nil {
		return articles, errors.New("error decoding config file: " + err.Error())
	}

	for _, transaction := range transactions {
		if transaction.Status == "in_ledger" && transaction.Direction == "outgoing" {
			if transaction.Metadata.Label.Map != nil {
				name := ""
				location := ""

				for _, keyValue := range transaction.Metadata.Label.Map {
					if keyValue.Key.String == "name" {
						name = keyValue.Value.String
					} else if keyValue.Key.String == "loc" {
						location = keyValue.Value.String
					}
				}

				if name == "" || location == "" {
					continue
				}

				articles = append(articles, CardanoArticle{
					transaction.ID,
					name,
					location,
				})
			}
		}
	}

	return articles, nil
}
