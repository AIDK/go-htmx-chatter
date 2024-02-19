package main

import (
	"bytes"
	"log"
	"sync"
	"text/template"
)

type Message struct {
	ClientId string // client id
	Text     string // message text
}

type WSMessage struct {
	Headers interface{} `json:"HEADERS"`
	Text    string      `json:"text"`
}

type Hub struct {
	sync.RWMutex
	clients    map[*Client]bool // registered clients
	messages   []*Message       // message history
	broadcast  chan *Message    // broadcast channel (send message to all clients)
	register   chan *Client     // register channel (add client to hub)
	unregister chan *Client     // unregister channel (remove client from hub)
}

// NewHub creates a new hub
func NewHub() *Hub {
	return &Hub{
		broadcast:  make(chan *Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) Run() {
	// this will listen for messages and broadcast them to clients
	for {
		select {
		case client := <-h.register:
			// add the client to the hub
			h.Lock()
			h.clients[client] = true // this isnt concurrent safe so we have to set a lock
			h.Unlock()

			log.Printf("client %s connected", client.id)
		case client := <-h.unregister:
			// we can remove the client from the hub,
			// but first we need to check if the client exists
			if _, ok := h.clients[client]; ok {
				log.Printf("client %s disconnected", client.id)
				// we close the send channel to prevent the client from hanging the connection open
				close(client.send)
				h.Lock()
				delete(h.clients, client)
				h.Unlock()
			}
		case msg := <-h.broadcast:
			// we add the message to the message history
			h.messages = append(h.messages, msg)

			for client := range h.clients {
				select {
				// here we send the message to the client but we're going
				// to use HTMX template to render the message.
				// If we were using JSON, here we would be returning the JSON to the client
				case client.send <- getMessageTemplate(msg):
				default:
					// we close  the connection if the send channel is closed
					close(client.send)
					// we remove the client from the hub
					delete(h.clients, client)
				}
			}
		}
	}
}

// getMessageTemplate returns the message template as a byte array to be sent to the client
func getMessageTemplate(msg *Message) []byte {

	// we're going to use HTMX to render the message and return the template
	// as a byte array to be sent to the client, so we parse the template file
	tmpl, err := template.ParseFiles("templates/message.html")
	// if there are any errors during the parse process, we log the error and exit
	if err != nil {
		log.Fatalf("template parsing: %s", err)
	}

	// we create a buffer to write the template to
	// because the template is a byte array
	var renderMsg bytes.Buffer
	// we execute the template and write it to the buffer we created
	err = tmpl.Execute(&renderMsg, msg)
	// if there are any errors during the execution process, we log the error and exit
	if err != nil {
		log.Fatalf("template executing: %s", err)
	}

	// we return the buffer as a byte array to be sent to the client
	return renderMsg.Bytes()
}
