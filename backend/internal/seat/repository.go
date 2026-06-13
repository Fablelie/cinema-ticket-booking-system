package seat

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Repository struct {
	collection *mongo.Collection
}

func NewRepository(db *mongo.Database) *Repository {
	return &Repository{
		collection: db.Collection("showtimes"),
	}
}

// GetShowtimeByID ดึงข้อมูลรอบฉายและผังที่นั่งทั้งหมดจาก MongoDB
func (r *Repository) GetShowtimeByID(ctx context.Context, id string) (*Showtime, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var showtime Showtime
	err = r.collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&showtime)
	if err != nil {
		return nil, err
	}

	return &showtime, nil
}

// UpdateSeatStatus อัปเดตสถานะที่นั่งในฐานข้อมูลหลัก (MongoDB)
func (r *Repository) UpdateSeatStatus(ctx context.Context, showtimeID string, seatNo string, status SeatStatus, userID string) error {
	objID, err := primitive.ObjectIDFromHex(showtimeID)
	if err != nil {
		return err
	}

	// ค้นหารอบฉายและเก้าอี้ตัวที่ระบุ จากนั้นเปลี่ยนสถานะ
	filter := bson.M{"_id": objID, "seats.seat_no": seatNo}
	update := bson.M{
		"$set": bson.M{
			"seats.$.status":    status,
			"seats.$.locked_by": userID,
		},
	}

	_, err = r.collection.UpdateOne(ctx, filter, update)
	return err
}
