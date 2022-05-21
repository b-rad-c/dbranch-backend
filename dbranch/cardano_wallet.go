package dbranch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/ssh/terminal"
)

//
// http
//

var client = &http.Client{}
var wallet_host string

func init() {
	client = &http.Client{Timeout: 30 * time.Second}
	wallet_host = os.Getenv("CARDANO_WALLET_HOST")
	if wallet_host == "" {
		wallet_host = "http://localhost:8090"
	}
}

func getRequest(endpoint string) (interface{}, error) {
	url := wallet_host + endpoint

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

type walletError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

//
// cardano primitives
//

type cborString struct {
	String string `json:"string"`
}

type cborKeyValue struct {
	Key   cborString `json:"k"`
	Value cborString `json:"v"`
}

type cborMap struct {
	Map []cborKeyValue `json:"map"`
}

type CardanoAddress struct {
	ID    string `json:"id"`
	State string `json:"state"`
}

type CardanoTransaction struct {
	ID        string                     `json:"id"`
	Direction string                     `json:"direction"`
	Status    string                     `json:"status"`
	Metadata  articleTransactionMetadata `json:"metadata"`
}

type transactionAmount struct {
	Quantity int    `json:"quantity"`
	Unit     string `json:"unit"`
}

type transactionPayment struct {
	Address string            `json:"address"`
	Amount  transactionAmount `json:"amount"`
}

type transactionRequest struct {
	Passphrase string                     `json:"passphrase"`
	Payments   []transactionPayment       `json:"payments"`
	Metadata   articleTransactionMetadata `json:"metadata"`
}

//
// published articles
//

type articleTransactionMetadata struct {
	Label cborMap `json:"451"`
}

type ArticleTransaction struct {
	TransactionID string `json:"transaction_id"`
	Name          string `json:"name"`
	Location      string `json:"loc"`
	Status        string `json:"status"`
}

//
// wallet apis
//

func WalletIds() ([]string, error) {
	var wallet_ids []string

	resp, err := getRequest("/v2/wallets")
	if err != nil {
		return wallet_ids, err
	}

	for _, wallet := range resp.([]interface{}) {
		wallet_ids = append(wallet_ids, wallet.(map[string]interface{})["id"].(string))
	}

	return wallet_ids, nil
}

func WalletAddresses(wallet_id string) ([]CardanoAddress, error) {
	var addresses []CardanoAddress

	resp, err := getRequest("/v2/wallets/" + wallet_id + "/addresses")
	if err != nil {
		return addresses, err
	}

	for _, address := range resp.([]interface{}) {
		addr := CardanoAddress{
			ID:    address.(map[string]interface{})["id"].(string),
			State: address.(map[string]interface{})["state"].(string),
		}

		addresses = append(addresses, addr)
	}

	return addresses, nil
}

func walletTransactions(wallet_id string) ([]CardanoTransaction, error) {
	// init
	transactions := make([]CardanoTransaction, 0)

	url := wallet_host + "/v2/wallets/" + wallet_id + "/transactions"

	// request
	resp, err := client.Get(url)
	if err != nil {
		return transactions, errors.New(url + " returned error: " + err.Error())
	}

	defer resp.Body.Close()

	// decode
	err = json.NewDecoder(resp.Body).Decode(&transactions)
	if err != nil {
		return transactions, errors.New("error decoding config file: " + err.Error())
	}

	return transactions, nil
}

//
// article signing
//

func ListSignedArticles(wallet_id string) ([]ArticleTransaction, error) {

	transactions, err := walletTransactions(wallet_id)
	if err != nil {
		return nil, err
	}

	articles := []ArticleTransaction{}

	// parse response and filter non articles
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

				articles = append(articles, ArticleTransaction{
					transaction.ID,
					name,
					location,
					transaction.Status,
				})
			}
		}
	}

	return articles, nil
}

func SignArticle(wallet_id, address, article_name, location string) (string, error) {
	// get user password
	fmt.Println("Cardano wallet password: ")
	password, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", errors.New("error reading password: " + err.Error())
	}
	fmt.Println() // needed to clear the password from the terminal

	// format request
	body := transactionRequest{
		Passphrase: string(password),
		Payments: []transactionPayment{
			{
				Address: address,
				Amount: transactionAmount{
					Quantity: 1000000,
					Unit:     "lovelace",
				},
			},
		},
		Metadata: articleTransactionMetadata{
			Label: cborMap{
				Map: []cborKeyValue{
					{
						Key:   cborString{String: "name"},
						Value: cborString{String: article_name},
					},
					{
						Key:   cborString{String: "loc"},
						Value: cborString{String: location},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return "", errors.New("Error encoding json body: " + err.Error())
	}

	url := wallet_host + "/v2/wallets/" + wallet_id + "/transactions"
	resp, err := client.Post(url, "application/json; charset=UTF-8", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", errors.New(url + " returned error: " + err.Error())
	}

	defer resp.Body.Close()

	if resp.StatusCode == 202 {
		// request status accepted
		var data interface{}

		err = json.NewDecoder(resp.Body).Decode(&data)
		if err != nil {
			return "", errors.New("error decoding config file: " + err.Error())
		}

		id := data.(map[string]interface{})["id"].(string)
		return id, nil

	} else {
		// handle error
		var err_msg = walletError{}
		err = json.NewDecoder(resp.Body).Decode(&err_msg)
		if err != nil {
			return "", errors.New(fmt.Sprintf("got status %v and error decoding resp: %v", resp.Status, err.Error()))
		}

		return "", errors.New(fmt.Sprintf("%v - %v - %v", resp.Status, err_msg.Code, err_msg.Message))

	}

}

//
// Daemon
//

func Status() (string, error) {
	resp, err := getRequest("/v2/network/information")
	if err != nil {
		return "", err
	}

	sync := resp.(map[string]interface{})
	status := sync["sync_progress"].(map[string]interface{})["status"].(string)
	return status, nil
}

func WaitForCardano() {
	status := ""
	for status != "ready" {
		status, _ = Status()
		time.Sleep(time.Second * 5)
	}
}
