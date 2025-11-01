package booking

import (
	"context"
	"time"

	"github.com/RathaTart/FoodBridge/entities"
)

type Filter struct {
	PostID         *int64
	ReceiverUserID *int64
	Status         *entities.BookingStatus
	Statuses       []string
}

type Repo interface {
	WithTx(ctx context.Context, fn func(r Repo) error) error
	TryReserveStock(ctx context.Context, postID int64) (bool, error)

	// Stock / post
	DecrementQty(ctx context.Context, postID int64, n int) error
	IncrementQty(ctx context.Context, postID int64, n int) error

	// Booking rows
	CreateBooking(ctx context.Context, b *entities.Booking) error
	UpdateBooking(ctx context.Context, b *entities.Booking) error
	GetBookingByID(ctx context.Context, id int64, forUpdate bool) (*entities.Booking, error)
	ListBookings(ctx context.Context, f Filter) ([]entities.Booking, error)
	CountReceivers(ctx context.Context, f Filter) (int64, error)

	// QR code
	GetBookingByQRToken(ctx context.Context, token string, forUpdate bool) (*entities.Booking, error)
	SetBookingQRToken(ctx context.Context, id int64, token string) error

	// Queue + daily limit
	NextQueuePos(ctx context.Context, postID int64) (int, error)
	FindNextQueued(ctx context.Context, postID int64) (*entities.Booking, error)
	CountActiveTodayByUser(ctx context.Context, userID int64, dayStart, dayEnd time.Time) (int64, error)

	GetPostOwnerID(ctx context.Context, postID int64) (int64, error)

	// Expiration sweep helpers
	// Return a batch of booking IDs that are PENDING and expired at or before the given time.
	ListExpiredPendingIDs(ctx context.Context, before time.Time, limit int) ([]int64, error)
}
