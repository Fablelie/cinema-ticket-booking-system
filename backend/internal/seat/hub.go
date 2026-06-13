package seat

import (
	"context"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	// เก็บที่อยู่ Connection ของผู้ใช้ทุกคน: true หมายถึงกำลังเชื่อมต่ออยู่
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.Mutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

// Run เปิดด้ายหลักของ Hub เพื่อคอยต้อนรับคนเข้า/ออก และพ่นข้อมูลแบบ Concurrency-Safe
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Println("[WEBSOCKET HUB] User connected.")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
				log.Println("[WEBSOCKET HUB] User disconnected.")
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				err := client.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) BroadcastRawMessage(message []byte) {
	h.broadcast <- message
}
