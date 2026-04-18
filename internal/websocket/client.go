package websocket

import (
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type ClientOptions struct {
	SendBuffer      int
	ReadLimitBytes  int64
	WriteTimeout    time.Duration
	PongWait        time.Duration
	PingInterval    time.Duration
	RateLimitPerSec int
}

type Client struct {
	id          string
	hub         *Hub
	conn        *websocket.Conn
	logger      *slog.Logger
	send        chan []byte
	rateLimiter *RateLimiter
	opts        ClientOptions
	closeOnce   sync.Once
}

func NewClient(id string, hub *Hub, conn *websocket.Conn, logger *slog.Logger, opts ClientOptions) *Client {
	if opts.SendBuffer <= 0 {
		opts.SendBuffer = 64
	}
	if opts.ReadLimitBytes <= 0 {
		opts.ReadLimitBytes = 4096
	}
	if opts.WriteTimeout <= 0 {
		opts.WriteTimeout = 10 * time.Second
	}
	if opts.PongWait <= 0 {
		opts.PongWait = 60 * time.Second
	}
	if opts.PingInterval <= 0 {
		opts.PingInterval = (opts.PongWait * 9) / 10
	}

	return &Client{
		id:          id,
		hub:         hub,
		conn:        conn,
		logger:      logger.With(slog.String("client_id", id)),
		send:        make(chan []byte, opts.SendBuffer),
		rateLimiter: NewRateLimiter(opts.RateLimitPerSec, time.Second),
		opts:        opts,
	}
}

func (c *Client) ReadLoop() {
	defer c.Close()

	c.conn.SetReadLimit(c.opts.ReadLimitBytes)
	_ = c.conn.SetReadDeadline(time.Now().Add(c.opts.PongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.opts.PongWait))
		return nil
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Warn("unexpected websocket close", slog.String("error", err.Error()))
			}
			return
		}

		if !c.rateLimiter.Allow(time.Now()) {
			c.sendError("rate_limit", "too many requests")
			c.logger.Warn("rate limit exceeded")
			return
		}

		var msg ClientMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			c.sendError("invalid_json", "request body is not valid JSON")
			continue
		}

		switch strings.ToLower(strings.TrimSpace(msg.Op)) {
		case "subscribe":
			c.handleSubscribe(msg.Args)
		case "unsubscribe":
			c.handleUnsubscribe(msg.Args)
		default:
			c.sendError("unsupported_op", "supported operations are subscribe and unsubscribe")
		}
	}
}

func (c *Client) WriteLoop() {
	pingTicker := time.NewTicker(c.opts.PingInterval)
	defer func() {
		pingTicker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case payload, ok := <-c.send:
			if !ok {
				_ = c.conn.SetWriteDeadline(time.Now().Add(c.opts.WriteTimeout))
				_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}

			_ = c.conn.SetWriteDeadline(time.Now().Add(c.opts.WriteTimeout))
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				c.logger.Warn("write failed", slog.String("error", err.Error()))
				c.Close()
				return
			}
		case <-pingTicker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(c.opts.WriteTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logger.Warn("ping failed", slog.String("error", err.Error()))
				c.Close()
				return
			}
		}
	}
}

func (c *Client) Enqueue(payload []byte) bool {
	select {
	case c.send <- payload:
		return true
	default:
		return false
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	})
}

func (c *Client) handleSubscribe(args []Subscription) {
	if len(args) == 0 {
		c.sendError("invalid_args", "subscribe requires at least one argument")
		return
	}

	normalized := make([]Subscription, 0, len(args))
	for _, sub := range args {
		ns, err := normalizeAndValidateSubscription(sub)
		if err != nil {
			c.sendError("invalid_subscription", err.Error())
			return
		}
		normalized = append(normalized, ns)
	}

	if err := c.hub.Subscribe(c, normalized); err != nil {
		c.sendError("subscribe_failed", err.Error())
		return
	}
	c.sendAck("subscribe", normalized)
}

func (c *Client) handleUnsubscribe(args []Subscription) {
	if len(args) == 0 {
		c.sendError("invalid_args", "unsubscribe requires at least one argument")
		return
	}

	normalized := make([]Subscription, 0, len(args))
	for _, sub := range args {
		ns, err := normalizeAndValidateSubscription(sub)
		if err != nil {
			c.sendError("invalid_subscription", err.Error())
			return
		}
		normalized = append(normalized, ns)
	}

	if err := c.hub.Unsubscribe(c, normalized); err != nil {
		c.sendError("unsubscribe_failed", err.Error())
		return
	}
	c.sendAck("unsubscribe", normalized)
}

func (c *Client) sendAck(op string, args []Subscription) {
	c.sendJSON(AckMessage{Type: "ack", Op: op, Result: "ok", Args: args})
}

func (c *Client) sendError(code, msg string) {
	c.sendJSON(ErrorMessage{Type: "error", Code: code, Message: msg})
}

func (c *Client) sendJSON(v any) {
	payload, err := json.Marshal(v)
	if err != nil {
		return
	}
	if !c.Enqueue(payload) {
		c.logger.Warn("dropping message because client queue is full")
	}
}
