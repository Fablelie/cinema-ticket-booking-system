package seat

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// SeatStatus กำหนดสถานะของที่นั่งตามที่โจทย์ระบุ
type SeatStatus string

const (
	StatusAvailable SeatStatus = "AVAILABLE"
	StatusLocked    SeatStatus = "LOCKED"
	StatusBooked    SeatStatus = "BOOKED"
)

// Showtime เก็บข้อมูลรอบฉายของภาพยนตร์แต่ละเรื่อง
type Showtime struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	MovieTitle string             `bson:"movie_title" json:"movie_title"`
	StartTime  time.Time          `bson:"start_time" json:"start_time"`
	TheaterNo  int                `bson:"theater_no" json:"theater_no"`
	Seats      []Seat             `bson:"seats" json:"seats"`
}

// Seat เก็บรายละเอียดสถานะและตำแหน่งของที่นั่งตัวนั้นๆ
type Seat struct {
	SeatNo      string     `bson:"seat_no" json:"seat_no"`                         // เช่น A1, A2, B1
	Status      SeatStatus `bson:"status" json:"status"`                           // AVAILABLE, LOCKED, BOOKED
	LockedBy    string     `bson:"locked_by,omitempty" json:"locked_by,omitempty"` // User ID ที่กำลังล็อก
	LockedUntil *time.Time `bson:"locked_until,omitempty" json:"locked_until,omitempty"`
}

// UpdateSeatStatusReq สำหรับรับค่าจากหน้าบ้านเวลาเลือกที่นั่ง
type UpdateSeatStatusReq struct {
	ShowtimeID string `json:"showtime_id" binding:"required"`
	SeatNo     string `json:"seat_no" binding:"required"`
}
