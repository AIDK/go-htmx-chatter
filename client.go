package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Client struct {
	id   string          // unique identifier for the client
	hub  *Hub            // the hub that the client is connected to
	conn *websocket.Conn // the websocket connection
	send chan []byte     // buffered channel of outbound messages
}

const (
	// time allowed the read the next pong message from the peer
	pongWait = 60 * time.Second
	// maximum message size allowed from the peer
	maxMessageSize = 512
	// send pings to peer with this period, must be less than pongWait
	pingPeriod = (pongWait * 9) / 10
	// time allowed to write a message to the peer
	writeWait = 10 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {

	// upgrade the HTTP server connection to a websocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not open websocket connection", http.StatusBadRequest)
		return
	}

	id := uuid.New().String()

	// create the client
	client := &Client{
		id:   id,
		hub:  hub,
		conn: conn,
		send: make(chan []byte),
	}

	// register the client with the hub
	client.hub.register <- client

	// start the client write and read pumps
	go client.writePump()
	go client.readPump()
}

// readPump pumps messages from the websocket connection to the hub.
func (c *Client) readPump() {

	defer func() {
		// close the connection when the function returns
		c.conn.Close()
		// unregister the client from the hub
		c.hub.unregister <- c
	}()

	// set the read limit for the connection,
	// this is to prevent the client from sending large messages
	c.conn.SetReadLimit(maxMessageSize)
	// set the read deadline for the connection,
	// this is to prevent the client from hanging the connection open
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	// set the pong handler for the connection,
	// this is to handle the pong message sent by the client
	c.conn.SetPingHandler(func(appData string) error {
		// set the read deadline for the connection,
		// this is to prevent the client from hanging the connection open
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// we start listening for messages from the client
	for {
		// read a message from the connection
		_, text, err := c.conn.ReadMessage()
		log.Printf("value %v", string(text))
		// we have to handle the error here otherwise the connection will hang open,
		// and the client will not be able to send any more messages
		if err != nil {
			// we log the error, and check if it is an unexpected close error (client disconnected)
			// if it is not, we break the loop and close the connection
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break // break the loop if there is an error (client disconnected)
		}

		// create a message from the text sent by the client
		msg := &WSMessage{}
		// create a reader from the text
		reader := bytes.NewReader(text)
		// create a decoder from the reader
		decoder := json.NewDecoder(reader)
		// decode the message from the decoder
		err = decoder.Decode(&msg)
		if err != nil {
			log.Printf("error: %v", err)
		}

		// create a message with the client id and the message text
		c.hub.broadcast <- &Message{
			ClientId: c.id,
			Text:     msg.Text,
		}
	}

}

// writePump pumps messages from the hub to the websocket connection.
func (c *Client) writePump() {

	ticker := time.NewTicker(pingPeriod)
	defer func() {
		// close the connection when the function returns (in case something goes wrong)
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			// set the write deadline for the connection,
			// this is to prevent the client from hanging the connection open
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// we can send a close message to the client
				// and return if the channel is closed (hub closed the channel)
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// reading a message sent to us from the hub
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			// write the message to the connection
			w.Write(msg)

			// add queued chat messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(msg)
			}

			if err := w.Close(); err != nil {
				return // this should be handled better
			}

		case <-ticker.C:
			// set the write deadline for the connection,
			// this is to prevent the client from hanging the connection open
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return // this should be handled better
			}
		}
	}
}
