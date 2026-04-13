package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	conn    *websocket.Conn
	spaceID string
	userID  string
	send    chan []byte
}

type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[*Client]bool
}

var globalHub = &Hub{
	rooms: make(map[string]map[*Client]bool),
}

type BroadcastMessage struct {
	SpaceID   string `json:"space_id"`
	SenderID  string `json:"sender_id"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

func (h *Hub) register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.rooms[client.spaceID]; !ok {
		h.rooms[client.spaceID] = make(map[*Client]bool)
	}
	h.rooms[client.spaceID][client] = true
}

func (h *Hub) unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.rooms[client.spaceID]; !ok {
		return
	}
	delete(h.rooms[client.spaceID], client)
	if len(h.rooms[client.spaceID]) == 0 {
		delete(h.rooms, client.spaceID)
	}
}

func (h *Hub) broadcast(sender *Client, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	room := h.rooms[sender.spaceID]
	for c := range room {
		if c == sender {
			continue
		}
		select {
		case c.send <- payload:
		default:
			// drop on slow consumer
		}
	}
}

func WsSpace(c *gin.Context) {
	spaceID := c.Query("space_id")
	userID := c.DefaultQuery("user_id", "")
	if spaceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "space_id is required"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("failed to upgrade: ", err)
		return
	}

	client := &Client{
		conn:    conn,
		spaceID: spaceID,
		userID:  userID,
		send:    make(chan []byte, 32),
	}
	globalHub.register(client)
	defer func() {
		globalHub.unregister(client)
		close(client.send)
		_ = conn.Close()
	}()

	go writePump(client)
	readPump(client)
}

func readPump(client *Client) {
	for {
		_, message, err := client.conn.ReadMessage()
		if err != nil {
			log.Println("disconnected: ", err)
			return
		}
		payload, err := json.Marshal(BroadcastMessage{
			SpaceID:   client.spaceID,
			SenderID:  client.userID,
			Message:   string(message),
			Timestamp: time.Now().UnixMilli(),
		})
		if err != nil {
			continue
		}
		globalHub.broadcast(client, payload)
	}
}

func writePump(client *Client) {
	for msg := range client.send {
		if err := client.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}