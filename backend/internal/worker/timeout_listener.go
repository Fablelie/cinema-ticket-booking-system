package worker

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/fablelie/cinema-ticket-booking-system/internal/seat"
	"github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

type TimeoutListener struct {
	redisClient *redis.Client
	seatRepo    *seat.Repository
	mqCh        *amqp091.Channel
}

func NewTimeoutListener(rdb *redis.Client, seatRepo *seat.Repository, ch *amqp091.Channel) *TimeoutListener {
	return &TimeoutListener{
		redisClient: rdb,
		seatRepo:    seatRepo,
		mqCh:        ch,
	}
}

// Start ดักฟัง Event คีย์หมดอายุจาก Redis แบบ Background Task
func (t *TimeoutListener) Start(ctx context.Context) {
	// ดักฟังเฉพาะช่องสัญญาณ Event Expired ของฐานข้อมูลที่ 0
	pubsub := t.redisClient.Subscribe(ctx, "__keyevent@0__:expired")

	log.Println("Redis Timeout Listener started... Waiting for seat expiration events.")

	go func() {
		defer pubsub.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-pubsub.Channel():
				// msg.Payload จะส่งชื่อคีย์ที่เพิ่งหมดอายุมา เช่น "lock:showtime:64b1f:seat:A1"
				key := msg.Payload

				// ตรวจสอบว่าเป็นคีย์ล็อกที่นั่งของเราจริงไหม
				if strings.HasPrefix(key, "lock:showtime:") {
					log.Printf("[TIMEOUT WORKER] Detected expired lock: %s", key)
					t.handleExpiredLock(context.Background(), key)
				}
			}
		}
	}()
}

func (t *TimeoutListener) handleExpiredLock(ctx context.Context, key string) {
	// แยกข้อความเพื่อแกะเอา showtime_id และ seat_no ออกมาใช้งาน
	// รูปแบบคีย์: lock:showtime:{showtime_id}:seat:{seat_no}
	parts := strings.Split(key, ":")
	if len(parts) < 5 {
		return
	}

	showtimeID := parts[2]
	seatNo := parts[4]

	showtime, err := t.seatRepo.GetShowtimeByID(ctx, showtimeID)
	if err == nil {
		for _, s := range showtime.Seats {
			if s.SeatNo == seatNo && s.Status == "BOOKED" {
				log.Printf("[TIMEOUT WORKER] Seat %s is already BOOKED. Skipping timeout release.", seatNo)
				return
			}
		}
	}

	// 1. 🔄 สั่งเปลี่ยนสถานะเก้าอี้กลับไปเป็น AVAILABLE (สีแดง) ใน MongoDB ทันที
	err = t.seatRepo.UpdateSeatStatus(ctx, showtimeID, seatNo, seat.SeatStatus("AVAILABLE"), "")
	if err != nil {
		log.Printf("[TIMEOUT WORKER] Error updating MongoDB for seat %s: %v", seatNo, err)
		return
	}

	// 2. 📣 ส่ง Event "SEATS_RELEASED" บอก RabbitMQ เพื่อให้ฝั่ง WebSocket สั่งเปลี่ยนสีจอคนอื่นเป็นสีแดง
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"event":       "SEATS_RELEASED",
		"showtime_id": showtimeID,
		"seats":       []string{seatNo},
	})

	// ใช้ background context สำหรับ publishing ใน background worker
	if err := t.mqCh.PublishWithContext(context.Background(),
		"",               // exchange
		"booking_events", // queue name
		false, false,
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        eventPayload,
		},
	); err != nil {
		log.Printf("[TIMEOUT WORKER] Failed to publish SEATS_RELEASED event: %v", err)
	}

	log.Printf("[TIMEOUT WORKER] Successfully released seat %s for showtime %s due to timeout", seatNo, showtimeID)
}
