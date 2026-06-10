// Package database builds the Aiven PostgreSQL connection used by the
// cmd/ utilities (and, later, the web server). Connection details come from
// the environment / .env so no secrets live in source.
package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// Connect loads .env, builds a TLS-verified connection string for Aiven,
// opens the pool and verifies it with a ping.
//
// Required env:
//   DATABASE_URL  full Aiven service URI (postgres://user:pass@host:port/db?...)
//   PG_CA_CERT    path to the Aiven CA certificate (relative to the working dir)
func Connect() (*sql.DB, error) {
	_ = godotenv.Load() // .env is optional; vars may already be exported

	serviceURI := os.Getenv("DATABASE_URL")
	if serviceURI == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set (see .env)")
	}

	conn, err := url.Parse(serviceURI)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}

	// Aiven requires TLS. verify-ca checks the server cert against the CA
	// we downloaded, which is stronger than the default sslmode=require.
	q := conn.Query()
	q.Set("sslmode", "verify-ca")
	if ca := os.Getenv("PG_CA_CERT"); ca != "" {
		q.Set("sslrootcert", ca)
	}
	conn.RawQuery = q.Encode()

	db, err := sql.Open("postgres", conn.String())
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Pool tuning, sized to the Aiven plan's hard cap. This plan reports
	// max_connections=20 with 3 reserved for superuser → only 17 usable for
	// the app. We must stay well under that to leave room for: a second
	// instance during a rolling deploy (old + new overlap), admin tools
	// (cmd/dbimport, cmd/dbping), and a local dev server pointed at the same DB.
	//
	// 8 open per instance means two instances (16) still fit under 17. A few
	// idle connections stay warm to skip the TLS handshake to Frankfurt on the
	// common path; the rest are released quickly so we don't hog a shared,
	// tiny connection budget. The product detail handler fans out queries
	// concurrently but caps its own concurrency (errgroup SetLimit) so a single
	// request never claims the whole pool.
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(90 * time.Second)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return db, nil
}
