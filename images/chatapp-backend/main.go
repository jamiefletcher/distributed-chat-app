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

var upgrader = websocket.Upgrader{
	// just return true for now for all origins
	CheckOrigin: func(r *http.Request) bool { return true },
}

func loadStoredMsgs(conn *websocket.Conn, redisdb *redis.Client, ctx context.Context) {
	// p is a []byte and msgType is an int
	// with value websocket.BinaryMessage or websocket.TextMessage
	defer conn.Close()
	for {
		msgType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		log.Printf("Request for messages received from %s", conn.RemoteAddr())
		req := msgRequest{}
		json.Unmarshal(p, &req)

		encodedMsgs, err := redisdb.LRange(ctx, REDIS_MESSAGES_KEY, req.FirstId, req.LastId).Result()
		if err != nil {
			log.Println(err)
			return
		}

		messages := []Message{}
		for _, m := range encodedMsgs {
			msg := Message{}
			json.Unmarshal([]byte(m), &msg)
			messages = append(messages, msg)
		}

		if len(messages) > 0 {
			rpl, err := json.Marshal(messages)
			if err != nil {
				log.Println(err)
				return
			}
			if err := conn.WriteMessage(msgType, rpl); err != nil {
				log.Println(err)
				return
			}
		}
	}
}

func publishNewMsgs(conn *websocket.Conn, redisdb *redis.Client, ctx context.Context) {
	defer conn.Close()
	// Subscribe to Redis channel and close at the end
	pubsub := redisdb.Subscribe(ctx, REDIS_CHANNEL)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
		if err != nil {
			log.Println(err)
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
		log.Printf("New message received from %s", r.RemoteAddr)
		msg := Message{
			Name:    html.EscapeString(r.FormValue("name")),
			Email:   html.EscapeString(r.FormValue("email")),
			Date:    html.EscapeString(time.Now().Format(time.ANSIC)),
			Topic:   html.EscapeString(r.FormValue("topic")),
			Content: html.EscapeString(r.FormValue("content")),
		}

		// Redis
		redisdb := redis.NewClient(&redis.Options{
			Addr:     "redis:6379",
			Password: "",
			DB:       0,
		})
		ctx := context.Background()

		//////////////////// Replace with DB
		// Get ID for next message from Redis
		// This isn't ideal for reasons noted below
		msgId, err := redisdb.Incr(ctx, REDIS_ID_KEY).Result()
		if err != nil {
			log.Println(err)
			return
		}
		msg.Id = msgId

		// Marshal message into json so we can store in Redis
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
		///////////////////

		// Re-encode the message as an array of messages (expected by frontend)
		msgJson, err = json.Marshal([]Message{msg})
		if err != nil {
			log.Println(err)
			return
		}

		// Publish new message to clients subscribed to Redis channel
		if err := redisdb.Publish(ctx, REDIS_CHANNEL, msgJson).Err(); err != nil {
			log.Println(err)
			return
		}
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

	log.Printf("Client connection from %s", r.RemoteAddr)

	// Redis
	ctx := context.Background()
	redisdb := redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "",
		DB:       0,
	})

	// handle initial request for stored messages
	go loadStoredMsgs(conn, redisdb, ctx)

	// listen for new messages published to Redis channel
	go publishNewMsgs(conn, redisdb, ctx)
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
