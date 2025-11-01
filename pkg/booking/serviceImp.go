package booking

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/RathaTart/FoodBridge/entities"

	// notif publisher (DI)
	notif "github.com/RathaTart/FoodBridge/pkg/notification"
)

type service struct {
	repo Repo
	cfg  Config
	pub  *notif.Publisher // optional; if nil -> skip notifications
}

func NewService(repo Repo, cfg Config, pub *notif.Publisher) Service {
	if cfg.DailyLimit == 0 {
		cfg.DailyLimit = 2
	}
	if cfg.HoldTTL == 0 {
		cfg.HoldTTL = 10 * time.Minute
	}
	if cfg.DayTZ == nil {
		loc, _ := time.LoadLocation("Asia/Bangkok")
		cfg.DayTZ = loc
	}
	if cfg.QRTokenTTL == 0 {
		cfg.QRTokenTTL = 10 * time.Minute
	}
	return &service{repo: repo, cfg: cfg, pub: pub}
}

func (s *service) dayBounds(t time.Time) (time.Time, time.Time) {
	loc := s.cfg.DayTZ
	tt := t.In(loc)
	start := time.Date(tt.Year(), tt.Month(), tt.Day(), 0, 0, 0, 0, loc)
	end := start.Add(24 * time.Hour)
	return start, end
}

type pendingNotif struct {
	userID int64
	typ    string
	title  string
	body   string
	data   any
}

func (s *service) Create(ctx context.Context, postID int64, receiverUserID int64) (*entities.Booking, error) {
	now := time.Now()
	dayStart, dayEnd := s.dayBounds(now)

	var out *entities.Booking
	var toSend []pendingNotif

	err := s.repo.WithTx(ctx, func(r Repo) error {
		// (A) Owner self-book guard
		ownerID, err := r.GetPostOwnerID(ctx, postID)
		if err != nil {
			return err
		}
		if ownerID == receiverUserID {
			return fmt.Errorf("cannot_book_own_post")
		}

		// (B) Daily limit
		cnt, err := r.CountActiveTodayByUser(ctx, receiverUserID, dayStart, dayEnd)
		if err != nil {
			return err
		}
		if cnt >= int64(s.cfg.DailyLimit) {
			return fmt.Errorf("no_booking_token_left")
		}

		// (C) Try to reserve stock (safe)
		reserved, err := r.TryReserveStock(ctx, postID)
		if err != nil {
			return err
		}

		if reserved {
			exp := now.Add(s.cfg.HoldTTL)
			b := &entities.Booking{
				PostID:         postID,
				ReceiverUserID: receiverUserID,
				Status:         entities.BookingPending,
				ExpireAt:       &exp,
			}
			if err := r.CreateBooking(ctx, b); err != nil {
				return err
			}
			out = b

			// schedule notifications (after commit)
			toSend = append(toSend,
				pendingNotif{
					userID: ownerID,
					typ:    "booking.pending",
					title:  "new booking request",
					body:   "you have a new booking request",
					data:   map[string]any{"booking_id": b.BookingID, "post_id": postID, "receiver_id": receiverUserID},
				},
				pendingNotif{
					userID: receiverUserID,
					typ:    "booking.created.pending",
					title:  "booking created",
					body:   "See you soon! Please pick up within hold time.",
					data:   map[string]any{"booking_id": b.BookingID, "post_id": postID},
				},
			)
			return nil
		}

		// No stock → enqueue
		pos, err := r.NextQueuePos(ctx, postID)
		if err != nil {
			return err
		}
		b := &entities.Booking{
			PostID:         postID,
			ReceiverUserID: receiverUserID,
			Status:         entities.BookingQueued,
			QueuePos:       &pos,
		}
		if err := r.CreateBooking(ctx, b); err != nil {
			return err
		}
		out = b

		// schedule notify receiver about queue
		toSend = append(toSend, pendingNotif{
			userID: receiverUserID,
			typ:    "booking.queued",
			title:  "Booking Queued",
			body:   fmt.Sprintf("Your queue position is #%d", pos),
			data:   map[string]any{"booking_id": b.BookingID, "post_id": postID, "queue_pos": pos},
		})
		return nil
	})

	// publish after commit
	if err == nil && s.pub != nil && len(toSend) > 0 {
		for _, n := range toSend {
			_ = s.pub.Notify(ctx, n.userID, n.typ, n.title, n.body, n.data)
		}
	}
	return out, err
}

func (s *service) Get(ctx context.Context, id int64) (*entities.Booking, error) {
	return s.repo.GetBookingByID(ctx, id, false)
}

func (s *service) List(ctx context.Context, f Filter) ([]entities.Booking, error) {
	return s.repo.ListBookings(ctx, f)
}

func (s *service) CountReceivers(ctx context.Context, f Filter) (int64, error) {
	return s.repo.CountReceivers(ctx, f)
}

func (s *service) Cancel(ctx context.Context, id int64) error {
	now := time.Now()
	var toSend []pendingNotif

	err := s.repo.WithTx(ctx, func(r Repo) error {
		// Lock the booking row
		b, err := r.GetBookingByID(ctx, id, true)
		if err != nil {
			return err
		}

		switch b.Status {
		case entities.BookingPending:
			// Try to promote the next queued for the same post
			next, err := r.FindNextQueued(ctx, b.PostID)
			if err == nil && next != nil {
				// Promote queued → pending (no net stock change)
				exp := now.Add(s.cfg.HoldTTL)
				next.Status = entities.BookingPending
				next.ExpireAt = &exp
				next.UpdatedAt = now
				if err := r.UpdateBooking(ctx, next); err != nil {
					return err
				}
				// schedule notify promoted receiver
				toSend = append(toSend, pendingNotif{
					userID: next.ReceiverUserID,
					typ:    "queue.promoted",
					title:  "Your queue position is up",
					body:   "Your booking request has been promoted. See you at the pickup point soon!",
					data:   map[string]any{"booking_id": next.BookingID, "post_id": b.PostID, "new_status": "PENDING"},
				})
			} else {
				// No one queued → return stock (+1)
				if err2 := r.IncrementQty(ctx, b.PostID, 1); err2 != nil {
					return err2
				}
				// ignore not-found; bubble other errors
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			}

			// mark current booking as CANCELLED
			b.Status = entities.BookingCancelled
			b.UpdatedAt = now
			if err := r.UpdateBooking(ctx, b); err != nil {
				return err
			}

			// notify provider that pending booking was cancelled by user
			ownerID, e := r.GetPostOwnerID(ctx, b.PostID)
			if e == nil {
				toSend = append(toSend, pendingNotif{
					userID: ownerID,
					typ:    "booking.cancelled.by_user",
					title:  "Booking Cancelled",
					body:   "Your pending booking request has been cancelled.",
					data:   map[string]any{"booking_id": b.BookingID, "post_id": b.PostID, "receiver_id": b.ReceiverUserID},
				})
			}
			return nil

		case entities.BookingQueued:
			// Simply drop it from the queue (no stock change)
			b.Status = entities.BookingCancelled
			b.UpdatedAt = now
			return r.UpdateBooking(ctx, b)

		default:
			// Other states: allow idempotent cancel only if already cancelled
			if b.Status == entities.BookingCancelled {
				return nil
			}
			// Completed/Expired cannot be cancelled
			return fmt.Errorf("invalid_status_transition")
		}
	})

	if err == nil && s.pub != nil && len(toSend) > 0 {
		for _, n := range toSend {
			_ = s.pub.Notify(ctx, n.userID, n.typ, n.title, n.body, n.data)
		}
	}
	return err
}

func (s *service) Complete(ctx context.Context, id int64) error {
	now := time.Now()
	var toSend []pendingNotif

	err := s.repo.WithTx(ctx, func(r Repo) error {
		b, err := r.GetBookingByID(ctx, id, true)
		if err != nil {
			return err
		}

		if b.Status == entities.BookingExpired || b.Status == entities.BookingCancelled {
			return fmt.Errorf("invalid_status_transition")
		}
		if b.Status == entities.BookingPending && b.ExpireAt != nil && now.After(*b.ExpireAt) {
			// treat as expired
			b.Status = entities.BookingExpired
			b.ExpiredAt = &now
			b.UpdatedAt = now
			if err := r.UpdateBooking(ctx, b); err != nil {
				return err
			}

			// ✅ notify receiver that it expired
			toSend = append(toSend, pendingNotif{
				userID: b.ReceiverUserID,
				typ:    "booking.expired",
				title:  "Booking Expired",
				body:   "Your booking request has expired.",
				data:   map[string]any{"booking_id": b.BookingID, "post_id": b.PostID},
			})

			next, e2 := r.FindNextQueued(ctx, b.PostID)
			if e2 == nil && next != nil {
				exp := now.Add(s.cfg.HoldTTL)
				next.Status = entities.BookingPending
				next.ExpireAt = &exp
				next.UpdatedAt = now
				if err := r.UpdateBooking(ctx, next); err != nil {
					return err
				}

				// notify promoted receiver
				toSend = append(toSend, pendingNotif{
					userID: next.ReceiverUserID,
					typ:    "queue.promoted",
					title:  "Your queue position is up",
					body:   "Your booking request has been promoted. See you at the pickup point soon!",
					data:   map[string]any{"booking_id": next.BookingID, "post_id": b.PostID, "new_status": "PENDING"},
				})
				return nil
			}
			if errors.Is(e2, gorm.ErrRecordNotFound) {
				return r.IncrementQty(ctx, b.PostID, 1)
			}
			return e2
		}

		// normal complete
		b.Status = entities.BookingCompleted
		b.UpdatedAt = now
		if err := r.UpdateBooking(ctx, b); err != nil {
			return err
		}

		// notify receiver + provider
		ownerID, e := r.GetPostOwnerID(ctx, b.PostID)
		if e == nil {
			toSend = append(toSend,
				pendingNotif{
					userID: b.ReceiverUserID,
					typ:    "booking.completed.receiver",
					title:  "booking completed",
					body:   "Your booking has been completed. Enjoy your meal!",
					data:   map[string]any{"booking_id": b.BookingID, "post_id": b.PostID},
				},
				pendingNotif{
					userID: ownerID,
					typ:    "booking.completed.provider",
					title:  "Booking Completed",
					body:   "Your booking has been completed.",
					data:   map[string]any{"booking_id": b.BookingID, "post_id": b.PostID, "receiver_id": b.ReceiverUserID},
				},
			)
		}

		return nil
	})

	if err == nil && s.pub != nil && len(toSend) > 0 {
		for _, n := range toSend {
			_ = s.pub.Notify(ctx, n.userID, n.typ, n.title, n.body, n.data)
		}
	}
	return err
}

// ===== QR HMAC =====

// create a deterministic, stable token per booking
func (s *service) makeStableQR(bookingID int64, createdAt time.Time) string {
	payload := fmt.Sprintf("%d:%d", bookingID, createdAt.Unix()) // stable inputs
	mac := hmac.New(sha256.New, s.cfg.QRSecret)
	mac.Write([]byte(payload))
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." +
		base64.RawURLEncoding.EncodeToString(sig)
}

func (s *service) IssueQR(ctx context.Context, id int64, _ time.Duration) (string, error) {
	b, err := s.repo.GetBookingByID(ctx, id, false)
	if err != nil {
		return "", err
	}
	// if already issued, return the stored one
	if b.QRToken != nil && *b.QRToken != "" {
		return *b.QRToken, nil
	}
	// generate once and persist
	token := s.makeStableQR(b.BookingID, b.CreatedAt)
	if err := s.repo.SetBookingQRToken(ctx, b.BookingID, token); err != nil {
		return "", err
	}
	return token, nil
}

func (s *service) ScanQR(ctx context.Context, token string) (*entities.Booking, error) {
	// 1) First, try the stable stored token
	if b, err := s.repo.GetBookingByQRToken(ctx, token, false); err == nil && b != nil {
		// server-side safety gates (prevents replay abuse)
		now := time.Now().In(s.cfg.DayTZ)
		if b.Status != entities.BookingPending {
			return nil, fmt.Errorf("invalid_status")
		}
		if b.ExpireAt != nil && now.After(*b.ExpireAt) {
			return nil, fmt.Errorf("booking_expired")
		}
		return b, nil
	}

	// 2) Backward-compat: accept old short-lived tokens (bookingID:exp.sig)
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("bad_qr")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("bad_qr")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("bad_qr")
	}
	mac := hmac.New(sha256.New, s.cfg.QRSecret)
	mac.Write(raw)
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, fmt.Errorf("bad_qr_sig")
	}
	payload := string(raw) // bookingID:exp
	colon := strings.IndexByte(payload, ':')
	if colon < 0 {
		return nil, fmt.Errorf("bad_qr_payload")
	}
	idStr := payload[:colon]
	expStr := payload[colon+1:]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("bad_qr_payload")
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("bad_qr_payload")
	}
	if time.Now().Unix() > exp {
		return nil, fmt.Errorf("qr_expired")
	}
	return s.repo.GetBookingByID(ctx, id, false)
}

// ExpireSweep transitions stale PENDING bookings to EXPIRED and promotes queue or returns stock.
// It runs in small batches to avoid long transactions.
func (s *service) ExpireSweep(ctx context.Context, max int) error {
	if max <= 0 {
		max = 100
	}
	now := time.Now()
	ids, err := s.repo.ListExpiredPendingIDs(ctx, now, max)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}

	// collect notifications to send after all commits
	var toSend []pendingNotif

	for _, id := range ids {
		// handle each booking in its own tx to reduce contention
		_ = s.repo.WithTx(ctx, func(r Repo) error {
			b, err := r.GetBookingByID(ctx, id, true)
			if err != nil {
				return err
			}
			// double-check status and expiry
			if b.Status != entities.BookingPending || b.ExpireAt == nil || now.Before(*b.ExpireAt) {
				return nil
			}
			// mark expired
			b.Status = entities.BookingExpired
			b.ExpiredAt = &now
			b.UpdatedAt = now
			if err := r.UpdateBooking(ctx, b); err != nil {
				return err
			}

			// notify receiver expiration
			toSend = append(toSend, pendingNotif{
				userID: b.ReceiverUserID,
				typ:    "booking.expired",
				title:  "Booking Expired",
				body:   "Your booking request has expired.",
				data:   map[string]any{"booking_id": b.BookingID, "post_id": b.PostID},
			})

			// promote next queued or return stock
			if next, e := r.FindNextQueued(ctx, b.PostID); e == nil && next != nil {
				exp := now.Add(s.cfg.HoldTTL)
				next.Status = entities.BookingPending
				next.ExpireAt = &exp
				next.UpdatedAt = now
				if err := r.UpdateBooking(ctx, next); err != nil {
					return err
				}
				toSend = append(toSend, pendingNotif{
					userID: next.ReceiverUserID,
					typ:    "queue.promoted",
					title:  "Your queue position is up",
					body:   "Your booking request has been promoted. See you at the pickup point soon!",
					data:   map[string]any{"booking_id": next.BookingID, "post_id": b.PostID, "new_status": "PENDING"},
				})
				return nil
			} else if errors.Is(e, gorm.ErrRecordNotFound) {
				// no queued, return stock
				return r.IncrementQty(ctx, b.PostID, 1)
			} else if e != nil {
				return e
			}
			return nil
		})
	}

	// publish notifications outside tx
	if s.pub != nil {
		for _, n := range toSend {
			_ = s.pub.Notify(ctx, n.userID, n.typ, n.title, n.body, n.data)
		}
	}
	return nil
}

func (s *service) DailyLimitLeft(ctx context.Context, userID int64, now time.Time) (int, int, int, time.Time, time.Time, error) {
	// match your current defaulting (you set 2 in NewService)
	limit := s.cfg.DailyLimit
	if limit == 0 {
		limit = 2
	}
	start, end := s.dayBounds(now)
	n, err := s.repo.CountActiveTodayByUser(ctx, userID, start, end)
	if err != nil {
		return 0, 0, 0, time.Time{}, time.Time{}, err
	}
	used := int(n)
	left := limit - used
	if left < 0 { left = 0 }
	return limit, used, left, start, end, nil
}