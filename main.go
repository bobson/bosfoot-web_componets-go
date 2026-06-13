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
	"bosfoot/internal/routes"
	"bosfoot/internal/tmpl"
	"bosfoot/logger"
)

func initializeLogger() *logger.Logger {
	logInstance, err := logger.NewLogger("bosfoot.log")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	return logInstance
}

func main() {
	logInstance := initializeLogger()
	defer logInstance.Close()

	db, err := database.Connect()
	if err != nil {
		logInstance.Error("Failed to connect to database", err)
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	logInstance.Info("Connected to database")

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

	productHandler := &handlers.ProductHandler{DB: db, Logger: logInstance}
	orderHandler := &handlers.OrderHandler{DB: db, Logger: logInstance}

	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "http://localhost:8080"
	}

	pageHandler := &handlers.PageHandler{
		DB:       db,
		Logger:   logInstance,
		Renderer: renderer,
		UI:       ui,
		SiteURL:  siteURL,
	}

	pc := cache.New(60 * time.Second)

	// Register all routes
	routes.Register(productHandler, orderHandler, pageHandler, pc)

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
