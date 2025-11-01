package booking

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/RathaTart/FoodBridge/entities"
)

type gormRepo struct{ db *gorm.DB }

func NewGormRepo(db *gorm.DB) Repo { return &gormRepo{db: db} }

func (r *gormRepo) WithTx(ctx context.Context, fn func(Repo) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&gormRepo{db: tx})
	})
}

func (r *gormRepo) TryReserveStock(ctx context.Context, postID int64) (bool, error) {
	res := r.db.WithContext(ctx).Exec(
		`UPDATE posts SET quantity = quantity - 1
    	WHERE post_id = ? AND quantity > 0`,
		postID,
	)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (r *gormRepo) DecrementQty(ctx context.Context, postID int64, n int) error {
	return r.db.WithContext(ctx).Exec(`UPDATE posts SET quantity = quantity - ? WHERE post_id = ?`, n, postID).Error
}
func (r *gormRepo) IncrementQty(ctx context.Context, postID int64, n int) error {
	return r.db.WithContext(ctx).Exec(`UPDATE posts SET quantity = quantity + ? WHERE post_id = ?`, n, postID).Error
}

func (r *gormRepo) CreateBooking(ctx context.Context, b *entities.Booking) error {
	return r.db.WithContext(ctx).Create(b).Error
}
func (r *gormRepo) UpdateBooking(ctx context.Context, b *entities.Booking) error {
	return r.db.WithContext(ctx).Model(&entities.Booking{}).
		Where("booking_id = ?", b.BookingID).
		Updates(b).Error
}
func (r *gormRepo) GetBookingByID(ctx context.Context, id int64, forUpdate bool) (*entities.Booking, error) {
	var b entities.Booking
	tx := r.db.WithContext(ctx).Where("booking_id = ?", id)
	if forUpdate {
		tx = tx.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := tx.First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *gormRepo) ListBookings(ctx context.Context, f Filter) ([]entities.Booking, error) {
    q := r.db.WithContext(ctx).Model(&entities.Booking{})
    if f.PostID != nil {
        q = q.Where("post_id = ?", *f.PostID)
    }
    if f.ReceiverUserID != nil {
        q = q.Where("receiver_user_id = ?", *f.ReceiverUserID)
    }
    if len(f.Statuses) > 0 {
        q = q.Where("status IN ?", f.Statuses)
    } else if f.Status != nil {
        q = q.Where("status = ?", *f.Status)
    }
    var out []entities.Booking
    if err := q.Order("created_at DESC").Find(&out).Error; err != nil {
        return nil, err
    }
    return out, nil
}

func (r *gormRepo) CountReceivers(ctx context.Context, f Filter) (int64, error) {
	q := r.db.WithContext(ctx).Model(&entities.Booking{})
	if f.PostID != nil {
		q = q.Where("post_id = ?", *f.PostID)
	}
	if f.ReceiverUserID != nil {
		q = q.Where("receiver_user_id = ?", *f.ReceiverUserID)
	}
	if len(f.Statuses) > 0 {
		q = q.Where("status IN ?", f.Statuses)
	} else if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	var n int64
	if err := q.Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func (r *gormRepo) GetBookingByQRToken(ctx context.Context, token string, forUpdate bool) (*entities.Booking, error) {
	var b entities.Booking
	tx := r.db.WithContext(ctx).Where("qr_token = ?", token)
	if forUpdate {
		tx = tx.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := tx.First(&b).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *gormRepo) SetBookingQRToken(ctx context.Context, id int64, token string) error {
	return r.db.WithContext(ctx).
		Model(&entities.Booking{}).
		Where("booking_id = ?", id).
		Update("qr_token", token).Error
}

func (r *gormRepo) NextQueuePos(ctx context.Context, postID int64) (int, error) {
	var max int
	if err := r.db.WithContext(ctx).
		Raw(`SELECT COALESCE(MAX(queue_pos),0) FROM bookings WHERE post_id = ? AND status = 'QUEUED'`, postID).
		Scan(&max).Error; err != nil {
		return 0, err
	}
	return max + 1, nil
}

func (r *gormRepo) FindNextQueued(ctx context.Context, postID int64) (*entities.Booking, error) {
	var b entities.Booking
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("post_id = ? AND status = 'QUEUED'", postID).
		Order("queue_pos ASC, created_at ASC").
		First(&b).Error
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *gormRepo) CountActiveTodayByUser(ctx context.Context, userID int64, dayStart, dayEnd time.Time) (int64, error) {
	var cnt int64
	err := r.db.WithContext(ctx).
		Model(&entities.Booking{}).
		Where("receiver_user_id = ?", userID).
		Where("created_at >= ? AND created_at < ?", dayStart, dayEnd).
		Where("status IN ('PENDING','QUEUED','COMPLETED')").
		Count(&cnt).Error
	return cnt, err
}

func (r *gormRepo) GetPostOwnerID(ctx context.Context, postID int64) (int64, error) {
	var ownerID int64
	// If your column is NOT posts.user_id, change it here (e.g., provider_user_id).
	err := r.db.WithContext(ctx).
		Raw(`SELECT provider_id FROM posts WHERE post_id = ?`, postID).
		Scan(&ownerID).Error
	return ownerID, err
}

// ListExpiredPendingIDs returns IDs of bookings where status=PENDING and expire_at <= before.
func (r *gormRepo) ListExpiredPendingIDs(ctx context.Context, before time.Time, limit int) ([]int64, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.WithContext(ctx).
		Model(&entities.Booking{}).
		Select("booking_id").
		Where("status = ?", entities.BookingPending).
		Where("expire_at IS NOT NULL AND expire_at <= ?", before).
		Order("expire_at ASC").
		Limit(limit).
		Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
