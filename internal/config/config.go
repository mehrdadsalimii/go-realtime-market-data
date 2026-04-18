package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr             string
	LogLevel         string
	Symbols          []string
	ProducerInterval time.Duration
	SendBuffer       int
	ReadLimitBytes   int64
	WriteTimeout     time.Duration
	PongWait         time.Duration
	PingInterval     time.Duration
	RateLimitPerSec  int
	ShutdownTimeout  time.Duration
}

func Load() Config {
	pongWait := getDuration("WS_PONG_WAIT", 60*time.Second)
	pingInterval := getDuration("WS_PING_INTERVAL", (pongWait*9)/10)

	return Config{
		Addr:             getString("ADDR", ":5002"),
		LogLevel:         strings.ToLower(getString("LOG_LEVEL", "info")),
		Symbols:          getSymbols("SYMBOLS", []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}),
		ProducerInterval: getDuration("PRODUCER_INTERVAL", 2*time.Second),
		SendBuffer:       getInt("WS_SEND_BUFFER", 64),
		ReadLimitBytes:   int64(getInt("WS_READ_LIMIT_BYTES", 4096)),
		WriteTimeout:     getDuration("WS_WRITE_TIMEOUT", 10*time.Second),
		PongWait:         pongWait,
		PingInterval:     pingInterval,
		RateLimitPerSec:  getInt("CLIENT_RATE_LIMIT", 20),
		ShutdownTimeout:  getDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

func getString(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	if n <= 0 {
		return fallback
	}
	return n
}

func getDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func getSymbols(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	parts := strings.Split(raw, ",")
	symbols := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.ToUpper(strings.TrimSpace(p))
		if s != "" {
			symbols = append(symbols, s)
		}
	}
	if len(symbols) == 0 {
		return fallback
	}
	return symbols
}
