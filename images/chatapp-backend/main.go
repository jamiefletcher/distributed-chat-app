package main

import (
	"fmt"
	"log"
	"net/http"
)

func basicEndpoint(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Basic endpoint")
}

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Websocket endpoint")
}

func setupRoutes() {
	http.HandleFunc("/", basicEndpoint)
	http.HandleFunc("/ws", wsEndpoint)
}

func main() {
	fmt.Println("Hello world")
	setupRoutes()
	log.Fatal(http.ListenAndServe(":8080", nil))
}
