package handlers

import (
	"bosfoot/internal/notify"
	"bosfoot/internal/uploads"
	"bosfoot/logger"
	"bosfoot/models"
	"database/sql"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
)

// maxReviewUpload caps a whole multipart review submission (all photos + fields).
var maxReviewUpload = int64(uploads.MaxPhotos*uploads.MaxFileBytes + (1 << 20))

// ReviewHandler handles review submission from the token page. Registered on
// POST /api/reviews (CSRF-protected). Like orders, the request is not trusted:
// the product being reviewed is read from the review token row, not the client,
// and the token is consumed atomically so it can't be replayed.
type ReviewHandler struct {
	DB       *sql.DB
	Logger   *logger.Logger
	Notifier *notify.Mailer // emails the owner on each pending review; nil/disabled is fine
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

	// The form submits multipart (so it can carry photos); older/no-photo callers
	// may still send JSON. Parse whichever it is into the same reviewReq.
	var req reviewReq
	var photoFiles []*multipart.FileHeader
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxReviewUpload)
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid or too-large upload")
			return
		}
		req.Token = strings.TrimSpace(r.FormValue("token"))
		req.AuthorName = strings.TrimSpace(r.FormValue("author_name"))
		req.Body = strings.TrimSpace(r.FormValue("body"))
		req.Rating, _ = strconv.Atoi(strings.TrimSpace(r.FormValue("rating")))
		if v := strings.TrimSpace(r.FormValue("fit")); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				req.Fit = &n
			}
		}
		if r.MultipartForm != nil {
			photoFiles = r.MultipartForm.File["photos"]
		}
	} else {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		req.Token = strings.TrimSpace(req.Token)
		req.AuthorName = strings.TrimSpace(req.AuthorName)
		req.Body = strings.TrimSpace(req.Body)
	}
	if req.Token == "" {
		writeJSONError(w, http.StatusBadRequest, "missing token")
		return
	}

	ctx := r.Context()

	// Convert + store photos BEFORE the transaction so image work never holds the
	// token lock / a DB connection. They land in the unserved pending dir; a bad
	// or oversized photo is logged and skipped (the review text still saves). If
	// the transaction below fails for any reason, the deferred cleanup removes
	// these pending files so nothing is orphaned on disk.
	var savedPhotos []string
	cleanupPhotos := true
	defer func() {
		if cleanupPhotos {
			for _, n := range savedPhotos {
				_ = uploads.Remove(n)
			}
		}
	}()
	for i, fh := range photoFiles {
		if i >= uploads.MaxPhotos {
			break
		}
		f, err := fh.Open()
		if err != nil {
			continue
		}
		name, err := uploads.SaveReviewPhoto(f)
		f.Close()
		if err != nil {
			h.Logger.Info("review photo skipped", "err", err.Error())
			continue
		}
		savedPhotos = append(savedPhotos, name)
	}

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

	// Snapshot the buyer context from the order (best-effort — it's display sugar,
	// so a missing city/variant never blocks the review). city from the order; the
	// exact size/colour from the ordered line for this product (first if several).
	var buyerCity, itemSize, itemColor sql.NullString
	_ = tx.QueryRowContext(ctx, `SELECT city FROM orders WHERE id = $1`, tok.OrderID).Scan(&buyerCity)
	_ = tx.QueryRowContext(ctx, `
		SELECT size, color FROM order_items
		WHERE order_id = $1 AND product_id = $2 ORDER BY id LIMIT 1
	`, tok.OrderID, tok.ProductID).Scan(&itemSize, &itemColor)

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

	var reviewID int
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO reviews (product_id, order_id, rating, fit, author_name, body, lang_code, status,
		                     buyer_city, size, color)
		VALUES ($1,$2,$3,$4,$5,$6,$7::lang_code,'pending',$8,$9,$10) RETURNING id
	`, review.ProductID, review.OrderID, review.Rating, review.Fit,
		review.AuthorName, nullStr(req.Body), review.Lang,
		buyerCity, itemSize, itemColor).Scan(&reviewID); err != nil {
		h.Logger.Error("CreateReview: insert failed", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Attach the converted photos (still pending files on disk; -approve promotes
	// them to the served dir).
	for i, name := range savedPhotos {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO review_photos (review_id, filename, sort_order) VALUES ($1,$2,$3)
		`, reviewID, name, i); err != nil {
			h.Logger.Error("CreateReview: photo insert failed", err)
			writeJSONError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		h.Logger.Error("CreateReview: commit failed", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	cleanupPhotos = false // committed — keep the pending files for moderation

	h.Logger.Info("Review submitted",
		"product_id", review.ProductID, "order_id", tok.OrderID, "rating", review.Rating,
		"photos", len(savedPhotos))

	// Notify the owner that a review is waiting for moderation. Best-effort: the
	// product name lookup and the send never affect the customer's response.
	if h.Notifier != nil {
		var productName string
		_ = h.DB.QueryRowContext(ctx, `SELECT name FROM products WHERE id = $1`, review.ProductID).Scan(&productName)
		h.Notifier.PendingReview(notify.ReviewNotice{
			ProductID:   review.ProductID,
			ProductName: productName,
			OrderID:     tok.OrderID,
			Rating:      review.Rating,
			AuthorName:  review.AuthorName,
			Body:        req.Body,
			Locale:      review.Lang,
			PhotoCount:  len(savedPhotos),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"ok": true})
}
