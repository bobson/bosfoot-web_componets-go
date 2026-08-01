package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bosfoot/handlers"
	"bosfoot/internal/cache"
	"bosfoot/internal/database"
	"bosfoot/internal/locale"
	"bosfoot/internal/middleware"
	"bosfoot/internal/notify"
	"bosfoot/internal/routes"
	"bosfoot/internal/tmpl"
	"bosfoot/logger"
)

func main() {
	logInstance, err := logger.NewLogger("bosfoot.log")
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
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

	notifier := notify.New(logInstance)

	productHandler := &handlers.ProductHandler{DB: db, Logger: logInstance}
	orderHandler := &handlers.OrderHandler{DB: db, Logger: logInstance, Notifier: notifier}
	trackHandler := &handlers.TrackHandler{Logger: logInstance}
	reviewHandler := &handlers.ReviewHandler{DB: db, Logger: logInstance}

	siteURL := os.Getenv("SITE_URL")
	if siteURL == "" {
		siteURL = "http://localhost:8080"
	}

	assistantHandler := &handlers.AssistantHandler{Logger: logInstance, SiteURL: siteURL}

	pageHandler := &handlers.PageHandler{
		DB:       db,
		Logger:   logInstance,
		Renderer: renderer,
		UI:       ui,
		SiteURL:  siteURL,
	}

	pc := cache.New(60 * time.Second)

	// Register all routes
	routes.Register(productHandler, orderHandler, pageHandler, trackHandler, reviewHandler, assistantHandler, pc)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr: ":" + port,
		// Outermost: RealIP (Cloudflare visitor IP) → RedirectTrailingSlash
		// (/en/ → /en for old indexed Astro URLs) → the default mux.
		Handler:           middleware.RealIP(middleware.RedirectTrailingSlash(http.DefaultServeMux)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Shut down gracefully on SIGINT/SIGTERM so in-flight requests — including
	// order POSTs — finish instead of being cut off on deploy.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logInstance.Info("Server listening on " + srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logInstance.Error("Server failed", err)
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-ctx.Done()
	stop() // restore default signal handling so a second signal force-quits
	logInstance.Info("Shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logInstance.Error("Graceful shutdown failed", err)
	}
	logInstance.Info("Server stopped")
}
