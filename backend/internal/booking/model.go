package booking

import (
	"time"
)

type BookingStatus string

const (
	StatusPending  BookingStatus = "PENDING"
	StatusBooked   BookingStatus = "BOOKED"
	StatusCanceled BookingStatus = "CANCELED"
)

// BookingResponse ส่งกลับไปให้หน้าบ้าน เพื่อเอาไปแสดงสถานะและนับเวลาถอยหลัง
type BookingResponse struct {
	Seats       []string  `json:"seats"` // เช่น ["A1", "A2"]
	ShowtimeID  string    `json:"showtime_id"`
	LockedUntil time.Time `json:"locked_until"` // ส่ง Timestamp: 2026-06-13T15:05:00Z
	Status      string    `json:"status"`
}

// ReserveSeatsReq รับข้อมูลรายการเก้าอี้ที่กดมาจากหน้าบ้าน
type ReserveSeatsReq struct {
	ShowtimeID string   `json:"showtime_id" binding:"required"`
	Seats      []string `json:"seats" binding:"required,gt=0"` // ต้องเลือกอย่างน้อย 1 ตัว
}

type AdminResetReq struct {
	ShowtimeID string `json:"showtime_id" binding:"required"`
}
