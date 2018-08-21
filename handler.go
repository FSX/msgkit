// msgkit is a simple websocket json message handling package. It makes it
// quick and easy to write a websocket server using traditional http style
// request/message handlers.

package msgkit

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
)

// Handler is a package of all required dependencies to run a msgkit websocket
// server
type Handler struct {
	socks    sync.Map           // holds the websockets
	upgrader websocket.Upgrader // shared upgrader

	// Event handlers for all connections
	handlers map[string]func(id, msg string)

	// OnOpen binds an on-open handler to the server which will be triggered
	// every time a connection is made
	OnOpen func(id string)

	// OnClose binds an on-close handler to the server which will trigger every
	// time a connection is closed
	OnClose func(id string)
}

// Handle adds a HandlerFunc to the map of websocket message handlers
func (h *Handler) Handle(name string, handler func(id, msg string)) {
	if h.handlers == nil {
		h.handlers = make(map[string]func(id, msg string))
	}
	h.handlers[name] = handler
}

// Send a message to a websocket.
func (h *Handler) Send(id string, message string) {
	if v, ok := h.socks.Load(id); ok {
		v.(*websocket.Conn).WriteMessage(1, []byte(message))
	}
}

// Range ranges over all ids
func (h *Handler) Range(f func(id string) bool) {
	h.socks.Range(func(key, value interface{}) bool {
		return f(key.(string))
	})
}

// ServeHTTP is the primary websocket handler method and conforms to the
// http.Handler interface.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Open and register the websocket
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("register:", err)
		return
	}
	defer conn.Close() // Defer close the websocket

	// generate a unique identifier
	var b [12]byte
	rand.Read(b[:])
	id := hex.EncodeToString(b[:])

	// Store the sockets
	h.socks.Store(id, conn)
	defer h.socks.Delete(id) // Defer unregister the connection

	// Trigger the OnOpen handler if one is defined
	if h.OnOpen != nil {
		h.OnOpen(id)
	}

	if h.OnClose != nil {
		// Defer trigger the OnClose handler if one is defined
		defer h.OnClose(id)
	}

	// For every message that comes through on the connection
	for {
		// Read the next message on the connection
		_, msgb, err := conn.ReadMessage()
		if err != nil {
			return
		}

		// JSON decode the type from the json formatted message
		msgType := gjson.GetBytes(msgb, "type").String()

		// If a handler exists for the message type, handle it
		if fn, ok := h.handlers[msgType]; ok {
			fn(id, string(msgb))
		} else {
			// Send an error back to the client letting them know that the
			// incoming type is unknown
			h.Send(id, `{"type":"Error","message":"Unknown type"}`)
		}
	}
}
