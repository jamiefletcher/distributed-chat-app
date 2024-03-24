package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// just return true for now for all origins
	CheckOrigin: func(r *http.Request) bool { return true },
}

func reader(conn *websocket.Conn) {
	for {
		// p is a []byte and messageType is an int
		// with value websocket.BinaryMessage or websocket.TextMessage
		// messageType, p, err := conn.ReadMessage()
		_, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		// cast p to string and print
		log.Println(string(p))
	}
}

func basicHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Basic endpoint")
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	// upgrade connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	log.Println("Client connected")

	// listen indefinitely for new messages
	reader(conn)
}

func setupRoutes() {
	http.HandleFunc("/", basicHandler)
	http.HandleFunc("/ws", wsHandler)
}

func main() {
	fmt.Println("Hello world")
	setupRoutes()
	log.Fatal(http.ListenAndServe(":8080", nil))
}
