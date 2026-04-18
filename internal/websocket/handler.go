package websocket

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/gorilla/websocket"
)

type Handler struct {
	hub      *Hub
	logger   *slog.Logger
	opts     ClientOptions
	clientID atomic.Uint64
	upgrader websocket.Upgrader
}

func NewHandler(hub *Hub, logger *slog.Logger, opts ClientOptions) *Handler {
	return &Handler{
		hub:    hub,
		logger: logger,
		opts:   opts,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Warn("websocket upgrade failed", slog.String("error", err.Error()))
		return
	}

	id := fmt.Sprintf("client-%d", h.clientID.Add(1))
	client := NewClient(id, h.hub, conn, h.logger, h.opts)
	h.hub.Register(client)

	go client.WriteLoop()
	go client.ReadLoop()
}
