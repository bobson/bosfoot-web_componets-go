package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"bosfoot/handlers"
	"bosfoot/internal/cache"
	"bosfoot/internal/database"
	"bosfoot/internal/locale"
	"bosfoot/internal/tmpl"
	"bosfoot/logger"
)

func initializeLogger() *logger.Logger {
	logInstance, err := logger.NewLogger("bosfoot.log")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	// Close() is owned by main, which keeps the logger alive for the
	// lifetime of the process. (Previously deferred here, which closed the
	// log file before the server ever started.)
	return logInstance
}

func main() {

	// Initialize the logger
	logInstance := initializeLogger()
	defer logInstance.Close()

	// Connect to the database
	db, err := database.Connect()
	if err != nil {
		logInstance.Error("Failed to connect to database", err)
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	logInstance.Info("Connected to database")

	// Load locale strings and templates.
	ui, err := locale.LoadUI("public/locales")
	if err != nil {
		logInstance.Error("Failed to load locale strings", err)
		log.Fatalf("Failed to load locale strings: %v", err)
	}
	renderer, err := tmpl.NewRenderer("templates", ui)
	if err != nil {
		logInstance.Error("Failed to parse templates", err)
		log.Fatalf("Failed to parse templates: %v", err)
	}

	// API handlers (JSON, CSR)
	productHandler := &handlers.ProductHandler{
		DB:     db,
		Logger: logInstance,
	}
	http.HandleFunc("/api/brands", productHandler.GetBrands)
	http.HandleFunc("/api/products", productHandler.GetProducts)
	http.HandleFunc("/api/products/{id}", productHandler.GetProductByID)

	orderHandler := &handlers.OrderHandler{
		DB:     db,
		Logger: logInstance,
	}
	http.HandleFunc("/api/orders", orderHandler.CreateOrder)

	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "http://localhost:8080"
	}

	// Page handlers (HTML, SSR)
	pageHandler := &handlers.PageHandler{
		DB:       db,
		Logger:   logInstance,
		Renderer: renderer,
		SiteURL:  siteURL,
	}

	// In-memory page cache. These SSR pages are identical for every visitor
	// (the cart is client-side), so a 60s-TTL cache lets most requests skip the
	// DB queries and template rendering entirely. See internal/cache.
	pc := cache.New(60 * time.Second)
	for _, loc := range []string{"mk", "sq", "en"} {
		http.HandleFunc("/"+loc, pc.Wrap(pageHandler.Home))
	}
	http.HandleFunc("/{locale}/products", pc.Wrap(pageHandler.ProductListing))
	http.HandleFunc("/{locale}/products/{brand}/{slug}", pc.Wrap(pageHandler.ProductDetail))
	http.HandleFunc("/{locale}/checkout", pc.Wrap(pageHandler.Checkout))
	http.HandleFunc("/sitemap.xml", pageHandler.Sitemap)

	// Catch-all: redirect / to the default locale, serve everything else
	// from public/ (images, CSS, JS, locales, manifest).
	// The more specific routes above take precedence over this handler.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/mk", http.StatusFound)
			return
		}
		http.FileServer(http.Dir("public")).ServeHTTP(w, r)
	})

	// PORT is overridable for deployment (DO/Hetzner) and local testing;
	// defaults to 8080 for dev parity with air.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	logInstance.Info("Server listening on " + addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		logInstance.Error("Server failed", err)
		log.Fatalf("Server failed: %v", err)
	}
}
