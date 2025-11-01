package booking

import (
	"context"
	"time"

	"github.com/RathaTart/FoodBridge/entities"
)

type Config struct {
	DailyLimit int           // default 1
	HoldTTL    time.Duration // default 10 * time.Minute
	DayTZ      *time.Location
	QRSecret   []byte
	QRTokenTTL time.Duration // default 10 * time.Minute
}

type Service interface {
	Create(ctx context.Context, postID int64, receiverUserID int64) (*entities.Booking, error)
	Get(ctx context.Context, id int64) (*entities.Booking, error)
	List(ctx context.Context, f Filter) ([]entities.Booking, error)
	CountReceivers(ctx context.Context, f Filter) (int64, error)
	Cancel(ctx context.Context, id int64) error
	Complete(ctx context.Context, id int64) error

	IssueQR(ctx context.Context, id int64, ttl time.Duration) (string, error)
	ScanQR(ctx context.Context, token string) (*entities.Booking, error)

	// ExpireSweep scans for expired pending bookings and transitions them to EXPIRED,
	// promoting queued bookings or returning stock as appropriate. It processes up to 'max' items per call.
	ExpireSweep(ctx context.Context, max int) error

	DailyLimitLeft(ctx context.Context, userID int64, now time.Time) (limit int, used int, left int, start time.Time, end time.Time, err error)
}
