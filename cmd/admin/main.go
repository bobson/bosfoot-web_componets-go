// Command admin is the owner's LOCAL admin: a small web UI on the laptop
// (http://localhost:8090) over the same database the terminal tools use. It is
// never deployed — there is no login because it only listens on 127.0.0.1.
//
//	go run ./cmd/admin            # open http://localhost:8090
//	bash scripts/localdb.sh go run ./cmd/admin   # against the sandbox DB
//
// Catalog: edit data/ (price, sale, mode, published, featured, order, brand
// visibility, colours, sizes) then Preview/Apply = cmd/shoeimport for the brand
// (never -stock). Apply is blocked while a published model has a colour with no
// photos locally or on the live site.
//
// Stock: live per model × colour × size from the DB. +/− or typing only STAGES
// changes; Publish shows them for confirmation and writes just those cells.
//
// Reviews: Pending / Approved / Rejected, with Approve and Reject. A review with
// photos is moderated on the server over SSH (photos live in its UPLOADS_DIR).
//
// Orders: Active / Finished / Cancelled / Test tabs; mark shipped, delivered,
// cancelled (stock lines go back on the shelf); flag test orders, and
// permanently delete only those; send the review request by email or Viber
// (same links/text as cmd/reviewinvites — sending one marks the order delivered).
//
// Safety: besides binding to loopback, every request must carry the right Host
// (blocks DNS-rebinding) and every POST a per-run secret (blocks other sites in
// the browser from submitting forms to localhost).
package main

import (
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"bosfoot/internal/database"
	"bosfoot/internal/notify"
	"bosfoot/internal/site"
	"bosfoot/logger"
)

//go:embed ui/*.html
var uiFS embed.FS

type app struct {
	db      *sql.DB
	mailer  *notify.Mailer
	siteURL string
	secret  string // per-run token every POST must carry
	tmpl    *template.Template
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8090", "listen address (keep it on 127.0.0.1)")
	open := flag.Bool("open", true, "open the admin in the browser on start")
	flag.Parse()

	host, _, err := net.SplitHostPort(*addr)
	if err != nil || (host != "127.0.0.1" && host != "localhost" && host != "::1") {
		log.Fatalf("-addr must be a loopback address (got %q) — the admin has no login", *addr)
	}

	db, err := database.Connect() // also loads .env
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var hasTest bool
	if err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_name = 'orders' AND column_name = 'is_test')`).Scan(&hasTest); err != nil {
		log.Fatal(err)
	}
	if !hasTest {
		log.Fatal("orders.is_test is missing — run `go run ./cmd/dbimport` first (adds db/admin.sql)")
	}

	siteURL := strings.TrimRight(os.Getenv("SITE_URL"), "/")
	if siteURL == "" {
		siteURL = "http://localhost:8080"
	}
	lg, err := logger.NewLogger("bosfoot.log")
	if err != nil {
		log.Fatal(err)
	}
	defer lg.Close()

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		log.Fatal(err)
	}
	a := &app{db: db, mailer: notify.New(lg), siteURL: siteURL, secret: hex.EncodeToString(b)}
	a.tmpl = template.Must(template.New("").Funcs(template.FuncMap{
		"orderNum": site.OrderNumber,
		"mkd":      fmtMKD,
		"orderRef": orderLabel,
		"dec":      func(n int) int { return n - 1 },
	}).ParseFS(uiFS, "ui/*.html"))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/orders", http.StatusFound)
	})
	mux.HandleFunc("GET /orders", a.orders)
	mux.HandleFunc("POST /orders/{id}/status", a.setStatus)
	mux.HandleFunc("POST /orders/{id}/test", a.toggleTest)
	mux.HandleFunc("POST /orders/{id}/delete", a.deleteTest)
	mux.HandleFunc("POST /orders/{id}/review/email", a.reviewEmail)
	mux.HandleFunc("POST /orders/{id}/review/viber", a.reviewViber)
	mux.HandleFunc("GET /catalog", a.catalog)
	mux.HandleFunc("GET /catalog/{brand}/{slug}", a.modelPage)
	mux.HandleFunc("POST /catalog/{brand}/visible", a.toggleBrand)
	mux.HandleFunc("POST /catalog/{brand}/apply", a.apply)
	mux.HandleFunc("POST /catalog/{brand}/{slug}/settings", a.saveSettings)
	mux.HandleFunc("POST /catalog/{brand}/{slug}/color/add", a.addColor)
	mux.HandleFunc("POST /catalog/{brand}/{slug}/color/remove", a.removeColor)
	mux.HandleFunc("POST /catalog/{brand}/{slug}/color/primary", a.primaryColor)
	mux.HandleFunc("POST /catalog/{brand}/{slug}/size/add", a.addSize)
	mux.HandleFunc("POST /catalog/{brand}/{slug}/size/remove", a.removeSize)
	mux.HandleFunc("GET /stock", a.stock)
	mux.HandleFunc("POST /stock/publish", a.stockPublish)
	mux.HandleFunc("GET /reviews", a.reviews)
	mux.HandleFunc("POST /reviews/{id}/approve", a.moderate("approve"))
	mux.HandleFunc("POST /reviews/{id}/reject", a.moderate("reject"))

	url := "http://" + *addr
	fmt.Printf("Bosfoot admin on %s  (database: %s)\nCtrl+C to stop.\n", url, dbLabel())
	if *open {
		_ = exec.Command("xdg-open", url).Start()
	}
	log.Fatal(http.ListenAndServe(*addr, a.guard(*addr, mux)))
}

// guard rejects requests whose Host isn't our own address (DNS rebinding) and
// POSTs without the per-run secret (cross-site form posts to localhost).
func (a *app) guard(addr string, next http.Handler) http.Handler {
	_, port, _ := net.SplitHostPort(addr)
	okHosts := map[string]bool{addr: true, "localhost:" + port: true, "127.0.0.1:" + port: true}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !okHosts[r.Host] {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if r.Method == http.MethodPost && r.FormValue("k") != a.secret {
			http.Error(w, "stale page — reload the admin and try again", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// page adds what every page's header needs: the DB label, the active nav item,
// the pending-review count (nav badge) and the per-run POST secret.
func (a *app) page(nav string, data map[string]any) map[string]any {
	var pending int
	_ = a.db.QueryRow(`SELECT count(*) FROM reviews WHERE status = 'pending'`).Scan(&pending)
	data["DB"], data["Nav"], data["PendingReviews"], data["K"] = dbLabel(), nav, pending, a.secret
	return data
}

// dbLabel says which database the admin is pointed at, so a sandbox session is
// never mistaken for production.
func dbLabel() string {
	u := os.Getenv("DATABASE_URL")
	if strings.Contains(u, "localhost") || strings.Contains(u, "127.0.0.1") {
		return "LOCAL SANDBOX"
	}
	return "PRODUCTION"
}

// fmtMKD formats 18460 as "18.460".
func fmtMKD(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return s
}
