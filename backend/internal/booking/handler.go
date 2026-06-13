package booking

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/fablelie/cinema-ticket-booking-system/internal/seat"
	"github.com/gin-gonic/gin"
	"github.com/rabbitmq/amqp091-go"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ReserveHandler จัดการเมื่อผู้ใช้กดปุ่ม Next บนหน้า p_seat เพื่อขอล็อกที่นั่ง (All-or-Nothing)
func (h *Handler) ReserveHandler(c *gin.Context) {
	// 1. ดึง user_id ที่แกะและผ่านการตรวจสอบสิทธิ์มาจาก GoogleAuthMiddleware
	userIDValue, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized: user context not found"})
		return
	}
	userID := userIDValue.(string)

	// 2. แกะข้อมูลพารามิเตอร์ที่หน้าบ้านส่งมา (showtime_id, seats)
	var req ReserveSeatsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 3. ส่งข้อมูลไปประมวลผลที่ Service Layer สำหรับการทำ Distributed Lock
	ctx := c.Request.Context()
	res, err := h.service.ReserveSeats(ctx, userID, req)
	if err != nil {
		// หากจองไม่สำเร็จ (เช่น มีคนแย่งล็อกเก้าอี้ตัวใดตัวหนึ่งไปก่อนในเสี้ยววินาทีนั้น) ส่ง 409 Conflict กลับไป
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	// 4. หากสำเร็จ ส่งข้อมูลและ Timestamp เวลาหมดอายุกลับไปให้หน้าบ้านเปิดหน้า p_confirm และเริ่มนับเวลาถอยหลัง 5 นาที
	c.JSON(http.StatusOK, res)
}

// CancelHandler จัดการเมื่อผู้ใช้กดปุ่ม Cancel บนหน้า p_confirm เพื่อปล่อยเก้าอี้กลับเป็น AVAILABLE
func (h *Handler) CancelHandler(c *gin.Context) {
	userIDValue, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID := userIDValue.(string)

	var req ReserveSeatsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	err := h.service.CancelReservation(ctx, userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "reservation canceled successfully"})
}

// ConfirmHandler จัดการเมื่อผู้ใช้กดปุ่ม Confirm บนหน้า p_confirm เพื่อเปลี่ยนสถานะเป็น BOOKED ถาวร
func (h *Handler) ConfirmHandler(c *gin.Context) {
	userIDValue, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID := userIDValue.(string)

	var req ReserveSeatsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// 1. อัปเดตสถานะเก้าอี้ใน MongoDB จาก LOCKED ให้กลายเป็น BOOKED (สีเทา) ถาวร
	for _, seatNo := range req.Seats {
		err := h.service.seatRepo.UpdateSeatStatus(ctx, req.ShowtimeID, seatNo, seat.SeatStatus("BOOKED"), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to confirm seat"})
			return
		}
	}

	// 2. ปลดล็อกเก้าอี้ออกจาก Redis เพื่อคืนสิทธิ์ระบบ (เคลียร์ Memory) เนื่องจากทำธุรกรรมเสร็จสิ้นสมบูรณ์แล้ว
	_ = h.service.bookingRepo.ReleaseMultipleLocks(ctx, req.ShowtimeID, req.Seats, userID)

	// 3. ส่ง Event "BOOKING_SUCCESS" เข้า RabbitMQ เพื่อให้ Worker นำไปบันทึกเป็น Audit Log ข้อมูลประวัติระบบแบบอซิงโครนัส
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"event":       "BOOKING_SUCCESS",
		"showtime_id": req.ShowtimeID,
		"seats":       req.Seats,
		"user_id":     userID,
		"timestamp":   time.Now(),
	})
	// ใช้ background context สำหรับ async publishing เพื่อหลีกเลี่ยง request context ที่อาจถูกยกเลิก
	if err := h.service.mqCh.PublishWithContext(context.Background(),
		"", "booking_events", false, false,
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        eventPayload,
		},
	); err != nil {
		log.Printf("[RabbitMQ] Failed to publish BOOKING_SUCCESS event: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "booking confirmed successfully"})
}
