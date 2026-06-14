package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/fablelie/cinema-ticket-booking-system/config"
	"github.com/fablelie/cinema-ticket-booking-system/internal/auth"
	"github.com/fablelie/cinema-ticket-booking-system/internal/booking"
	"github.com/fablelie/cinema-ticket-booking-system/internal/seat"
	"github.com/fablelie/cinema-ticket-booking-system/internal/worker"

	"github.com/gin-gonic/gin"
	"github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// 1. โหลดคอนฟิกจาก Environment Variables (ดึงค่าจาก .env ผ่าน Docker Compose)
	cfg := config.LoadConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 2. เชื่อมต่อฐานข้อมูลหลัก MongoDB
	log.Println("Connecting to MongoDB...")
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoURI))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(context.Background())

	if err := mongoClient.Ping(ctx, nil); err != nil {
		log.Fatalf("MongoDB ping failed: %v", err)
	}
	mongoDB := mongoClient.Database("cinema")
	log.Println("MongoDB connected successfully!")

	// 🎯 เรียกใช้ Seeding Script เพื่อเตรียมเก้าอี้ 2 แถว แถวละ 5 ตัว (A1-B5) พร้อมเปิดระบบ Index
	seat.SeedDefaultShowtime(mongoDB, cfg.DefaultShowtimeID)

	// 3. เชื่อมต่อฐานข้อมูล Cache/Lock Redis
	log.Println("Connecting to Redis...")
	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisURI,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Redis connected successfully!")

	// 4. เชื่อมต่อระบบ Message Queue RabbitMQ (พร้อมระบบ Retry 5 ครั้งตอนเปิดเครื่อง)
	var mqConn *amqp091.Connection
	log.Println("Connecting to RabbitMQ...")
	for i := 1; i <= 5; i++ {
		mqConn, err = amqp091.Dial(cfg.RabbitMQURI)
		if err == nil {
			break
		}
		log.Printf("RabbitMQ not ready, retrying in 3 seconds... (%d/5)", i)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ after retries: %v", err)
	}
	defer mqConn.Close()

	mqPublishCh, err := mqConn.Channel()
	if err != nil {
		log.Fatalf("Failed to open RabbitMQ publish channel: %v", err)
	}
	defer mqPublishCh.Close()

	mqConsumeCh, err := mqConn.Channel()
	if err != nil {
		log.Fatalf("Failed to open RabbitMQ consume channel: %v", err)
	}
	defer mqConsumeCh.Close()

	// สร้างคิวหลักของระบบจองเพื่อใช้เก็บประวัติและสื่อสารข้อมูลเรียลไทม์
	_, err = mqPublishCh.QueueDeclare(
		"booking_events", // ชื่อคิว
		true,             // durable
		false,            // delete when unused
		false,            // exclusive
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		log.Fatalf("Failed to declare RabbitMQ queue: %v", err)
	}
	log.Println("RabbitMQ connected and queue declared successfully!")

	// 5. เปิดเครื่องระบบจัดการเรียลไทม์ WebSocket Hub ส่วนกลาง
	wsHub := seat.NewHub()
	go wsHub.Run(context.Background()) // แยกการกระจายข้อความไปรันเบื้องหลัง (Goroutine)

	// 6. ทำ Dependency Injection สำหรับกลุ่มธุรกิจจัดการข้อมูลที่นั่ง (Seat Domain)
	seatRepo := seat.NewRepository(mongoDB)
	seatService := seat.NewService(seatRepo)
	seatHandler := seat.NewHandler(seatService, wsHub)

	// 7. ทำ Dependency Injection สำหรับกลุ่มธุรกิจจัดการระบบจองและล็อกที่นั่ง (Booking Domain)
	bookingRepo := booking.NewRepository(mongoDB, rdb)
	bookingService := booking.NewService(bookingRepo, seatRepo, mqPublishCh, cfg) // Inject config for seat lock TTL
	bookingHandler := booking.NewHandler(bookingService)

	// 8. เริ่มต้นสวิตช์การทำงานของกลุ่มสคริปต์เบื้องหลัง (Background Workers)

	// ตัวที่ 1: ดักฟังคีย์ล็อกใน Redis หมดอายุ เพื่อเคลียร์สถานะกลับเป็น AVAILABLE ใน MongoDB
	timeoutWorker := worker.NewTimeoutListener(rdb, seatRepo, mqPublishCh)
	timeoutWorker.Start(context.Background())

	// ตัวที่ 2: ดึงประวัติกิจกรรมจากคิว RabbitMQ มาบันทึกลงฐานข้อมูลเป็น Audit Logs ของแอดมิน
	rabbitConsumer := worker.NewRabbitConsumer(mqConsumeCh, mongoDB, wsHub)
	rabbitConsumer.Start(context.Background())

	log.Println("All Background workers initialized successfully!")

	// 9. เปิดใช้งาน HTTP REST API Server ด้วย Gin เฟรมเวิร์ก
	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-User-ID")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// พอร์ตสำหรับให้ผู้ตรวจใช้เช็กสถานะการเชื่อมต่อภาพรวม
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":   "UP",
			"database": "CONNECTED",
			"cache":    "CONNECTED",
			"queue":    "CONNECTED",
		})
	})

	// ลงทะเบียนเส้นทางเดินข้อมูลระบบ (Routing Group) เวอร์ชัน 1
	apiV1 := r.Group("/api/v1")
	{
		// ช่องทางอัปเกรดเรียลไทม์: ws://localhost:8080/api/v1/seats/ws
		apiV1.GET("/seats/ws", seatHandler.WebSocketHandler)

		// เส้นทาง REST API ปกติสำหรับดึงข้อมูลและทำรายการจองตั๋วหนัง
		apiV1.GET("/seats/:showtime_id", seatHandler.GetSeatMapHandler)

		securedBooking := apiV1.Group("/bookings")
		securedBooking.Use(auth.GoogleAuthMiddleware(cfg))
		{
			securedBooking.GET("/me", auth.GetProfileHandler(cfg))

			securedBooking.POST("/reserve", bookingHandler.ReserveHandler)
			securedBooking.POST("/confirm", bookingHandler.ConfirmHandler)
			securedBooking.POST("/cancel", bookingHandler.CancelHandler)
		}

		adminRoutes := apiV1.Group("/admin")
		adminRoutes.Use(auth.GoogleAuthMiddleware(cfg), auth.AdminOnlyMiddleware(cfg))
		{
			adminRoutes.GET("/dashboard", bookingHandler.GetAdminDashboardDataHandler)
			adminRoutes.POST("/reset-seats", bookingHandler.ResetSeatsHandler)
		}
	}

	log.Printf("Server is starting on port %s...", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
