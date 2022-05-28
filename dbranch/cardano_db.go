package dbranch

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB

func fileValue(file string) string {
	data, err := ioutil.ReadFile(file)
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
		pw = fileValue("../secrets/postgres_password")
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

//
// db status
//

type DBMeta struct {
	ID          int64     `json:"id"`
	StartTime   time.Time `json:"start_time"`
	NetworkName string    `json:"network_name"`
	Version     string    `json:"version"`
}

type DBStatus struct {
	Percent       *float32      `json:"percent"`
	LastBlockTime *time.Time    `json:"last_block_time"`
	SecondsBehind time.Duration `json:"seconds_behind"`
	TimeBehind    string        `json:"time_behind"`
}

type DBBlockStatus struct {
	LastChainBlockNumber  uint `json:"last_chain_block_number"`
	LastDaemonBlockNumber uint `json:"last_daemon_block_number"`
	Difference            int  `json:"difference"`
}

// methods

func CardanoDBPing() error {
	return db.Ping()
}

func WaitForCardanoDB() {

	for {
		log.Println("checking if cardano db is ready")
		err := CardanoDBPing()
		if err == nil {
			break
		}
		time.Sleep(time.Second * 5)
	}
	log.Println("cardano db is ready")
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

func CardanoDBSyncStatus() (DBStatus, error) {
	status := DBStatus{}

	err := db.QueryRow(`select
	100 * (extract (epoch from (max (time) at time zone 'UTC')) - extract (epoch from (min (time) at time zone 'UTC')))
	/ (extract (epoch from (now () at time zone 'UTC')) - extract (epoch from (min (time) at time zone 'UTC')))
   	as sync_percent from block;`).Scan(&status.Percent)

	if err != nil {
		return status, err
	}

	err = db.QueryRow("select max(time) from block;").Scan(&status.LastBlockTime)
	if err != nil {
		return status, err
	}

	diff := time.Since(*status.LastBlockTime)
	status.SecondsBehind = diff / 1000000000
	status.TimeBehind = diff.String()
	return status, nil
}

func CardanoBlockStatus() (DBBlockStatus, error) {
	status := DBBlockStatus{}

	err := db.QueryRow("SELECT max(block_no) from block;").Scan(&status.LastChainBlockNumber)
	if err != nil {
		return status, err
	}

	status.LastDaemonBlockNumber = loadLastBlock()
	status.Difference = int(status.LastChainBlockNumber) - int(status.LastDaemonBlockNumber)

	return status, nil
}

//
// db records
//

type CardanoArticleRecord struct {
	Name          string       `json:"name"`
	Location      string       `json:"location"`
	Address       string       `json:"address"`
	BlockNumber   uint         `json:"block_number"`
	TxId          int64        `json:"tx_id"`
	TxHash        string       `json:"tx_hash"`
	TxHashRaw     sql.RawBytes `json:"tx_hash_raw"`
	DatePublished time.Time    `json:"date_published"`
}

type RecordFilter func(index int, query string, args []any) (string, []any, error)

func formatRecordRows(rows *sql.Rows) ([]CardanoArticleRecord, error) {
	records := []CardanoArticleRecord{}

	for rows.Next() {
		record := CardanoArticleRecord{}
		err := rows.Scan(
			&record.Name,
			&record.Location,
			&record.Address,
			&record.TxId,
			&record.TxHashRaw,
			&record.DatePublished,
			&record.BlockNumber,
		)
		if err != nil {
			return records, err
		}
		record.TxHash = hex.EncodeToString(record.TxHashRaw)
		records = append(records, record)
	}

	return records, nil
}

// filters

func AddressFilter(address string) RecordFilter {
	return func(index int, query string, args []any) (string, []any, error) {
		query = fmt.Sprintf("%s AND tx_out.address = $%d", query, index)
		return query, append(args, address), nil
	}
}

func TxHashFilter(tx_hash string) RecordFilter {
	return func(index int, query string, args []any) (string, []any, error) {
		query = fmt.Sprintf("%s AND tx.hash = $%d", query, index)
		tx_raw, err := hex.DecodeString(tx_hash)
		if err != nil {
			return query, args, err
		}
		return query, append(args, tx_raw), nil
	}
}

func SinceBlockFilter(block_number uint) RecordFilter {
	// filter for blocks where block_no > block_number
	return func(index int, query string, args []any) (string, []any, error) {
		query = fmt.Sprintf("%s AND block_no > $%d", query, index)
		return query, append(args, block_number), nil
	}
}

// methods

func ListCardanoRecords(filters ...RecordFilter) ([]CardanoArticleRecord, error) {

	query := `SELECT tx_metadata.json->>'name', tx_metadata.json->>'loc', tx_out.address, tx.id, tx.hash, block.time, block.block_no
	FROM ((tx_metadata INNER JOIN tx ON tx_metadata.tx_id = tx.id) INNER JOIN block ON tx.block_id = block.id) INNER JOIN tx_out ON tx.id = tx_out.tx_id
	WHERE tx_metadata.key = '451' AND tx_metadata.json->>'name' IS NOT NULL AND tx_metadata.json->>'loc' IS NOT NULL AND tx_out.index = 0`
	args := []any{}
	var err error

	for index, filter := range filters {
		query, args, err = filter(index+1, query, args)
		if err != nil {
			return []CardanoArticleRecord{}, err
		}
	}

	rows, err := db.Query(query, args...)
	defer rows.Close()
	if err != nil {
		return nil, err
	}

	return formatRecordRows(rows)
}

func AddRecordByCardanoTxHash(tx_hash string) error {
	record, err := ListCardanoRecords(TxHashFilter(tx_hash))
	if err != nil {
		return err
	}

	if len(record) == 0 {
		return errors.New("no record found for hash: " + tx_hash)
	}

	return AddCardanoRecordToLocal(&record[0])
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
