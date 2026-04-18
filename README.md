# Basic WebSocket Skeleton (Go)

A minimal WebSocket server for public GitHub usage.

## Run

```bash
go run ./cmd/wsserver
```

Server starts on `:5002` (or `ADDR` env var).

- WebSocket endpoint: `/ws`
- Health check: `/healthz`

## Subscribe Example

Send this message after connecting:

```json
{"op":"subscribe","channel":"ticker","symbol":"BTCUSDT"}
```

Only one channel is supported in this basic version:

- `ticker`

Sample response:

```json
{"op":"subscribe","result":"ok","channel":"ticker","symbol":"BTCUSDT"}
```

Then the server pushes mock ticker updates every 2 seconds.
