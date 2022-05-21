package dbranch

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB

func fileValue(file string) string {
	data, err := ioutil.ReadFile("read.go")
	if err != nil {
		panic(err)
	}
	return string(data)
}

func init() {
	var err error

	db_host, db_name, pw, user := "", "", "", ""

	db_host = os.Getenv("POSTGRES_DB_HOST")
	if db_host == "" {
		db_host = "localhost"
	}

	db_file := os.Getenv("POSTGRES_DB_FILE")
	if db_file == "" {
		db_name = "cexplorer"
	} else {
		db_name = fileValue(db_file)
	}

	pw_file := os.Getenv("POSTGRES_PASSWORD_FILE")
	if pw_file == "" {
		// * * * this is a test pw from the cardano-db-sync repo * * *
		pw = "v8hlDV0yMAHHlIurYupj"
	} else {
		pw = fileValue(pw_file)
	}

	user_file := os.Getenv("POSTGRES_USER_FILE")
	if user_file == "" {
		user = "postgres"
	} else {
		user = fileValue(user_file)
	}

	ssl := os.Getenv("POSTGRES_SSL_MODE")
	if ssl == "" {
		ssl = "disable"
	}

	// conn_str := fmt.Sprintf("user=%s dbname=%s password=%s sslmode=%s", user, db_name, pw, ssl)
	conn_str := fmt.Sprintf("postgresql://%s:%s@%s/%s?sslmode=%s", user, pw, db_host, db_name, ssl)

	db, err = sql.Open("postgres", conn_str)
	if err != nil {
		panic(err)
	}
}

type CardanoArticleRecord struct {
	Name          string       `json:"name"`
	Location      string       `json:"location"`
	Address       string       `json:"address"`
	TxId          int64        `json:"tx_id"`
	TxHash        string       `json:"tx_hash"`
	TxHashRaw     sql.RawBytes `json:"tx_hash_raw"`
	DatePublished time.Time    `json:"date_published"`
}

type DBMeta struct {
	ID          int64     `json:"id"`
	StartTime   time.Time `json:"start_time"`
	NetworkName string    `json:"network_name"`
	Version     string    `json:"version"`
}

func formatRecordRows(rows *sql.Rows) ([]CardanoArticleRecord, error) {
	records := []CardanoArticleRecord{}

	for rows.Next() {
		record := CardanoArticleRecord{}
		err := rows.Scan(&record.Name, &record.Location, &record.Address, &record.TxId, &record.TxHashRaw, &record.DatePublished)
		if err != nil {
			return records, err
		}
		record.TxHash = hex.EncodeToString(record.TxHashRaw)
		records = append(records, record)
	}

	return records, nil
}

//
// db actions
//

func CardanoDBPing() error {
	return db.Ping()
}

func CardanoDBMeta() (*DBMeta, error) {
	db_meta := &DBMeta{}

	rows, err := db.Query("select * from meta")
	defer rows.Close()
	if err != nil {
		return db_meta, err
	}

	for rows.Next() {
		err = rows.Scan(&db_meta.ID, &db_meta.StartTime, &db_meta.NetworkName, &db_meta.Version)
		if err != nil {
			// there will only ever be one entry in this table so we can return without iterating the whole range
			break
		}
	}

	return db_meta, nil
}

func CardanoDBSyncStatus() (*float32, error) {
	var percent float32

	err := db.QueryRow(`select
	100 * (extract (epoch from (max (time) at time zone 'UTC')) - extract (epoch from (min (time) at time zone 'UTC')))
	/ (extract (epoch from (now () at time zone 'UTC')) - extract (epoch from (min (time) at time zone 'UTC')))
   	as sync_percent from block ;`).Scan(&percent)

	return &percent, err
}

func CardanoRecords(filters ...string) ([]CardanoArticleRecord, error) {

	query := `SELECT tx_metadata.json->>'name', tx_metadata.json->>'loc', tx_out.address, tx.id, tx.hash, block.time
	FROM ((tx_metadata INNER JOIN tx ON tx_metadata.tx_id = tx.id) INNER JOIN block ON tx.block_id = block.id) INNER JOIN tx_out ON tx.id = tx_out.tx_id
	WHERE tx_metadata.key = '451' AND tx_metadata.json->>'name' IS NOT NULL AND tx_metadata.json->>'loc' IS NOT NULL AND tx_out.index = 0`

	rows, err := db.Query(query)
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	return formatRecordRows(rows)
}

func CardanoRecordsByAddress(addr string) ([]CardanoArticleRecord, error) {
	query := `SELECT tx_metadata.json->>'name', tx_metadata.json->>'loc', tx_out.address, tx.id, tx.hash, block.time
	FROM ((tx INNER JOIN tx_metadata ON tx.id = tx_metadata.tx_id) INNER JOIN tx_out ON tx.id = tx_out.tx_id) INNER JOIN block ON tx.block_id = block.id
	WHERE tx_metadata.key = '451' AND tx_metadata.json->>'name' IS NOT NULL AND tx_metadata.json->>'loc' IS NOT NULL AND tx_out.index = 0
		AND tx_out.address = $1;`

	rows, err := db.Query(query, addr)
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	return formatRecordRows(rows)
}

func CardanoRecordsByTxHash(tx_hash string) (CardanoArticleRecord, error) {
	query := `SELECT tx_metadata.json->>'name', tx_metadata.json->>'loc', tx_out.address, tx.id, tx.hash, block.time
	FROM ((tx INNER JOIN tx_metadata ON tx_metadata.tx_id = tx.id) INNER JOIN block ON tx.block_id = block.id) INNER JOIN tx_out ON tx.id = tx_out.tx_id
	WHERE tx_metadata.key = '451' AND tx_metadata.json->>'name' IS NOT NULL AND tx_metadata.json->>'loc' IS NOT NULL AND tx_out.index = 0
		AND tx.hash = $1;
	`
	record := CardanoArticleRecord{}

	tx_raw, err := hex.DecodeString(tx_hash)
	if err != nil {
		return record, err
	}
	rows, err := db.Query(query, tx_raw)
	defer rows.Close()
	if err != nil {
		return record, err
	}

	results, err := formatRecordRows(rows)
	if err != nil {
		return record, err
	}

	if len(results) == 0 {
		return record, errors.New("could not find an article with tx hash: " + tx_hash)
	}

	return results[0], nil
}

func AddRecordByCardanoTxHash(tx_hash string) error {
	record, err := CardanoRecordsByTxHash(tx_hash)
	if err != nil {
		return err
	}

	return AddCardanoRecordToLocal(&record)
}

func AddCardanoRecordToLocal(record *CardanoArticleRecord) error {

	if !strings.HasPrefix(record.Location, "ipfs://") {
		return errors.New("invalid location: " + record.Location)
	}

	article := &ArticleRecord{
		Name:          record.Name,
		CID:           strings.Replace(record.Location, "ipfs://", "", 1),
		DatePublished: record.DatePublished,
		CardanoTxHash: record.TxHash,
	}

	return AddRecordToLocal(article)

}

/*
queries

- find article by name
select tx_metadata.json from tx_metadata where tx_metadata.key = '451' and tx_metadata.json->>'name' = 'dbranch_intro.news';

- list all articles
		SELECT tx_metadata.id, tx_metadata.json, tx_metadata.tx_id
		FROM tx_metadata
		WHERE tx_metadata.key = '451' AND tx_metadata.json->>'name' IS NOT NULL AND tx_metadata.json->>'loc' IS NOT NULL;

- articles with tx id
SELECT tx_metadata.json, tx_metadata.tx_id FROM tx_metadata WHERE tx_metadata.key = '451' AND tx_metadata.json->>'name' IS NOT NULL AND tx_metadata.json->>'loc' IS NOT NULL;

- articles with tx hash
SELECT tx.hash, tx_metadata.tx_id, tx_metadata.json
FROM tx_metadata
INNER JOIN tx ON tx_metadata.tx_id = tx.id
WHERE tx_metadata.key = '451' AND tx_metadata.json->>'name' IS NOT NULL AND tx_metadata.json->>'loc' IS NOT NULL;

-- article by tx hash
SELECT tx_metadata.json->>'name', tx_metadata.json->>'loc', tx.hash
FROM tx INNER JOIN tx_metadata ON tx_metadata.tx_id = tx.id
WHERE tx_metadata.key = '451' AND tx_metadata.json->>'name' IS NOT NULL AND tx_metadata.json->>'loc' IS NOT NULL
	AND tx.hash = '\xc40dbc63df9966a4704880cd0da79dfff68cc60fcce9bd04a35c81909ff7721a';

- articles for address
SELECT tx_metadata.json->>'name', tx_metadata.json->>'loc', tx_out.address, tx.hash
FROM (tx INNER JOIN tx_metadata ON tx.id = tx_metadata.tx_id) INNER JOIN tx_out ON tx.id = tx_out.tx_id
WHERE tx_metadata.key = '451' AND tx_metadata.json->>'name' IS NOT NULL AND tx_metadata.json->>'loc' IS NOT NULL
	AND tx_out.address = 'addr_test1qzp4lqggu2qfr2qs5plsjh8q7l9y3afcxzwwyfv3em2aqe0k69w3xsq4ruy5tenk59cshs2m26ftpdvacmqcn7yfljps7zazwv';

-- tx in for a transaction
SELECT * FROM tx_in WHERE tx_in.tx_out_id = 4037362;

-- tx outs for a transaction
SELECT tx_out.id, tx_out.tx_id, tx_out.index, tx_out.address, tx_out.value FROM tx_out WHERE tx_out.tx_id = 4041637;

-- tx outs for a transaction hash
SELECT tx.id, tx_out.id, tx_out.index, tx_out.address, tx_out.value
FROM tx_out
INNER JOIN tx ON tx_out.tx_id = tx.id
WHERE tx.hash = '\xc40dbc63df9966a4704880cd0da79dfff68cc60fcce9bd04a35c81909ff7721a';

-- address for a transaction hash
SELECT tx.id, tx_out.id, tx_out.index, tx_out.address, tx_out.value
FROM tx_out
INNER JOIN tx ON tx_out.tx_id = tx.id
WHERE tx.hash = '\xc40dbc63df9966a4704880cd0da79dfff68cc60fcce9bd04a35c81909ff7721a' AND tx_out.index = 0;

-- tx outs for an address
SELECT tx_out.id, tx_out.tx_id, tx_out.index, tx_out.address, tx_out.value FROM tx_out WHERE tx_out.address = 'addr_test1qzp4lqggu2qfr2qs5plsjh8q7l9y3afcxzwwyfv3em2aqe0k69w3xsq4ruy5tenk59cshs2m26ftpdvacmqcn7yfljps7zazwv';

-- spent tx outs for an address
SELECT tx_out.id, tx_out.tx_id, tx_out.address, tx_out.value FROM tx_out WHERE tx_out.address = 'addr_test1qzp4lqggu2qfr2qs5plsjh8q7l9y3afcxzwwyfv3em2aqe0k69w3xsq4ruy5tenk59cshs2m26ftpdvacmqcn7yfljps7zazwv';

*/
