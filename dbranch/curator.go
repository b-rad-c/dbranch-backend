package dbranch

import (
	"bufio"
	"io"
	"log"
	"os"
	"path"
	"strconv"
	"time"
)

// filepath to store last block number between executions of the curator daemon
var last_block_file string

func init() {
	home_dir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("cannot find user home dir: " + err.Error())
	}
	dbranch_dir := path.Join(home_dir, ".dbranch")
	os.Mkdir(dbranch_dir, 0755)
	if err != nil && !os.IsExist(err) {
		log.Fatal(err)
	}
	last_block_file = path.Join(dbranch_dir, "last_block")
}

func ListCardanoAddresses() ([]string, error) {
	address_path := os.Getenv("CARDANO_ADDRESS_FILE")
	if address_path == "" {
		address_path = "./samples/cardano_addresses.txt"
	}

	file, err := os.Open(address_path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	addresses := []string{}
	reader := bufio.NewReader(file)

	for {
		line, _, err := reader.ReadLine()

		if err == io.EOF {
			break
		}

		addresses = append(addresses, string(line))
	}

	return addresses, nil
}

func loadLastBlock() uint {
	data, err := os.ReadFile(last_block_file)
	if os.IsNotExist(err) {
		return 0
	} else if err != nil {
		log.Fatal("can't read last block file: ", err)
	}

	block_no, err := strconv.ParseUint(string(data), 10, 32)

	if err != nil {
		log.Fatal("can't parse last block file: ", err)
	}

	log.Printf("loaded last block number: %d from: %s\n", block_no, last_block_file)
	return uint(block_no)
}

func saveLastBlock(block_no uint) {
	file, err := os.Create(last_block_file)
	defer file.Close()

	if err != nil {
		log.Printf("can't create last block file: %s\n", err)
	}

	block_as_str := strconv.FormatUint(uint64(block_no), 10)
	_, err = file.WriteString(block_as_str)
	if err != nil {
		log.Printf("error writing to last block file: %s\n", err)
	} else {
		log.Printf("saved block number: %d to: %s\n", block_no, last_block_file)
	}
}

func CuratorDaemon() {
	log.Println("Cardano curator daemon starting")

	addrs, err := ListCardanoAddresses()
	if err != nil {
		log.Fatalf("could not list addresses: %s", err)
	}

	log.Printf("found %d addresses", len(addrs))

	WaitForCardanoDB()

	log.Println("entering curator loop")

	block_no := loadLastBlock()
	var refresh bool

	for {

		refresh = false

		for _, addr := range addrs {
			records, err := ListCardanoRecords(AddressFilter(addr), SinceBlockFilter(block_no))
			if err != nil {
				log.Printf("could not list records: %s", err)
				continue
			}

			for _, record := range records {
				log.Printf("adding record from hash: %s\n", record.TxHash)
				block_no = record.BlockNumber
				log.Printf("new block_no: %d\n", block_no)
				saveLastBlock(block_no)
				refresh = true
			}
		}

		if refresh {
			err = RefreshArticleIndex()
			if err != nil {
				log.Printf("could not refresh article index: %s", err)
			}
		}

		time.Sleep(time.Second * 20)
	}
}
