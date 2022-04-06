package main

import (
	"fmt"

	shell "github.com/ipfs/go-ipfs-api"
)

func main() {
	// Where your local node is running on localhost:5001
	sh := shell.NewShell("localhost:5001")
	id, err := sh.ID()
	if err != nil {
		panic(err)
	}
	fmt.Printf("local id %s\n", id.ID)
}
