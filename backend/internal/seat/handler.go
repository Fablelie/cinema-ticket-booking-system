package seat

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// เปิดสิทธิ์ยอมรับคำขอข้าม Domain (CORS) เพื่อให้ Vue 3 เรียกใช้งานได้ไม่มีปัญหา
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Handler struct {
	service *Service
	hub     *Hub // เพิ่มตัวแปร Hub เข้ามาใช้งานร่วมกัน
}

func NewHandler(service *Service, hub *Hub) *Handler {
	return &Handler{
		service: service,
		hub:     hub,
	}
}

// GetSeatMapHandler (ฟังก์ชันดั้งเดิม ยิงมาดึงผังรอบแรก)
func (h *Handler) GetSeatMapHandler(c *gin.Context) {
	showtimeID := c.Param("showtime_id")
	if showtimeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "showtime_id is required"})
		return
	}

	ctx := c.Request.Context()
	showtime, err := h.service.GetSeatMap(ctx, showtimeID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "showtime not found"})
		return
	}

	c.JSON(http.StatusOK, showtime)
}

// WebSocketHandler สลับโปรโตคอลเข้าสู่โหมดส่งสีปุ่มเก้าอี้แบบเรียลไทม์
func (h *Handler) WebSocketHandler(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	// ส่ง Connection นี้เข้าไปลงทะเบียนใน Hub ส่วนกลาง
	h.hub.register <- conn

	// เปิดการดักฟังข้อความสั้นๆ เผื่อกรณีฝั่งหน้าบ้านมีการส่งสัญญาณชีพ (Heartbeat/Ping)
	go func() {
		defer func() {
			h.hub.unregister <- conn
		}()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break // หากผู้ใช้ปิดหน้าต่างเบราว์เซอร์ลูปจะหลุดออก
			}
		}
	}()
}
