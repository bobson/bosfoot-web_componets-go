package handlers

import (
	"bosfoot/logger"
	"bosfoot/models"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
)

// ReviewHandler handles review submission from the token page. Registered on
// POST /api/reviews (CSRF-protected). Like orders, the request is not trusted:
// the product being reviewed is read from the review token row, not the client,
// and the token is consumed atomically so it can't be replayed.
type ReviewHandler struct {
	DB     *sql.DB
	Logger *logger.Logger
}

type reviewReq struct {
	Token      string `json:"token"`
	Rating     int    `json:"rating"`
	Fit        *int   `json:"fit"` // optional; nil = not answered
	AuthorName string `json:"author_name"`
	Body       string `json:"body"`
}

// CreateReview handles POST /api/reviews.
func (h *ReviewHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req reviewReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Token = strings.TrimSpace(req.Token)
	req.AuthorName = strings.TrimSpace(req.AuthorName)
	req.Body = strings.TrimSpace(req.Body)
	if req.Token == "" {
		writeJSONError(w, http.StatusBadRequest, "missing token")
		return
	}

	ctx := r.Context()

	// The token is proof-of-purchase and binds the review to a product/order.
	// Look it up first (must exist and be unused). We do the consume + insert in
	// one transaction so a double submit can't create two reviews.
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		h.Logger.Error("CreateReview: begin tx failed", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback() // no-op after Commit

	// SELECT ... FOR UPDATE locks the token row so a concurrent submit blocks
	// here and then sees used_at set (matching 0 rows on the consume below).
	var tok models.ReviewToken
	var usedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT token, order_id, product_id, email, lang_code::text, used_at
		FROM review_tokens WHERE token = $1 FOR UPDATE
	`, req.Token).Scan(&tok.Token, &tok.OrderID, &tok.ProductID, &tok.Email, &tok.Lang, &usedAt)
	if err == sql.ErrNoRows {
		writeJSONError(w, http.StatusNotFound, "invalid token")
		return
	}
	if err != nil {
		h.Logger.Error("CreateReview: token lookup failed", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if usedAt.Valid {
		writeJSONError(w, http.StatusConflict, "already reviewed")
		return
	}

	// Build + validate the review. product_id and lang come from the token, never
	// the client. Status starts 'pending' (moderated before it shows).
	review := models.Review{
		ProductID:  tok.ProductID,
		OrderID:    &tok.OrderID,
		Rating:     req.Rating,
		Fit:        req.Fit,
		AuthorName: req.AuthorName,
		Lang:       tok.Lang,
	}
	if req.Body != "" {
		review.Body = &req.Body
	}
	if err := review.Validate(); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Consume the token. The WHERE used_at IS NULL guard + the row lock make a
	// concurrent double submit safe: the loser matches 0 rows and is rejected.
	res, err := tx.ExecContext(ctx, `
		UPDATE review_tokens SET used_at = now() WHERE token = $1 AND used_at IS NULL
	`, req.Token)
	if err != nil {
		h.Logger.Error("CreateReview: consume token failed", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		writeJSONError(w, http.StatusConflict, "already reviewed")
		return
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO reviews (product_id, order_id, rating, fit, author_name, body, lang_code, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7::lang_code,'pending')
	`, review.ProductID, review.OrderID, review.Rating, review.Fit,
		review.AuthorName, nullStr(req.Body), review.Lang); err != nil {
		h.Logger.Error("CreateReview: insert failed", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := tx.Commit(); err != nil {
		h.Logger.Error("CreateReview: commit failed", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	h.Logger.Info("Review submitted",
		"product_id", review.ProductID, "order_id", tok.OrderID, "rating", review.Rating)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
