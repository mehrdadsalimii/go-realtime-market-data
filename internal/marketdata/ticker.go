package marketdata

type TickerEvent struct {
	Symbol           string `json:"symbol"`
	LastPrice        string `json:"last_price"`
	Open24h          string `json:"open_24h"`
	High24h          string `json:"high_24h"`
	Low24h           string `json:"low_24h"`
	Volume24h        string `json:"volume_24h"`
	Change24h        string `json:"change_24h"`
	ChangePercent24h string `json:"change_percent_24h"`
	BestBid          string `json:"best_bid"`
	BestAsk          string `json:"best_ask"`
	EventTime        int64  `json:"event_time"`
}
