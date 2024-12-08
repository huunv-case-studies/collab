package chat

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait = 10 * time.Second
	pongWait = 60 * time.Second
	pingInterval = (9 * pongWait) / 10
)

var (
	hub = Hub {}
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

func serveHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "../chat/home.html")
}

func serveWs(w http.ResponseWriter, r *http.Request) {
	ws, _ := upgrader.Upgrade(w, r, nil)
	client := Client{}
	client.Open(r.FormValue("name"), ws, &hub)
}

func Execute() {
	hub.Open()
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", serveWs)
	http.ListenAndServe(":8081", nil)
}

