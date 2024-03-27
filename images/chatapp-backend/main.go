package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

type msgRequest struct {
	FirstId int64 `json:"first_id"`
	LastId  int64 `json:"last_id"`
}

type Message struct {
	Id      int64  `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Date    string `json:"date"`
	Topic   string `json:"topic"`
	Content string `json:"content"`
}

const REDIS_CHANNEL = "messages"
const REDIS_ID_KEY = "id"
const REDIS_MESSAGES_KEY = "messages"

// TODO replace these with Redis
var nMsgs int64 = 0
var msgStore []Message

var upgrader = websocket.Upgrader{
	// just return true for now for all origins
	CheckOrigin: func(r *http.Request) bool { return true },
}

func reader(conn *websocket.Conn) error {
	// p is a []byte and msgType is an int
	// with value websocket.BinaryMessage or websocket.TextMessage
	msgType, p, err := conn.ReadMessage()
	if err != nil {
		log.Println(err)
		return err
	}
	log.Printf("Request for messages received from %s", conn.RemoteAddr())
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
			return err
		}

		log.Println(string(reply))

		// TODO use Redis to broadcast this instead of writing directly back
		if err := conn.WriteMessage(msgType, reply); err != nil {
			log.Println(err)
			return err
		}
	}
	return nil
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
		log.Printf("New message received from %s", r.RemoteAddr)
		msg := Message{
			Name:    html.EscapeString(r.FormValue("name")),
			Email:   html.EscapeString(r.FormValue("email")),
			Date:    html.EscapeString(time.Now().Format(time.ANSIC)),
			Topic:   html.EscapeString(r.FormValue("topic")),
			Content: html.EscapeString(r.FormValue("content")),
		}
		// TODO store messages in Redis
		// TODO get message IDs from redis

		// Redis
		redisdb := redis.NewClient(&redis.Options{
			Addr:     "redis:6379",
			Password: "",
			DB:       0,
		})

		ctx := context.Background()
		// Get ID for next message from Redis
		// This isn't ideal for reasons noted below
		msgId, err := redisdb.Incr(ctx, REDIS_ID_KEY).Result()
		if err != nil {
			log.Println(err)
			return
		}
		msg.Id = msgId

		// Marshal message into json so we can store in Redis and publish
		msgJson, err := json.Marshal(msg)
		if err != nil {
			// This leaves us in a bad state where msg.Id is incremented but
			// the message was not added to Redis (because json.Marshal fails)
			log.Fatal(err)
			return
		}

		// Push new message into message list in Redis
		if err := redisdb.RPush(ctx, REDIS_MESSAGES_KEY, msgJson).Err(); err != nil {
			// This leaves us in a bad state where msg.Id is incremented but
			// the message itself was not added to Redis
			log.Fatal(err)
			return
		}

		nMsgs++
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

	// listen for new messages
	// if we get an error, just close down and let client restart
	for err == nil {
		err = reader(conn)
	}
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
