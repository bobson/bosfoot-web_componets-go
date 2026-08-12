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
		// Must run before the freet* size_chart seeds (which insert the new column
		// names): renames size_chart.foot_*→insole_* on existing DBs. Idempotent.
		"db/size-chart-insole.sql",
		"db/freet.sql",
		"db/freet-update.sql",
		"db/price-eur.sql",
		// Always run: migrates the reviews table to the guest-checkout model and
		// creates review_tokens. Idempotent, so safe on both fresh and existing DBs.
		"db/reviews.sql",
		// Always run (after the freet* seeds): replaces the placeholder size_chart
		// with Freet's official insole/width chart. Idempotent (DELETE + INSERT).
		"db/freet-sizechart.sql",
		// Always run: adds the 'cancelled' order status. Idempotent (ADD VALUE IF
		// NOT EXISTS), so safe on both fresh and existing DBs.
		"db/orders-status.sql",
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
