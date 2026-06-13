package seat

import (
	"context"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rabbitmq/amqp091-go"
)

type Hub struct {
	// เก็บที่อยู่ Connection ของผู้ใช้ทุกคน: true หมายถึงกำลังเชื่อมต่ออยู่
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.Mutex
	mqCh       *amqp091.Channel
}

func NewHub(ch *amqp091.Channel) *Hub {
	return &Hub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		mqCh:       ch,
	}
}

// Run เปิดด้ายหลักของ Hub เพื่อคอยต้อนรับคนเข้า/ออก และพ่นข้อมูลแบบ Concurrency-Safe
func (h *Hub) Run(ctx context.Context) {
	// เปิดฟังคิว RabbitMQ ควบคู่ไปด้วย เพื่อให้รู้ว่าตอนไหนต้องสั่งอัปเดตสีหน้าจอคนอื่น
	msgs, err := h.mqCh.Consume(
		"booking_events", // ดึงข้อมูลคิวชุดเดียวกับ Audit Log
		"", true, false, false, false, nil,
	)
	if err == nil {
		go func() {
			for d := range msgs {
				// เมื่อมีใครกดจอง/ล็อก/ปล่อยตั๋ว ส่งสารนั้นไปกระจายบอกหน้าบ้านทุกคนทันที
				h.broadcast <- d.Body
			}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Println("[WEBSOCKET HUB] New user connected.")

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
					log.Printf("[WEBSOCKET HUB] Write error: %v", err)
					client.Close()
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}
