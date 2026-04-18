package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"websocket/internal/config"
	"websocket/internal/marketdata"
	"websocket/internal/websocket"
)

func main() {
	cfg := config.Load()
	logger := newLogger(cfg.LogLevel)

	hub := websocket.NewHub(logger)
	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()
	go hub.Run(hubCtx)

	producer := marketdata.NewProducer(logger, marketdata.Config{
		Symbols:  cfg.Symbols,
		Interval: cfg.ProducerInterval,
	})
	go producer.Run(hubCtx, hub.PublishTicker)

	handler := websocket.NewHandler(hub, logger, websocket.ClientOptions{
		SendBuffer:      cfg.SendBuffer,
		ReadLimitBytes:  cfg.ReadLimitBytes,
		WriteTimeout:    cfg.WriteTimeout,
		PongWait:        cfg.PongWait,
		PingInterval:    cfg.PingInterval,
		RateLimitPerSec: cfg.RateLimitPerSec,
	})

	mux := http.NewServeMux()
	mux.Handle("/ws", handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info("server starting",
		slog.String("addr", cfg.Addr),
		slog.Any("symbols", cfg.Symbols),
		slog.Duration("producer_interval", cfg.ProducerInterval),
	)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server crashed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutdown signal received")
	hubCancel()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", slog.String("error", err.Error()))
	}
}

func newLogger(level string) *slog.Logger {
	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel})
	return slog.New(handler)
}
