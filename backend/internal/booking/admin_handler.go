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
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetAdminDashboardDataHandler ดึงข้อมูลประวัติกิจกรรม (Audit Logs) ทั้งหมดมาโชว์บน Dashboard พร้อมระบบ Filter (Requirement 2.2)
func (h *Handler) GetAdminDashboardDataHandler(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := h.service.bookingRepo.db.Collection("audit_logs")

	// 🔍 1. สร้าง Filter Dynamic ตามพารามิเตอร์ที่แอดมินส่งมาจาก Query Params หน้าบ้าน
	filter := bson.M{}

	// Filter ตามรหัสผู้ใช้: ?user_id=xxxxx
	if userID := c.Query("user_id"); userID != "" {
		filter["user_id"] = userID
	}

	// Filter ตามประเภทเหตุการณ์: ?event=BOOKING_SUCCESS
	if event := c.Query("event"); event != "" {
		filter["event"] = event
	}

	// Filter ตามวันที่ (ค้นหาประวัติย้อนหลังของวันนั้นๆ): ?date=2026-06-13
	if dateStr := c.Query("date"); dateStr != "" {
		t, err := time.Parse("2006-01-02", dateStr)
		if err == nil {
			// ดึงข้อมูลตั้งแต่เวลา 00:00:00 ถึง 23:59:59 ของวันนั้น
			nextDay := t.AddDate(0, 0, 1)
			filter["timestamp"] = bson.M{
				"$gte": t,
				"$lt":  nextDay,
			}
		}
	}

	// 2. ดึงข้อมูลจาก MongoDB เรียงจากเหตุการณ์ล่าสุดขึ้นก่อน (Sort Descending)
	findOptions := options.Find().SetSort(bson.D{{Key: "timestamp", Value: -1}})
	cursor, err := collection.Find(ctx, filter, findOptions)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch audit logs"})
		return
	}
	defer cursor.Close(ctx)

	var logs []bson.M
	if err = cursor.All(ctx, &logs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse audit logs"})
		return
	}

	// 3. ส่งข้อมูลกลับไปให้หน้า Admin Dashboard ของ Vue 3 ในรูป JSON
	c.JSON(http.StatusOK, logs)
}

func (h *Handler) ResetSeatsHandler(c *gin.Context) {
	ctx := context.Background()
	var req AdminResetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 1. ล้างคีย์ล็อกเก้าอี้ทั้งหมด (A1-B5) ใน Redis ออกให้หมด
	allSeats := []string{"A1", "A2", "A3", "A4", "A5", "B1", "B2", "B3", "B4", "B5"}
	_ = h.service.bookingRepo.ReleaseMultipleLocks(ctx, req.ShowtimeID, allSeats, "")

	// 2. ปรับสถานะเก้าอี้ทุกตัวใน MongoDB กลับเป็น AVAILABLE (สีแดง)
	for _, seatNo := range allSeats {
		_ = h.service.seatRepo.UpdateSeatStatus(ctx, req.ShowtimeID, seatNo, seat.SeatStatus("AVAILABLE"), "")
	}

	// 3. บันทึกประวัติลงคิว RabbitMQ ว่าแอดมินทำการรีเซ็ตระบบ
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"event":       "SYSTEM_RESET_BY_ADMIN",
		"showtime_id": req.ShowtimeID,
		"seats":       allSeats,
		"timestamp":   time.Now(),
	})
	// ใช้ background context สำหรับ async publishing เพื่อหลีกเลี่ยง request context ที่อาจถูกยกเลิก
	if err := h.service.mqCh.PublishWithContext(context.Background(), "", "booking_events", false, false,
		amqp091.Publishing{ContentType: "application/json", Body: eventPayload}); err != nil {
		log.Printf("[RabbitMQ] Failed to publish SYSTEM_RESET_BY_ADMIN event: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "system reset completed successfully"})
}
