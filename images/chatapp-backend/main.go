package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type msgRequest struct {
	FirstId int `json:"first_id"`
	LastId  int `json:"last_id"`
}

type Message struct {
	Id      int
	Name    string
	Email   string
	Date    time.Time
	Topic   string
	Content string
}

var nMsgs = 0

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

		req := msgRequest{}
		json.Unmarshal(p, &req)

		log.Printf("first_id: %d last_id: %d", req.FirstId, req.LastId)
	}
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	const maxMem = 500 << 10 // 500 KB
	switch r.Method {
	case "POST":
		// frontend POSTS form with Content-Type: multipart/form-data
		if err := r.ParseMultipartForm(maxMem); err != nil {
			log.Println(err)
			return
		}
		log.Println("New message received")
		// TODO get message IDs from redis
		nMsgs++
		msg := Message{
			Id:      nMsgs,
			Name:    r.FormValue("name"),
			Email:   r.FormValue("email"),
			Date:    time.Now(),
			Topic:   r.FormValue("topic"),
			Content: r.FormValue("content"),
		}
		log.Printf("%+v", msg)
	default:
		log.Println("Only POST requests supported")
		return
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	// upgrade connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	fmt.Println("Client connected")

	// listen indefinitely for new messages
	reader(conn)
}

func setupRoutes() {
	http.HandleFunc("/chatapp/send", postHandler)
	http.HandleFunc("/chatapp/websocket", wsHandler)
}

func main() {
	fmt.Println("Hello world")
	setupRoutes()
	log.Fatal(http.ListenAndServe(":14222", nil))
}
