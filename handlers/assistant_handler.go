package handlers

import (
	"bosfoot/internal/faq"
	"bosfoot/internal/site"
	"bosfoot/logger"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// AssistantHandler powers POST /api/assistant — the free-text half of the FAQ
// assistant widget. It grounds a small Claude model on internal/faq so answers
// stay accurate and on-brand, and answers in the customer's language.
//
// This is a PUBLIC, unauthenticated, money-spending endpoint, so cost/abuse
// control is the load-bearing part, not the model call:
//   - gated on ANTHROPIC_API_KEY (site.AssistantEnabled) — no key, no endpoint;
//   - max_tokens capped low, so a single answer can't run away;
//   - per-IP + global-daily rate limits (in-memory: single droplet, real client
//     IP via RealIP/Cf-Connecting-Ip);
//   - input length capped;
//   - Origin check rejects trivial cross-site browser embedding.
// CSRF is deliberately skipped: it doesn't stop cost-abuse (curl has no cookie),
// the _csrf cookie isn't seeded globally, and the rate limit is the real guard
// (same rationale as the /api/cart-add beacon).
type AssistantHandler struct {
	Logger  *logger.Logger
	SiteURL string
}

// The Anthropic call: cheapest capable model, low token ceiling. One shared
// client with a timeout so the outbound call finishes well inside the server's
// 30s WriteTimeout.
const (
	assistantModel    = "claude-haiku-4-5-20251001"
	assistantMaxTok   = 400
	assistantMaxInput = 500 // chars of the user question we forward
)

var assistantHTTP = &http.Client{Timeout: 20 * time.Second}

// In-memory rate limiter. Single instance, so a package-level map guarded by a
// mutex is enough; it resets daily to stay bounded.
const (
	assistantPerIPWindow = 5 * time.Minute
	assistantPerIPMax    = 8
	assistantGlobalMax   = 600 // per calendar day
)

var (
	assistantMu     sync.Mutex
	assistantHits   = map[string][]time.Time{}
	assistantDay    string
	assistantGlobal int
)

// assistantAllow reports whether a request from ip may proceed, recording it if
// so. Enforces both the per-IP sliding window and the global daily cap.
func assistantAllow(ip string) bool {
	assistantMu.Lock()
	defer assistantMu.Unlock()

	now := time.Now()
	if day := now.Format("2006-01-02"); day != assistantDay {
		assistantDay = day
		assistantGlobal = 0
		assistantHits = map[string][]time.Time{}
	}
	if assistantGlobal >= assistantGlobalMax {
		return false
	}

	cutoff := now.Add(-assistantPerIPWindow)
	kept := make([]time.Time, 0, len(assistantHits[ip]))
	for _, t := range assistantHits[ip] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	if len(kept) >= assistantPerIPMax {
		assistantHits[ip] = kept
		return false
	}
	assistantHits[ip] = append(kept, now)
	assistantGlobal++
	return true
}

type assistantReq struct {
	Question string `json:"question"`
	Locale   string `json:"locale"`
}

type assistantResp struct {
	Answer string `json:"answer"`
}

// Answer handles POST /api/assistant.
func (h *AssistantHandler) Answer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !site.AssistantEnabled() {
		// The widget hides the text box when this is off, so a request here means
		// a stale client or a probe — refuse without touching the API.
		http.Error(w, "assistant disabled", http.StatusServiceUnavailable)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && !h.sameOrigin(origin) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req assistantReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		http.Error(w, "empty question", http.StatusBadRequest)
		return
	}
	if len(question) > assistantMaxInput {
		question = question[:assistantMaxInput]
	}
	locale := req.Locale
	if locale != "mk" && locale != "sq" && locale != "en" {
		locale = "mk"
	}

	ip := clientIP(r)
	if !assistantAllow(ip) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	answer, err := h.ask(ctx, locale, question)
	if err != nil {
		h.Logger.Error("assistant request failed", err, "ip", ip, "locale", locale)
		http.Error(w, "assistant error", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(assistantResp{Answer: answer})
}

// sameOrigin reports whether the browser Origin header belongs to this site.
// curl and server-to-server callers send no Origin and are handled by the rate
// limiter instead.
func (h *AssistantHandler) sameOrigin(origin string) bool {
	o, err := url.Parse(origin)
	if err != nil {
		return false
	}
	s, err := url.Parse(h.SiteURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(o.Host, s.Host)
}

// --- Anthropic Messages API (raw net/http; the backend has no SDK dependency) ---

type anthropicCacheControl struct {
	Type string `json:"type"`
}

type anthropicSystemBlock struct {
	Type         string                 `json:"type"`
	Text         string                 `json:"text"`
	CacheControl *anthropicCacheControl `json:"cache_control,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string                 `json:"model"`
	MaxTokens int                    `json:"max_tokens"`
	System    []anthropicSystemBlock `json:"system"`
	Messages  []anthropicMessage     `json:"messages"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

func (h *AssistantHandler) ask(ctx context.Context, locale, question string) (string, error) {
	payload, err := json.Marshal(anthropicRequest{
		Model:     assistantModel,
		MaxTokens: assistantMaxTok,
		// The grounding block is byte-identical every call, so mark it cacheable;
		// it's a free cost win when the prefix is large enough to cache.
		System: []anthropicSystemBlock{{
			Type:         "text",
			Text:         assistantSystemPrompt(locale),
			CacheControl: &anthropicCacheControl{Type: "ephemeral"},
		}},
		Messages: []anthropicMessage{{Role: "user", Content: question}},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.anthropic.com/v1/messages", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY")))
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := assistantHTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<10))
		return "", fmt.Errorf("anthropic status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	for _, block := range out.Content {
		if block.Type == "text" {
			if text := strings.TrimSpace(block.Text); text != "" {
				return text, nil
			}
		}
	}
	return "", errors.New("empty assistant response")
}

// assistantSystemPrompt constrains the model to the FAQ knowledge base, the
// customer's language, and Bosfoot's curator (not maker) voice.
func assistantSystemPrompt(locale string) string {
	lang := map[string]string{"mk": "Macedonian", "sq": "Albanian", "en": "English"}[locale]

	var b strings.Builder
	b.WriteString("You are the friendly assistant for Bosfoot, a curated barefoot-shoe online store based in Ohrid, North Macedonia. ")
	b.WriteString("Answer the customer's question ONLY using the facts in the knowledge base below. ")
	b.WriteString("Keep answers short (1-3 sentences), warm, and plain. ")
	b.WriteString("Always reply in " + lang + ", regardless of the language the question is written in. ")
	b.WriteString("Bosfoot is a reseller and curator — it does NOT design or manufacture shoes; never imply otherwise. ")
	b.WriteString("If the knowledge base does not cover the question — a specific order's status, whether a specific size is in stock, or anything you are unsure about — say you are not certain and suggest emailing info@bosfoot.com. ")
	b.WriteString("Never invent prices, delivery times, or policies that are not stated below.\n\n")
	b.WriteString("KNOWLEDGE BASE:\n\n")
	b.WriteString(faq.GroundingText(locale))
	return b.String()
}
