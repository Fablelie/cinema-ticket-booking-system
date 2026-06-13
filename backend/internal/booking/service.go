package booking

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/fablelie/cinema-ticket-booking-system/internal/seat"
	"github.com/rabbitmq/amqp091-go"
)

type Service struct {
	bookingRepo *Repository
	seatRepo    *seat.Repository
	mqCh        *amqp091.Channel
}

func NewService(bookingRepo *Repository, seatRepo *seat.Repository, ch *amqp091.Channel) *Service {
	return &Service{
		bookingRepo: bookingRepo,
		seatRepo:    seatRepo,
		mqCh:        ch,
	}
}

// ReserveSeats ดำเนินการล็อกเก้าอี้หลายตัวพร้อมกันแบบ All-or-Nothing
func (s *Service) ReserveSeats(ctx context.Context, userID string, req ReserveSeatsReq) (*BookingResponse, error) {
	ttl := 5 * time.Minute
	lockedUntil := time.Now().Add(ttl)

	// 1. 🔒 เรียกคำสั่งทำ Distributed Lock ใน Redis แบบรวดเดียวผ่าน Pipeline
	success, err := s.bookingRepo.AcquireMultipleLocks(ctx, req.ShowtimeID, req.Seats, userID, ttl)
	if err != nil {
		return nil, err
	}
	if !success {
		// หากมีเก้าอี้แม้แต่ตัวเดียวโดนคนอื่นล็อกตัดหน้าไปก่อน ระบบจะสั่ง Reject ทันที!
		return nil, errors.New("one or more selected seats are already locked or booked")
	}

	// 2. 📝 อัปเดตสถานะเก้าอี้ตัวที่เลือกลงในฐานข้อมูลหลัก MongoDB ให้เป็น LOCKED (สีส้ม)
	for _, seatNo := range req.Seats {
		err := s.seatRepo.UpdateSeatStatus(ctx, req.ShowtimeID, seatNo, seat.SeatStatus("LOCKED"), userID)
		if err != nil {
			// กรณีอัปเดต MongoDB พลาด ให้สั่ง Rollback ลบ Lock ใน Redis คืนทันทีเพื่อความปลอดภัย
			_ = s.bookingRepo.ReleaseMultipleLocks(ctx, req.ShowtimeID, req.Seats, userID)
			return nil, err
		}
	}

	// 3. 📣 ส่ง Event เข้า RabbitMQ เพื่อให้ Worker เอาไป Broadcast ออก WebSocket บอกหน้าบ้านคนอื่น
	eventPayload, err := json.Marshal(map[string]interface{}{
		"event":        "SEATS_LOCKED",
		"showtime_id":  req.ShowtimeID,
		"seats":        req.Seats,
		"user_id":      userID,
		"locked_until": lockedUntil,
	})

	if err != nil {
		log.Printf("[BOOKING SERVICE] json marshal err ")
	}

	log.Printf("[BOOKING SERVICE] Publishing SEATS_LOCKED event for user %s, seats: %v", userID, req.Seats)

	// ใช้ background context สำหรับ async publishing เพื่อหลีกเลี่ยง request context ที่อาจถูกยกเลิก
	if err := s.mqCh.PublishWithContext(context.Background(),
		"",               // exchange
		"booking_events", // queue name
		false, false,
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        eventPayload,
		},
	); err != nil {
		log.Printf("[RabbitMQ] Failed to publish SEATS_LOCKED event: %v", err)
	} else {
		log.Printf("[RabbitMQ] Successfully published SEATS_LOCKED event")
	}

	// 4. 🚀 ส่งข้อมูลสรุปกลับไปให้หน้าบ้านเอาใช้เปิดหน้า p_confirm และเริ่มนับเวลาถอยหลัง 5 นาที
	return &BookingResponse{
		Seats:       req.Seats,
		ShowtimeID:  req.ShowtimeID,
		LockedUntil: lockedUntil,
		Status:      "LOCKED",
	}, nil
}

// CancelReservation ปลดล็อกเก้าอี้ทันทีเมื่อผู้ใช้เปลี่ยนใจกดปุ่ม Cancel บนหน้า p_confirm
func (s *Service) CancelReservation(ctx context.Context, userID string, req ReserveSeatsReq) error {
	// 1. ปลดล็อกใน Redis อย่างปลอดภัย (เช็กความเป็นเจ้าของด้วย Lua Script เสมอ)
	err := s.bookingRepo.ReleaseMultipleLocks(ctx, req.ShowtimeID, req.Seats, userID)
	if err != nil {
		return err
	}

	// 2. ปรับสถานะเก้าอี้ใน MongoDB คืนกลับไปเป็น AVAILABLE (สีแดง)
	for _, seatNo := range req.Seats {
		_ = s.seatRepo.UpdateSeatStatus(ctx, req.ShowtimeID, seatNo, seat.SeatStatus("AVAILABLE"), "")
	}

	// 3. ส่ง Event แจ้ง RabbitMQ ว่าเก้าอี้ว่างแล้ว หน้าจอคนอื่นจะได้เปลี่ยนเป็นสีแดงทันที
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"event":       "SEATS_RELEASED",
		"showtime_id": req.ShowtimeID,
		"seats":       req.Seats,
	})

	// ใช้ background context สำหรับ async publishing เพื่อหลีกเลี่ยง request context ที่อาจถูกยกเลิก
	if err := s.mqCh.PublishWithContext(context.Background(),
		"",
		"booking_events",
		false, false,
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        eventPayload,
		},
	); err != nil {
		log.Printf("[RabbitMQ] Failed to publish SEATS_RELEASED event: %v", err)
	}

	return nil
}
