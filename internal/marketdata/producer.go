package marketdata

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"strconv"
	"time"
)

type PublishFunc func(TickerEvent)

type Config struct {
	Symbols  []string
	Interval time.Duration
}

type Producer struct {
	logger *slog.Logger
	cfg    Config
	rng    *rand.Rand
	books  map[string]*symbolState
}

type symbolState struct {
	basePrice float64
	open24h   float64
	high24h   float64
	low24h    float64
	volume24h float64
}

func NewProducer(logger *slog.Logger, cfg Config) *Producer {
	books := make(map[string]*symbolState, len(cfg.Symbols))
	for i, symbol := range cfg.Symbols {
		seed := float64(100+i) * 250
		if symbol == "BTCUSDT" {
			seed = 65000
		}
		if symbol == "ETHUSDT" {
			seed = 3200
		}
		if symbol == "SOLUSDT" {
			seed = 145
		}
		books[symbol] = &symbolState{
			basePrice: seed,
			open24h:   seed,
			high24h:   seed,
			low24h:    seed,
			volume24h: 1000,
		}
	}

	if cfg.Interval <= 0 {
		cfg.Interval = 2 * time.Second
	}

	return &Producer{
		logger: logger,
		cfg:    cfg,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
		books:  books,
	}
}

func (p *Producer) Run(ctx context.Context, publish PublishFunc) {
	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("market producer stopped")
			return
		case now := <-ticker.C:
			for _, symbol := range p.cfg.Symbols {
				publish(p.nextEvent(symbol, now.UnixMilli()))
			}
		}
	}
}

func (p *Producer) nextEvent(symbol string, eventTime int64) TickerEvent {
	state := p.books[symbol]
	if state == nil {
		state = &symbolState{basePrice: 1000, open24h: 1000, high24h: 1000, low24h: 1000, volume24h: 1000}
		p.books[symbol] = state
	}

	wave := (p.rng.Float64() - 0.5) * 0.008
	next := state.basePrice * (1 + wave)
	if next <= 0 {
		next = state.basePrice
	}
	state.basePrice = next

	if next > state.high24h {
		state.high24h = next
	}
	if next < state.low24h {
		state.low24h = next
	}

	state.volume24h += p.rng.Float64() * 250
	change := next - state.open24h
	changePct := (change / state.open24h) * 100
	spread := next * 0.0002

	return TickerEvent{
		Symbol:           symbol,
		LastPrice:        price(next),
		Open24h:          price(state.open24h),
		High24h:          price(state.high24h),
		Low24h:           price(state.low24h),
		Volume24h:        qty(state.volume24h),
		Change24h:        signed(change),
		ChangePercent24h: fmt.Sprintf("%+.2f", changePct),
		BestBid:          price(next - spread),
		BestAsk:          price(next + spread),
		EventTime:        eventTime,
	}
}

func price(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func qty(v float64) string {
	return strconv.FormatFloat(v, 'f', 4, 64)
}

func signed(v float64) string {
	return fmt.Sprintf("%+.2f", v)
}
