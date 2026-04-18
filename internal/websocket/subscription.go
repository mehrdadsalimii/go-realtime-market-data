package websocket

import (
	"errors"
	"strings"
)

func normalizeAndValidateSubscription(sub Subscription) (Subscription, error) {
	sub.Channel = strings.ToLower(strings.TrimSpace(sub.Channel))
	sub.Symbol = strings.ToUpper(strings.TrimSpace(sub.Symbol))

	if sub.Channel == "" {
		return Subscription{}, errors.New("channel is required")
	}
	if sub.Symbol == "" {
		return Subscription{}, errors.New("symbol is required")
	}
	if sub.Channel != ChannelTicker {
		return Subscription{}, errors.New("unsupported channel")
	}

	return sub, nil
}
