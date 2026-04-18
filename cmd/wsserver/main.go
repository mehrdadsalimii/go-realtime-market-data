package main

import (
	"log"
	"net/http"
	"os"
	"websocket/internal/websocket"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":5002"
	}

	hub := websocket.NewHub()
	go hub.Run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		websocket.ServeWS(hub, w, r)
	})

	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("websocket server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
