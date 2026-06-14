package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/fablelie/cinema-ticket-booking-system/internal/seat"
	"github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/mongo"
)

// AuditLog โครงสร้างข้อมูลสำหรับบันทึกประวัติกิจกรรมลง MongoDB
type AuditLog struct {
	Event      string    `bson:"event" json:"event"` // SEATS_LOCKED, BOOKING_SUCCESS, SEATS_RELEASED
	ShowtimeID string    `bson:"showtime_id" json:"showtime_id"`
	Seats      []string  `bson:"seats" json:"seats"`
	UserID     string    `bson:"user_id,omitempty" json:"user_id,omitempty"`
	ErrorMsg   string    `bson:"error_msg,omitempty" json:"error_msg,omitempty"`
	Timestamp  time.Time `bson:"timestamp" json:"timestamp"`
}

type RabbitConsumer struct {
	mqCh *amqp091.Channel
	db   *mongo.Database
	hub  *seat.Hub
}

func NewRabbitConsumer(ch *amqp091.Channel, db *mongo.Database, hub *seat.Hub) *RabbitConsumer {
	return &RabbitConsumer{
		mqCh: ch,
		db:   db,
		hub:  hub,
	}
}

// Start เปิดด้ายเบื้องหลังคอยดึงข้อมูลจากคิว RabbitMQ ตลอดเวลา
func (c *RabbitConsumer) Start(ctx context.Context) {
	// 1. ลงทะเบียนขอดึงข้อมูลจากคิว booking_events
	msgs, err := c.mqCh.Consume(
		"booking_events", // queue name
		"",               // consumer tag
		true,             // auto-ack (ยอมรับข้อความทันทีเมื่อดึงออกไป)
		false,            // exclusive
		false,            // no-local
		false,            // no-wait
		nil,              // args
	)
	if err != nil {
		log.Fatalf("[RABBIT CONSUMER] Failed to register a consumer: %v", err)
	}

	log.Println("RabbitMQ Consumer started... Listening for booking events to write Audit Logs.")

	// 2. แตก Goroutine คอยรับข้อมูลมาบันทึกลง Database
	go func() {
		collection := c.db.Collection("audit_logs")

		for {
			select {
			case <-ctx.Done():
				return
			case d := <-msgs:
				log.Printf("[RABBITMQ RAW RECEIVED] %s", string(d.Body))

				if c.hub != nil {
					c.hub.BroadcastRawMessage(d.Body)
				}

				// แปลงข้อความ JSON ที่ได้จากคิวให้กลับมาเป็น Struct
				var rawData map[string]interface{}
				if err := json.Unmarshal(d.Body, &rawData); err != nil {
					log.Printf("[RABBIT CONSUMER] Error parsing message body: %v", err)
					continue
				}

				// ดึงค่าพื้นฐานออกมาประกอบร่างเป็น AuditLog
				event, ok := rawData["event"].(string)
				if !ok || event == "" {
					log.Printf("[RABBIT CONSUMER] Warning: Invalid or missing event field in message")
					continue
				}
				log.Printf("[RABBIT CONSUMER] get showtime_id : %s |", event)
				showtimeID, _ := rawData["showtime_id"].(string)
				log.Printf("[RABBIT CONSUMER] get user_id : %s |", event)
				userID, _ := rawData["user_id"].(string)

				errorMsg, _ := rawData["error_msg"].(string)

				log.Printf("[RABBIT CONSUMER] finish get user_id : %s |", event)
				// แปลงอินเทอร์เฟซของรายชื่อเก้าอี้กลับมาเป็น Array ของ String
				var seats []string
				if rawSeats, ok := rawData["seats"].([]interface{}); ok {
					for _, s := range rawSeats {
						if strSeat, ok := s.(string); ok {
							seats = append(seats, strSeat)
						}
					}
				}

				// ดึง Timestamp จากเหตุการณ์ (ถ้ามี) หรือใช้เวลาปัจจุบัน
				var eventTimestamp time.Time
				if rawTimestamp, ok := rawData["timestamp"]; ok {
					switch ts := rawTimestamp.(type) {
					case string:
						// พยายามแปลง timestamp string หลายรูปแบบ
						if parsed, err := time.Parse(time.RFC3339Nano, ts); err == nil {
							eventTimestamp = parsed
						} else if parsed, err := time.Parse("2006-01-02T15:04:05Z07:00", ts); err == nil {
							eventTimestamp = parsed
						} else {
							eventTimestamp = time.Now()
							log.Printf("[RABBIT CONSUMER] Warning: Could not parse timestamp string '%s', using current time", ts)
						}
					case float64:
						// ถ้าเป็น Unix timestamp (seconds)
						eventTimestamp = time.Unix(int64(ts), 0)
					default:
						eventTimestamp = time.Now()
					}
				} else {
					eventTimestamp = time.Now()
				}

				// จัดเตรียมข้อมูล Log เพื่อบันทึก
				auditLog := AuditLog{
					Event:      event,
					ShowtimeID: showtimeID,
					Seats:      seats,
					UserID:     userID,
					ErrorMsg:   errorMsg,
					Timestamp:  eventTimestamp, // ใช้เวลาที่เกิดเหตุการณ์จริง มิใช่เวลาบันทึก
				}

				log.Printf("[RABBIT CONSUMER] collection insertOne Event: %s |", event)

				// 3. 💾 เขียนลง MongoDB คอลเลกชัน audit_logs
				_, err := collection.InsertOne(context.Background(), auditLog)
				if err != nil {
					log.Printf("[RABBIT CONSUMER] Error inserting audit log into MongoDB: %v", err)
				} else {
					// d.Ack(false)
					log.Printf("[AUDIT LOG SAVED AA] Event: %s | Showtime: %s | Seats: %v | User: %s | Timestamp: %s",
						event, showtimeID, seats, userID, eventTimestamp.Format("2006-01-02 15:04:05"))
				}
			}
		}
	}()
}
