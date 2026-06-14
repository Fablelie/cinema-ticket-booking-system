package seat

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SeedDefaultShowtime ทำหน้าที่สร้างผังเก้าอี้เริ่มต้น และทำ Indexing เพื่อประสิทธิภาพสูงสุด
func SeedDefaultShowtime(db *mongo.Database, defaultID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := db.Collection("showtimes")

	// 1. สร้าง Unique Index
	_, _ = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})

	// 2. แปลงรหัส ID จาก .env ให้เป็น ObjectID ของ MongoDB
	objID, err := primitive.ObjectIDFromHex(defaultID)
	if err != nil {
		log.Printf("[SEEDER ERROR] Invalid DEFAULT_SHOWTIME_ID format: %v", err)
		return
	}

	// 3. เช็กก่อนว่ามีข้อมูลไอดีนี้อยู่ในระบบแล้วหรือยัง
	var existing bson.M
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&existing)
	if err == nil {
		log.Println("[SEEDER] Default seat map already exists. Skipping seed data.")
		return
	}

	// 4. วงลูปสร้างรายชื่อเก้าอี้ 2 แถว แถวละ 5 ตัว A1 - B5
	var seats []Seat
	rows := []string{"A", "B"}

	for _, row := range rows {
		for i := 1; i <= 5; i++ {
			seatNo := fmt.Sprintf("%s%d", row, i)
			seats = append(seats, Seat{
				SeatNo: seatNo,
				Status: StatusAvailable,
			})
		}
	}

	// 5. ประกอบร่างข้อมูล
	defaultShowtime := bson.M{
		"_id":         objID,
		"movie_title": "Take-Home Assignment Cinema",
		"theater_no":  1,
		"seats":       seats,
		"created_at":  time.Now(),
	}

	// 6. บันทึกลงฐานข้อมูล MongoDB
	_, err = collection.InsertOne(ctx, defaultShowtime)
	if err != nil {
		log.Printf("[SEEDER ERROR] Failed to insert default seat map: %v", err)
		return
	}

	log.Printf("[SEEDER SUCCESS] Created default seat map (ID: %s) with 10 AVAILABLE seats and Database Index!", defaultID)
}
