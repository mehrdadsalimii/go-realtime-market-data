package websocket

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
	"time"

	"websocket/internal/marketdata"
)

func TestHubBroadcastsOnlyToSubscribedClients(t *testing.T) {
	hub := newTestHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	btcClient := &Client{id: "btc", send: make(chan []byte, 4)}
	ethClient := &Client{id: "eth", send: make(chan []byte, 4)}

	hub.Register(btcClient)
	hub.Register(ethClient)

	if err := hub.Subscribe(btcClient, []Subscription{{Channel: "ticker", Symbol: "BTCUSDT"}}); err != nil {
		t.Fatalf("subscribe btc: %v", err)
	}
	if err := hub.Subscribe(ethClient, []Subscription{{Channel: "ticker", Symbol: "ETHUSDT"}}); err != nil {
		t.Fatalf("subscribe eth: %v", err)
	}

	hub.PublishTicker(marketdata.TickerEvent{Symbol: "BTCUSDT", LastPrice: "65000", EventTime: time.Now().UnixMilli()})

	btcMsg := readMessage(t, btcClient.send)
	if btcMsg.Symbol != "BTCUSDT" {
		t.Fatalf("expected BTCUSDT, got %s", btcMsg.Symbol)
	}

	select {
	case <-ethClient.send:
		t.Fatalf("eth client should not receive btc event")
	case <-time.After(120 * time.Millisecond):
	}
}

func TestHubUnsubscribeStopsDelivery(t *testing.T) {
	hub := newTestHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := &Client{id: "c1", send: make(chan []byte, 4)}
	hub.Register(client)

	sub := Subscription{Channel: "ticker", Symbol: "SOLUSDT"}
	if err := hub.Subscribe(client, []Subscription{sub}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := hub.Unsubscribe(client, []Subscription{sub}); err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}

	hub.PublishTicker(marketdata.TickerEvent{Symbol: "SOLUSDT", LastPrice: "145", EventTime: time.Now().UnixMilli()})

	select {
	case <-client.send:
		t.Fatalf("client should not receive event after unsubscribe")
	case <-time.After(120 * time.Millisecond):
	}
}

func TestHubDisconnectsSlowClient(t *testing.T) {
	hub := newTestHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	client := &Client{id: "slow", send: make(chan []byte, 1)}
	hub.Register(client)

	sub := Subscription{Channel: "ticker", Symbol: "BTCUSDT"}
	if err := hub.Subscribe(client, []Subscription{sub}); err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	client.send <- []byte("queue is full")
	hub.PublishTicker(marketdata.TickerEvent{Symbol: "BTCUSDT", LastPrice: "65000", EventTime: time.Now().UnixMilli()})

	time.Sleep(80 * time.Millisecond)
	if err := hub.Subscribe(client, []Subscription{sub}); err == nil {
		t.Fatalf("expected error for disconnected client")
	}
}

func newTestHub() *Hub {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewHub(logger)
}

func readMessage(t *testing.T, ch <-chan []byte) EventMessage {
	t.Helper()
	select {
	case payload := <-ch:
		var msg EventMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatalf("unmarshal message: %v", err)
		}
		return msg
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for message")
		return EventMessage{}
	}
}
