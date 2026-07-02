package pubsub

import (
	"sync"
)

// StockEvent represents the payload broadcasted when inventory changes.
type StockEvent struct {
	ProductID int     `json:"product_id"`
	EUSize    float64 `json:"eu_size"`
	Color     string  `json:"color"`
	Qty       int     `json:"qty"`
}

// StockHub maintains the set of active client subscription channels
// and broadcasts stock updates thread-safely.
type StockHub struct {
	mu          sync.RWMutex
	subscribers map[chan StockEvent]bool
}

// Hub is the global instance of the stock event hub.
var Hub = NewStockHub()

// NewStockHub creates a new StockHub.
func NewStockHub() *StockHub {
	return &StockHub{
		subscribers: make(map[chan StockEvent]bool),
	}
}

// Subscribe registers a new subscriber channel and returns it.
func (h *StockHub) Subscribe() chan StockEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan StockEvent, 10) // buffered to prevent slow clients from blocking
	h.subscribers[ch] = true
	return ch
}

// Unsubscribe closes the subscriber channel and removes it.
func (h *StockHub) Unsubscribe(ch chan StockEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.subscribers[ch]; exists {
		delete(h.subscribers, ch)
		close(ch)
	}
}

// Broadcast broadcasts a StockEvent to all subscribed channels.
func (h *StockHub) Broadcast(event StockEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers {
		select {
		case ch <- event:
		default:
			// Buffer full: slow client is skipped to prevent blocking the checkout flow
		}
	}
}
