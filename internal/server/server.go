package server

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/nityam123-pixle/testify-cli/internal/detector"
	"github.com/nityam123-pixle/testify-cli/internal/scanner"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Message struct {
	Type   string             `json:"type"`
	Routes []scanner.Route    `json:"routes,omitempty"`
	Stack  detector.StackInfo `json:"stack,omitempty"`
}

func Start(port string, routes []scanner.Route, info detector.StackInfo) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		msg := Message{
			Type:   "init",
			Routes: routes,
			Stack:  info,
		}

		b, err := json.Marshal(msg)
		if err == nil {
			_ = conn.WriteMessage(websocket.TextMessage, b)
		}

		// Block and read to keep the connection open
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	})

	_ = http.ListenAndServe(":"+port, mux)
}
