package seat

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrSeatStateConflict = errors.New("seat state conflict")

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
	if status == StatusAvailable {
		update["$unset"] = bson.M{"seats.$.locked_until": ""}
	}

	_, err = r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *Repository) LockSeat(ctx context.Context, showtimeID string, seatNo string, userID string, lockedUntil time.Time) error {
	objID, err := primitive.ObjectIDFromHex(showtimeID)
	if err != nil {
		return err
	}

	filter := bson.M{
		"_id": objID,
		"seats": bson.M{"$elemMatch": bson.M{
			"seat_no": seatNo,
			"status":  StatusAvailable,
		}},
	}
	update := bson.M{
		"$set": bson.M{
			"seats.$.status":       StatusLocked,
			"seats.$.locked_by":    userID,
			"seats.$.locked_until": lockedUntil,
		},
	}

	res, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrSeatStateConflict
	}
	return nil
}

func (r *Repository) ConfirmLockedSeats(ctx context.Context, showtimeID string, seatNos []string, userID string) error {
	objID, err := primitive.ObjectIDFromHex(showtimeID)
	if err != nil {
		return err
	}

	// Simple filter: just check for the showtime, let array filter handle seat matching
	filter := bson.M{"_id": objID}
	update := bson.M{
		"$set": bson.M{
			"seats.$[seat].status":    StatusBooked,
			"seats.$[seat].locked_by": userID,
		},
		"$unset": bson.M{
			"seats.$[seat].locked_until": "",
		},
	}
	opts := options.Update().SetArrayFilters(options.ArrayFilters{
		Filters: []interface{}{bson.M{
			"seat.seat_no":   bson.M{"$in": seatNos},
			"seat.status":    StatusLocked,
			"seat.locked_by": userID,
		}},
	})

	res, err := r.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return err
	}
	// Check if any seats were actually modified (not just matched)
	if res.ModifiedCount == 0 {
		return ErrSeatStateConflict
	}
	return nil
}

func (r *Repository) ReleaseLockedSeats(ctx context.Context, showtimeID string, seatNos []string, userID string) error {
	objID, err := primitive.ObjectIDFromHex(showtimeID)
	if err != nil {
		return err
	}

	// Simple filter: just check for the showtime, let array filter handle seat matching
	filter := bson.M{"_id": objID}
	update := bson.M{
		"$set": bson.M{
			"seats.$[seat].status":    StatusAvailable,
			"seats.$[seat].locked_by": "",
		},
		"$unset": bson.M{
			"seats.$[seat].locked_until": "",
		},
	}
	opts := options.Update().SetArrayFilters(options.ArrayFilters{
		Filters: []interface{}{bson.M{
			"seat.seat_no":   bson.M{"$in": seatNos},
			"seat.status":    StatusLocked,
			"seat.locked_by": userID,
		}},
	})

	res, err := r.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return err
	}
	// Check if any seats were actually modified (not just matched)
	if res.ModifiedCount == 0 {
		return ErrSeatStateConflict
	}
	return nil
}

// DEPRECATED: seatsOwnedByUserFilter has been replaced with simpler per-seat array filtering
// to avoid strict filter requirements that cause cancellations to fail.
// The array filter in UpdateOne now handles all seat matching logic.
func seatsOwnedByUserFilter(showtimeID primitive.ObjectID, seatNos []string, status SeatStatus, userID string) bson.M {
	conditions := make([]bson.M, 0, len(seatNos))
	for _, seatNo := range seatNos {
		conditions = append(conditions, bson.M{
			"seats": bson.M{"$elemMatch": bson.M{
				"seat_no":   seatNo,
				"status":    status,
				"locked_by": userID,
			}},
		})
	}

	return bson.M{
		"_id":  showtimeID,
		"$and": conditions,
	}
}
