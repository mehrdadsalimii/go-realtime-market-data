package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 54 * time.Second
	maxMessageSize = 1024
)

type subscription struct {
	Channel string `json:"channel"`
	Symbol  string `json:"symbol"`
}

type wsRequest struct {
	Op      string `json:"op"`
	Channel string `json:"channel"`
	Symbol  string `json:"symbol"`
}

type wsResponse struct {
	Op      string `json:"op,omitempty"`
	Result  string `json:"result,omitempty"`
	Error   string `json:"error,omitempty"`
	Channel string `json:"channel,omitempty"`
	Symbol  string `json:"symbol,omitempty"`
}

type Client struct {
	hub          *Hub
	conn         *websocket.Conn
	send         chan []byte
	subscription *subscription
}

func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 16),
	}
}

func (c *Client) Read() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("unexpected websocket close: %v", err)
			}
			return
		}

		var req wsRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			c.sendError("invalid json")
			continue
		}

		switch req.Op {
		case "subscribe":
			c.handleSubscribe(req)
		case "unsubscribe":
			c.subscription = nil
			c.sendJSON(wsResponse{Op: "unsubscribe", Result: "ok"})
		default:
			c.sendError("unsupported op")
		}
	}
}

func (c *Client) Write() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleSubscribe(req wsRequest) {
	if req.Channel != "ticker" {
		c.sendError("only ticker channel is supported in this basic example")
		return
	}
	if req.Symbol == "" {
		c.sendError("symbol is required")
		return
	}

	c.subscription = &subscription{
		Channel: req.Channel,
		Symbol:  req.Symbol,
	}

	c.sendJSON(wsResponse{
		Op:      "subscribe",
		Result:  "ok",
		Channel: req.Channel,
		Symbol:  req.Symbol,
	})
}

func (c *Client) sendError(msg string) {
	c.sendJSON(wsResponse{Error: msg})
}

func (c *Client) sendJSON(v any) {
	payload, err := json.Marshal(v)
	if err != nil {
		return
	}
	c.send <- payload
}
