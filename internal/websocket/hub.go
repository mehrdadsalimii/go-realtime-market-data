package websocket

import (
	"encoding/json"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

type Hub struct {
	clients    map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

type TickerUpdate struct {
	Channel string `json:"channel"`
	Symbol  string `json:"symbol"`
	Price   string `json:"price"`
	TS      int64  `json:"ts"`
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	publishTicker := time.NewTicker(2 * time.Second)
	defer publishTicker.Stop()

	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.mu.Unlock()
		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()
		case <-publishTicker.C:
			h.broadcastTickerSample()
		}
	}
}

func (h *Hub) broadcastTickerSample() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for c := range h.clients {
		if c.subscription == nil {
			continue
		}
		if c.subscription.Channel != "ticker" {
			continue
		}

		payload, err := json.Marshal(TickerUpdate{
			Channel: "ticker",
			Symbol:  c.subscription.Symbol,
			Price:   randomPrice(),
			TS:      time.Now().UnixMilli(),
		})
		if err != nil {
			continue
		}

		select {
		case c.send <- payload:
		default:
			close(c.send)
			delete(h.clients, c)
		}
	}
}

func randomPrice() string {
	base := 50000.0
	jitter := rand.Float64() * 1000
	return strconv.FormatFloat(base+jitter, 'f', 2, 64)
}
