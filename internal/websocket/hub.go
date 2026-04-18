package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	"websocket/internal/marketdata"
)

const ChannelTicker = "ticker"

type Hub struct {
	logger *slog.Logger

	clients             map[*Client]struct{}
	clientSubscriptions map[*Client]map[Subscription]struct{}
	subscribers         map[Subscription]map[*Client]struct{}

	registerCh    chan registerReq
	unregisterCh  chan unregisterReq
	subscribeCh   chan subscribeReq
	unsubscribeCh chan unsubscribeReq
	marketCh      chan marketdata.TickerEvent
}

type registerReq struct {
	client *Client
	done   chan struct{}
}

type unregisterReq struct {
	client *Client
	done   chan struct{}
}

type subscribeReq struct {
	client *Client
	subs   []Subscription
	errCh  chan error
}

type unsubscribeReq struct {
	client *Client
	subs   []Subscription
	errCh  chan error
}

func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		logger:              logger,
		clients:             make(map[*Client]struct{}),
		clientSubscriptions: make(map[*Client]map[Subscription]struct{}),
		subscribers:         make(map[Subscription]map[*Client]struct{}),
		registerCh:          make(chan registerReq, 64),
		unregisterCh:        make(chan unregisterReq, 64),
		subscribeCh:         make(chan subscribeReq, 256),
		unsubscribeCh:       make(chan unsubscribeReq, 256),
		marketCh:            make(chan marketdata.TickerEvent, 1024),
	}
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.shutdownAllClients()
			return
		case req := <-h.registerCh:
			h.handleRegister(req.client)
			close(req.done)
		case req := <-h.unregisterCh:
			h.handleUnregister(req.client)
			close(req.done)
		case req := <-h.subscribeCh:
			req.errCh <- h.handleSubscribe(req.client, req.subs)
			close(req.errCh)
		case req := <-h.unsubscribeCh:
			req.errCh <- h.handleUnsubscribe(req.client, req.subs)
			close(req.errCh)
		case event := <-h.marketCh:
			h.broadcastTicker(event)
		}
	}
}

func (h *Hub) Register(client *Client) {
	done := make(chan struct{})
	h.registerCh <- registerReq{client: client, done: done}
	<-done
}

func (h *Hub) Unregister(client *Client) {
	done := make(chan struct{})
	h.unregisterCh <- unregisterReq{client: client, done: done}
	<-done
}

func (h *Hub) Subscribe(client *Client, subs []Subscription) error {
	errCh := make(chan error, 1)
	h.subscribeCh <- subscribeReq{client: client, subs: subs, errCh: errCh}
	return <-errCh
}

func (h *Hub) Unsubscribe(client *Client, subs []Subscription) error {
	errCh := make(chan error, 1)
	h.unsubscribeCh <- unsubscribeReq{client: client, subs: subs, errCh: errCh}
	return <-errCh
}

func (h *Hub) PublishTicker(event marketdata.TickerEvent) {
	select {
	case h.marketCh <- event:
	default:
		h.logger.Warn("market event dropped", slog.String("symbol", event.Symbol))
	}
}

func (h *Hub) handleRegister(client *Client) {
	h.clients[client] = struct{}{}
	h.clientSubscriptions[client] = make(map[Subscription]struct{})
	h.logger.Info("client connected", slog.String("client_id", client.id))
}

func (h *Hub) handleUnregister(client *Client) {
	if _, ok := h.clients[client]; !ok {
		return
	}
	h.removeClient(client)
	h.logger.Info("client disconnected", slog.String("client_id", client.id))
}

func (h *Hub) handleSubscribe(client *Client, subs []Subscription) error {
	if _, ok := h.clients[client]; !ok {
		return errors.New("client not registered")
	}

	for _, sub := range subs {
		normalized, err := normalizeAndValidateSubscription(sub)
		if err != nil {
			return err
		}

		h.clientSubscriptions[client][normalized] = struct{}{}
		if h.subscribers[normalized] == nil {
			h.subscribers[normalized] = make(map[*Client]struct{})
		}
		h.subscribers[normalized][client] = struct{}{}
	}
	return nil
}

func (h *Hub) handleUnsubscribe(client *Client, subs []Subscription) error {
	if _, ok := h.clients[client]; !ok {
		return errors.New("client not registered")
	}

	for _, sub := range subs {
		normalized, err := normalizeAndValidateSubscription(sub)
		if err != nil {
			return err
		}
		h.removeSubscription(client, normalized)
	}
	return nil
}

func (h *Hub) removeSubscription(client *Client, sub Subscription) {
	clientSubs := h.clientSubscriptions[client]
	delete(clientSubs, sub)

	listeners := h.subscribers[sub]
	delete(listeners, client)
	if len(listeners) == 0 {
		delete(h.subscribers, sub)
	}
}

func (h *Hub) removeClient(client *Client) {
	clientSubs := h.clientSubscriptions[client]
	for sub := range clientSubs {
		listeners := h.subscribers[sub]
		delete(listeners, client)
		if len(listeners) == 0 {
			delete(h.subscribers, sub)
		}
	}

	delete(h.clientSubscriptions, client)
	delete(h.clients, client)
	close(client.send)
}

func (h *Hub) shutdownAllClients() {
	for c := range h.clients {
		close(c.send)
		delete(h.clients, c)
	}
	h.clientSubscriptions = map[*Client]map[Subscription]struct{}{}
	h.subscribers = map[Subscription]map[*Client]struct{}{}
	h.logger.Info("hub stopped")
}

func (h *Hub) broadcastTicker(event marketdata.TickerEvent) {
	sub := Subscription{Channel: ChannelTicker, Symbol: strings.ToUpper(event.Symbol)}
	listeners := h.subscribers[sub]
	if len(listeners) == 0 {
		return
	}

	payload, err := json.Marshal(EventMessage{
		Type:    "event",
		Channel: ChannelTicker,
		Symbol:  sub.Symbol,
		Data:    event,
		TS:      event.EventTime,
	})
	if err != nil {
		h.logger.Error("failed to marshal ticker event", slog.String("error", err.Error()))
		return
	}

	for client := range listeners {
		if !client.Enqueue(payload) {
			h.logger.Warn("disconnecting slow client",
				slog.String("client_id", client.id),
				slog.String("symbol", sub.Symbol),
			)
			h.removeClient(client)
		}
	}
}
