package main

import (
	"encoding/json"
	"fmt"
	"html"
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
	Id      int    `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Date    string `json:"date"`
	Topic   string `json:"topic"`
	Content string `json:"content"`
}

// TODO replace these with Redis
var nMsgs = 0
var msgStore []Message

var upgrader = websocket.Upgrader{
	// just return true for now for all origins
	CheckOrigin: func(r *http.Request) bool { return true },
}

func reader(conn *websocket.Conn) {
	for {
		// p is a []byte and msgType is an int
		// with value websocket.BinaryMessage or websocket.TextMessage
		msgType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		log.Printf("Received a request for messages from %s", conn.RemoteAddr())
		req := msgRequest{}
		json.Unmarshal(p, &req)
		// frontend can request all messages with LastId == -1
		if req.LastId < 0 {
			req.LastId = nMsgs
		}
		// only respond if we have messages
		if req.LastId > 0 {
			reply, err := json.Marshal(msgStore[req.FirstId:req.LastId])
			if err != nil {
				log.Println(err)
				return
			}

			log.Println(string(reply))

			// TODO use Redis to broadcast this instead of writing directly back
			if err := conn.WriteMessage(msgType, reply); err != nil {
				log.Println(err)
				return
			}
		}
	}
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		// frontend POSTS with Content-Type: multipart/form-data
		const maxMem = 500 << 10 // 500 KB
		if err := r.ParseMultipartForm(maxMem); err != nil {
			log.Println(err)
			return
		}
		log.Println("New message received")
		msg := Message{
			Name:    html.EscapeString(r.FormValue("name")),
			Email:   html.EscapeString(r.FormValue("email")),
			Date:    html.EscapeString(time.Now().Format(time.ANSIC)),
			Topic:   html.EscapeString(r.FormValue("topic")),
			Content: html.EscapeString(r.FormValue("content")),
		}
		// TODO store messages in Redis
		// TODO get message IDs from redis
		nMsgs++
		msg.Id = nMsgs
		msgStore = append(msgStore, msg)
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
