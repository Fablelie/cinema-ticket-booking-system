package seat

import (
	"context"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetSeatMap(ctx context.Context, showtimeID string) (*Showtime, error) {
	return s.repo.GetShowtimeByID(ctx, showtimeID)
}

func (s *Service) UpdateSeat(ctx context.Context, showtimeID string, seatNo string, status SeatStatus, userID string) error {
	return s.repo.UpdateSeatStatus(ctx, showtimeID, seatNo, status, userID)
}
