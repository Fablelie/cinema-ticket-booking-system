package booking

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/fablelie/cinema-ticket-booking-system/config"
	"github.com/fablelie/cinema-ticket-booking-system/internal/seat"
	"github.com/rabbitmq/amqp091-go"
)

type Service struct {
	bookingRepo *Repository
	seatRepo    *seat.Repository
	mqCh        *amqp091.Channel
	cfg         *config.Config
	mu          sync.Mutex
}

func NewService(bookingRepo *Repository, seatRepo *seat.Repository, ch *amqp091.Channel, cfg *config.Config) *Service {
	return &Service{
		bookingRepo: bookingRepo,
		seatRepo:    seatRepo,
		mqCh:        ch,
		cfg:         cfg,
	}
}

// ReserveSeats locks all requested seats for the same user, or rolls back.
func (s *Service) ReserveSeats(ctx context.Context, userID string, req ReserveSeatsReq) (*BookingResponse, error) {
	// Use configurable TTL from environment (default 5 minutes)
	ttl := s.cfg.SeatLockTTL
	lockedUntil := time.Now().Add(ttl)

	log.Printf("[BOOKING SERVICE] Reserving seats with TTL=%v (SEAT_LOCK_TTL_SECONDS=%d) for user=%s seats=%v",
		ttl, int64(ttl.Seconds()), userID, req.Seats)

	success, err := s.bookingRepo.AcquireMultipleLocks(ctx, req.ShowtimeID, req.Seats, userID, ttl)
	if err != nil {
		eventPayload, err := json.Marshal(map[string]interface{}{
			"event":       "SYSTEM_ERROR",
			"showtime_id": req.ShowtimeID,
			"seats":       req.Seats,
			"user_id":     userID,
			"error_msg":   "Redis connection failure: " + err.Error(),
		})

		s.mu.Lock()
		if err := s.mqCh.PublishWithContext(context.Background(),
			"",
			"booking_events",
			false, false,
			amqp091.Publishing{
				ContentType: "application/json",
				Body:        eventPayload,
			},
		); err != nil {
			log.Printf("[RabbitMQ] Failed to publish SYSTEM_ERROR event: %v", err)
		} else {
			log.Printf("[RabbitMQ] Successfully published SYSTEM_ERROR event")
		}
		s.mu.Unlock()

		return nil, err
	}
	if !success {
		eventPayload, err := json.Marshal(map[string]interface{}{
			"event":       "SYSTEM_LOCK_FAIL",
			"showtime_id": req.ShowtimeID,
			"seats":       req.Seats,
			"user_id":     userID,
			"error_msg":   "Concurrency conflict: One or more seats are already locked by another user",
		})

		if err != nil {
			log.Printf("[BOOKING SERVICE] SYSTEM_LOCK_FAIL json marshal err: %v", err)
		}

		s.mu.Lock()
		if err := s.mqCh.PublishWithContext(context.Background(),
			"",
			"booking_events",
			false, false,
			amqp091.Publishing{
				ContentType: "application/json",
				Body:        eventPayload,
			},
		); err != nil {
			log.Printf("[RabbitMQ] Failed to publish SYSTEM_LOCK_FAIL event: %v", err)
		} else {
			log.Printf("[RabbitMQ] Successfully published SYSTEM_LOCK_FAIL event")
		}
		s.mu.Unlock()

		return nil, errors.New("one or more selected seats are already locked or booked")
	}

	lockedSeats := make([]string, 0, len(req.Seats))
	for _, seatNo := range req.Seats {
		err := s.seatRepo.LockSeat(ctx, req.ShowtimeID, seatNo, userID, lockedUntil)
		if err != nil {
			for _, rollbackSeatNo := range lockedSeats {
				_ = s.seatRepo.UpdateSeatStatus(ctx, req.ShowtimeID, rollbackSeatNo, seat.StatusAvailable, "")
			}
			_ = s.bookingRepo.ReleaseMultipleLocks(ctx, req.ShowtimeID, req.Seats, userID)
			eventPayload, _ := json.Marshal(map[string]interface{}{
				"event":       "SYSTEM_LOCK_FAIL",
				"showtime_id": req.ShowtimeID,
				"seats":       req.Seats,
				"user_id":     userID,
				"error_msg":   "Database conflict or incomplete booking: " + err.Error(),
			})

			s.mu.Lock()
			_ = s.mqCh.PublishWithContext(context.Background(), "", "booking_events", false, false,
				amqp091.Publishing{ContentType: "application/json", Body: eventPayload})
			s.mu.Unlock()
			if errors.Is(err, seat.ErrSeatStateConflict) {
				return nil, errors.New("one or more selected seats are no longer available")
			}
			return nil, err
		}
		lockedSeats = append(lockedSeats, seatNo)
	}

	eventPayload, err := json.Marshal(map[string]interface{}{
		"event":        "SEATS_LOCKED",
		"showtime_id":  req.ShowtimeID,
		"seats":        req.Seats,
		"user_id":      userID,
		"locked_until": lockedUntil,
	})
	if err != nil {
		log.Printf("[BOOKING SERVICE] SEATS_LOCKED json marshal err: %v", err)
	}

	log.Printf("[BOOKING SERVICE] Publishing SEATS_LOCKED event for user %s, seats: %v", userID, req.Seats)
	s.mu.Lock()
	if err := s.mqCh.PublishWithContext(context.Background(),
		"",
		"booking_events",
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
	s.mu.Unlock()

	return &BookingResponse{
		Seats:       req.Seats,
		ShowtimeID:  req.ShowtimeID,
		LockedUntil: lockedUntil,
		Status:      "LOCKED",
	}, nil
}

// CancelReservation releases seats locked by the same user.
// 1. Releases Redis locks immediately for responsiveness
// 2. Releases MongoDB seat locks with proper error handling
// 3. Publishes event to notify frontend/workers
func (s *Service) CancelReservation(ctx context.Context, userID string, req ReserveSeatsReq) error {
	// Step 1: Release Redis locks (timeout mechanisms)
	released, err := s.bookingRepo.ReleaseMultipleLocksWithCount(context.Background(), req.ShowtimeID, req.Seats, userID)
	if err != nil {
		log.Printf("[BOOKING SERVICE] Error releasing Redis locks for user=%s: %v", userID, err)
		return err
	}
	if released != int64(len(req.Seats)) {
		log.Printf("[BOOKING SERVICE] WARNING: Released %d/%d Redis locks. user=%s seats=%v (some locks may have already expired)",
			released, len(req.Seats), userID, req.Seats)
	} else {
		log.Printf("[BOOKING SERVICE] Successfully released %d Redis locks for user=%s seats=%v",
			len(req.Seats), userID, req.Seats)
	}

	// Step 2: Release MongoDB seat locks
	err = s.seatRepo.ReleaseLockedSeats(ctx, req.ShowtimeID, req.Seats, userID)
	if err != nil {
		if errors.Is(err, seat.ErrSeatStateConflict) {
			log.Printf("[BOOKING SERVICE] ERROR: Seats not in expected state for user=%s. seats=%v. This may indicate a race condition or timeout.",
				userID, req.Seats)
			return errors.New("one or more selected seats are not locked by this user or have already been modified")
		}
		log.Printf("[BOOKING SERVICE] ERROR releasing MongoDB locks for user=%s: %v", userID, err)
		return err
	}
	log.Printf("[BOOKING SERVICE] Successfully released %d MongoDB seat locks for user=%s seats=%v",
		len(req.Seats), userID, req.Seats)

	// Step 3: Publish cancellation event for frontend/workers
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"event":       "SEATS_RELEASED",
		"showtime_id": req.ShowtimeID,
		"seats":       req.Seats,
		"user_id":     userID,
		"timestamp":   time.Now(),
	})

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
	} else {
		log.Printf("[RabbitMQ] Successfully published SEATS_RELEASED event for user=%s seats=%v", userID, req.Seats)
	}

	return nil
}

// ConfirmReservation books seats only if they are still locked by this user.
// 1. Confirms seats are locked by this user in MongoDB
// 2. Releases Redis timeout locks
// 3. Publishes booking confirmation event
func (s *Service) ConfirmReservation(ctx context.Context, userID string, req ReserveSeatsReq) error {
	// Step 1: Confirm seats are locked by this user and transition to BOOKED
	err := s.seatRepo.ConfirmLockedSeats(ctx, req.ShowtimeID, req.Seats, userID)
	if err != nil {
		if errors.Is(err, seat.ErrSeatStateConflict) {
			log.Printf("[BOOKING SERVICE] ERROR: Seats not in expected LOCKED state for user=%s seats=%v", userID, req.Seats)
			return errors.New("one or more selected seats are not locked by this user or have already been booked")
		}
		log.Printf("[BOOKING SERVICE] ERROR confirming seats for user=%s: %v", userID, err)
		return err
	}
	log.Printf("[BOOKING SERVICE] Successfully confirmed %d seats to BOOKED status for user=%s seats=%v",
		len(req.Seats), userID, req.Seats)

	// Step 2: Release Redis locks (no longer need timeout mechanism)
	released, err := s.bookingRepo.ReleaseMultipleLocksWithCount(ctx, req.ShowtimeID, req.Seats, userID)
	if err != nil {
		log.Printf("[BOOKING SERVICE] WARNING: Error releasing Redis locks after booking confirm for user=%s: %v", userID, err)
	} else if released > 0 {
		log.Printf("[BOOKING SERVICE] Released %d Redis timeout locks after confirmation for user=%s", released, userID)
	}

	// Step 3: Publish booking confirmation event
	eventPayload, _ := json.Marshal(map[string]interface{}{
		"event":       "BOOKING_SUCCESS",
		"showtime_id": req.ShowtimeID,
		"seats":       req.Seats,
		"user_id":     userID,
		"timestamp":   time.Now(),
	})

	if err := s.mqCh.PublishWithContext(context.Background(),
		"", "booking_events", false, false,
		amqp091.Publishing{
			ContentType: "application/json",
			Body:        eventPayload,
		},
	); err != nil {
		log.Printf("[RabbitMQ] Failed to publish BOOKING_SUCCESS event: %v", err)
	} else {
		log.Printf("[RabbitMQ] Successfully published BOOKING_SUCCESS event for user=%s seats=%v", userID, req.Seats)
	}

	return nil
}
