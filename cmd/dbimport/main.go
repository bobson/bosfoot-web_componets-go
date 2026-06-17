// Command dbimport loads the SQL files into the Aiven database, in order.
// Run from the project root so the relative db/ paths resolve:
//
//	go run ./cmd/dbimport
//
// All files are idempotent (ON CONFLICT DO NOTHING), so re-running is safe.
package main

import (
	"fmt"
	"log"
	"os"

	"bosfoot/internal/database"
)

func main() {
	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Skip schema + seed if the DB is already initialised (lang_code type exists).
	// This makes re-running safe after the first setup.
	var initialized bool
	if err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_type WHERE typname = 'lang_code')").Scan(&initialized); err != nil {
		log.Fatalf("init check: %v", err)
	}

	var files []string
	if !initialized {
		files = append(files, "db/schema.sql", "db/seed.sql")
	} else {
		fmt.Println("schema already initialised, skipping schema.sql + seed.sql")
	}
	files = append(files,
		"db/freet.sql",
		"db/freet-update.sql",
		"db/price-eur.sql",
	)

	for _, f := range files {
		sqlBytes, err := os.ReadFile(f)
		if err != nil {
			log.Fatalf("read %s: %v", f, err)
		}
		// lib/pq's simple query protocol runs every semicolon-separated
		// statement in a single Exec (works because we pass no placeholders),
		// so no fragile manual statement splitting is needed.
		if _, err := db.Exec(string(sqlBytes)); err != nil {
			log.Fatalf("exec %s: %v", f, err)
		}
		fmt.Printf("✓ %s\n", f)
	}

	fmt.Println("Import complete.")
}
