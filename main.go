package main

import (
	"log"
	"net/http"
)

func main() {

	// create a new hub (this will manage the clients and messages)
	hub := NewHub()
	// start the hub (this will listen for messages and broadcast them to clients)
	go hub.Run()

	// this will handle serving the landing page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {

		// if the request is not for the root path, return a 404
		if r.URL.Path != "/" {
			http.Error(w, "404 not found.", http.StatusNotFound)
			return
		}

		// if the request method is not GET, return a 404
		if r.Method != "GET" {
			http.Error(w, "Method is not supported.", http.StatusNotFound)
			return
		}

		// serve the index.html file
		http.ServeFile(w, r, "templates/index.html")
	})

	// this will handle the websocket connection
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})

	log.Fatal(http.ListenAndServe(":3000", nil))
}
