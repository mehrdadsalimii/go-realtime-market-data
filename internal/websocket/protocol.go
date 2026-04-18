package websocket

type Subscription struct {
	Channel string `json:"channel"`
	Symbol  string `json:"symbol"`
}

type ClientMessage struct {
	Op   string         `json:"op"`
	Args []Subscription `json:"args"`
}

type AckMessage struct {
	Type   string         `json:"type"`
	Op     string         `json:"op"`
	Result string         `json:"result"`
	Args   []Subscription `json:"args,omitempty"`
}

type ErrorMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type EventMessage struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Symbol  string `json:"symbol"`
	Data    any    `json:"data"`
	TS      int64  `json:"ts"`
}
