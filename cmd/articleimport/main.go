// Command articleimport loads the foot-health articles into the database.
//
//	go run ./cmd/articleimport
//
// It runs db/articles.sql (idempotent ADD COLUMN migration) then upserts the
// articles + per-locale translations from db/articles.json using parameterised
// queries — the MK/SQ/EN bodies are too large to hand-escape as SQL. Re-running
// is safe: ON CONFLICT ... DO UPDATE refreshes existing rows in place.
//
// Run from the project root so the relative db/ paths resolve.
package main

import (
	"encoding/json"
	"log"
	"os"
	"time"

	"bosfoot/internal/database"
	"bosfoot/models"
)

type seed struct {
	ID          string                           `json:"id"`
	Featured    bool                             `json:"featured"`
	Author      string                           `json:"author"`
	PublishedAt string                           `json:"publishedAt"`
	Slug        map[string]string                `json:"slug"`
	Title       map[string]string                `json:"title"`
	Lead        map[string]string                `json:"lead"`
	Body        map[string][]models.ArticleBlock `json:"body"`
}

var langs = []string{"mk", "sq", "en"}

func main() {
	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 1. Additive schema migration (idempotent).
	migration, err := os.ReadFile("db/articles.sql")
	if err != nil {
		log.Fatalf("read db/articles.sql: %v", err)
	}
	if _, err := db.Exec(string(migration)); err != nil {
		log.Fatalf("exec db/articles.sql: %v", err)
	}
	log.Println("✓ db/articles.sql")

	// 2. Load content.
	raw, err := os.ReadFile("db/articles.json")
	if err != nil {
		log.Fatalf("read db/articles.json: %v", err)
	}
	var seeds []seed
	if err := json.Unmarshal(raw, &seeds); err != nil {
		log.Fatalf("parse db/articles.json: %v", err)
	}

	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	for _, s := range seeds {
		publishedAt, err := time.Parse(time.RFC3339, s.PublishedAt)
		if err != nil {
			log.Fatalf("article %s: bad publishedAt %q: %v", s.ID, s.PublishedAt, err)
		}

		var articleID int
		err = tx.QueryRow(`
			INSERT INTO articles (slug, is_published, is_featured, author, published_at)
			VALUES ($1, TRUE, $2, $3, $4)
			ON CONFLICT (slug) DO UPDATE SET
				is_published = EXCLUDED.is_published,
				is_featured  = EXCLUDED.is_featured,
				author       = EXCLUDED.author,
				published_at = EXCLUDED.published_at,
				updated_at   = now()
			RETURNING id
		`, s.Slug["en"], s.Featured, s.Author, publishedAt).Scan(&articleID)
		if err != nil {
			log.Fatalf("article %s: upsert failed: %v", s.ID, err)
		}

		for _, lang := range langs {
			bodyJSON, err := json.Marshal(s.Body[lang])
			if err != nil {
				log.Fatalf("article %s/%s: marshal body: %v", s.ID, lang, err)
			}
			_, err = tx.Exec(`
				INSERT INTO article_translations (article_id, lang, title, lead, body, slug)
				VALUES ($1, $2::lang_code, $3, $4, $5, $6)
				ON CONFLICT (article_id, lang) DO UPDATE SET
					title = EXCLUDED.title,
					lead  = EXCLUDED.lead,
					body  = EXCLUDED.body,
					slug  = EXCLUDED.slug
			`, articleID, lang, s.Title[lang], s.Lead[lang], string(bodyJSON), s.Slug[lang])
			if err != nil {
				log.Fatalf("article %s/%s: translation upsert failed: %v", s.ID, lang, err)
			}
		}
		log.Printf("  ✓ %s (id %d)", s.ID, articleID)
	}

	if err := tx.Commit(); err != nil {
		log.Fatalf("commit: %v", err)
	}
	log.Printf("Imported %d articles.", len(seeds))
}
