package handlers

import (
	"bosfoot/internal/notify"
	"bosfoot/logger"
	"bosfoot/models"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lib/pq"
)

// OrderHandler handles order placement (guest checkout). Registered on
// POST /api/orders. The cart lives in the browser's localStorage, so the
// request carries only product_id + variant + qty — never the price. The
// server re-prices every line from the DB so a tampered client can't change
// what an order costs.
type OrderHandler struct {
	DB       *sql.DB
	Logger   *logger.Logger
	Notifier *notify.Mailer // emails the owner on each order; nil/disabled is fine
}

type orderItemReq struct {
	ProductID int    `json:"product_id"`
	Size      string `json:"size"`
	Color     string `json:"color"`
	Qty       int    `json:"qty"`
}

type orderReq struct {
	Email         string         `json:"email"`
	Phone         string         `json:"phone"`
	FirstName     string         `json:"first_name"`
	LastName      string         `json:"last_name"`
	Address       string         `json:"address"`
	City          string         `json:"city"`
	PostalCode    string         `json:"postal_code"`
	Notes         string         `json:"notes"`
	PaymentMethod string         `json:"payment_method"`
	Locale        string         `json:"locale"` // mk | sq | en
	Items         []orderItemReq `json:"items"`
}

type orderResp struct {
	ID            int    `json:"id"`
	TotalMKD      int    `json:"total_mkd"`
	PaymentMethod string `json:"payment_method"`
}

// CreateOrder handles POST /api/orders.
func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req orderReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Normalise + validate the contact/shipping fields.
	req.Email = strings.TrimSpace(req.Email)
	req.Phone = strings.TrimSpace(req.Phone)
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Address = strings.TrimSpace(req.Address)
	req.City = strings.TrimSpace(req.City)
	req.PostalCode = strings.TrimSpace(req.PostalCode)
	req.Notes = strings.TrimSpace(req.Notes)
	req.Locale = strings.ToLower(strings.TrimSpace(req.Locale))
	if req.Locale != "mk" && req.Locale != "sq" && req.Locale != "en" {
		req.Locale = "mk" // default
	}

	// Map request to model for validation
	order := models.Order{
		Email:         req.Email,
		Phone:         &req.Phone,
		FirstName:     req.FirstName,
		LastName:      req.LastName,
		Address:       req.Address,
		City:          req.City,
		PostalCode:    &req.PostalCode,
		Notes:         &req.Notes,
		PaymentMethod: req.PaymentMethod,
		Locale:        req.Locale,
	}
	for _, it := range req.Items {
		order.Items = append(order.Items, models.OrderItem{
			ProductID: it.ProductID,
			Quantity:  it.Qty,
		})
	}

	if err := order.Validate(); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Re-price every line from the DB. The client-supplied price is ignored —
	// only product_id, variant and qty are trusted. Inactive/unpublished
	// products are rejected.
	ids := make([]int, 0, len(req.Items))
	for _, it := range req.Items {
		if it.Qty <= 0 {
			writeJSONError(w, http.StatusBadRequest, "invalid quantity")
			return
		}
		ids = append(ids, it.ProductID)
	}

	priceRows, err := h.DB.QueryContext(r.Context(), `
		SELECT id, name, price_mkd
		FROM products
		WHERE id = ANY($1) AND is_active = TRUE AND is_published = TRUE
	`, pq.Array(ids))
	if err != nil {
		h.Logger.Error("CreateOrder: price query failed", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	type prodInfo struct {
		name  string
		price int
	}
	prodByID := make(map[int]prodInfo)
	for priceRows.Next() {
		var id, price int
		var name string
		if err := priceRows.Scan(&id, &name, &price); err != nil {
			priceRows.Close()
			h.Logger.Error("CreateOrder: price scan failed", err)
			writeJSONError(w, http.StatusInternalServerError, "internal error")
			return
		}
		prodByID[id] = prodInfo{name: name, price: price}
	}
	priceRows.Close()

	total := 0
	for _, it := range req.Items {
		p, ok := prodByID[it.ProductID]
		if !ok {
			writeJSONError(w, http.StatusBadRequest, "product unavailable")
			return
		}
		total += p.price * it.Qty
	}

	// Persist the order + items in one transaction.
	tx, err := h.DB.BeginTx(r.Context(), nil)
	if err != nil {
		h.Logger.Error("CreateOrder: begin tx failed", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}
	defer tx.Rollback() // no-op after a successful Commit

	var orderID int
	err = tx.QueryRowContext(r.Context(), `
		INSERT INTO orders
			(email, phone, first_name, last_name, address, city, postal_code,
			 country, notes, payment_method, total_mkd, locale)
		VALUES ($1,$2,$3,$4,$5,$6,$7,'MK',$8,$9,$10,$11)
		RETURNING id
	`,
		req.Email, req.Phone, req.FirstName, req.LastName, req.Address, req.City,
		nullStr(req.PostalCode), nullStr(req.Notes), req.PaymentMethod, total, req.Locale,
	).Scan(&orderID)
	if err != nil {
		h.Logger.Error("CreateOrder: insert order failed", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Reservation mode (pre-launch): we take reservations, not real sales, so we
	// do NOT validate against or decrement product_stock — every size is
	// reservable regardless of inventory. Product existence and price are already
	// validated in the pricing loop above. To switch back to real selling, restore
	// the SELECT ... FOR UPDATE variant check, the insufficient-stock guard, and
	// the `UPDATE product_stock SET qty = qty - $1` decrement here.
	for _, it := range req.Items {
		if _, err := tx.ExecContext(r.Context(), `
			INSERT INTO order_items
				(order_id, product_id, size, color, quantity, unit_price_mkd)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, orderID, it.ProductID, nullStr(it.Size), nullStr(it.Color), it.Qty, prodByID[it.ProductID].price); err != nil {
			h.Logger.Error("CreateOrder: insert item failed", err)
			writeJSONError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		h.Logger.Error("CreateOrder: commit failed", err)
		writeJSONError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Durable record of the sale, independent of email — visible in stdout/journalctl
	// even if SMTP is down or unconfigured.
	h.Logger.Info("Order placed",
		"order_id", orderID, "total_mkd", total,
		"email", req.Email, "payment", req.PaymentMethod, "items", len(req.Items))

	// Best-effort owner notification (async; never blocks or fails the order).
	if h.Notifier != nil {
		items := make([]notify.Item, 0, len(req.Items))
		for _, it := range req.Items {
			items = append(items, notify.Item{
				Name:  prodByID[it.ProductID].name,
				Size:  it.Size,
				Color: it.Color,
				Qty:   it.Qty,
				Price: prodByID[it.ProductID].price,
			})
		}
		notifyOrder := notify.Order{
			ID:            orderID,
			Total:         total,
			PaymentMethod: req.PaymentMethod,
			Name:          strings.TrimSpace(req.FirstName + " " + req.LastName),
			Email:         req.Email,
			Phone:         req.Phone,
			Address:       req.Address,
			City:          req.City,
			PostalCode:    req.PostalCode,
			Notes:         req.Notes,
			Locale:        req.Locale,
			Items:         items,
		}
		h.Notifier.OrderPlaced(notifyOrder)
		h.Notifier.CustomerConfirmation(notifyOrder)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(orderResp{
		ID:            orderID,
		TotalMKD:      total,
		PaymentMethod: req.PaymentMethod,
	})
}

// nullStr maps "" → SQL NULL so optional columns (phone, postal_code, notes,
// size, color) stay NULL rather than empty strings.
func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
