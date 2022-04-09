package main

import (
	"log"

	dbranch "github.com/b-rad-c/dbranch-backend/dbranch"
)

func main() {
	wire := dbranch.WireSubFromEnv()

	wire.VerifyCanRun()

	wire.WaitForService()

	wire.SubscribeLoop()

	log.Println("exiting 0")
}
