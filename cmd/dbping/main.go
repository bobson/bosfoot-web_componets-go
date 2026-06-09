// Command dbping verifies the Aiven PostgreSQL connection.
//
//	go run ./cmd/dbping
package main

import (
	"fmt"
	"log"

	"bosfoot/internal/database"
)

func main() {
	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var version string
	if err := db.QueryRow("SELECT version()").Scan(&version); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Connected ✓\nPostgres version: %s\n", version)
}
