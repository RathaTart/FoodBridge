package post

import (
	"encoding/json"
	"strings"

	"github.com/RathaTart/FoodBridge/dto"
	"github.com/RathaTart/FoodBridge/entities"
	"gorm.io/gorm"
)

type repositoryImpl struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repositoryImpl{db: db}
}

// ปิดโพสต์ที่ close_time (epoch sec) <= nowUnix
func (r *repositoryImpl) CloseExpired(nowUnix int64) error {
	return r.db.Model(&entities.Post{}).
		Where("status = ?", "OPEN").
		Where("close_time IS NOT NULL AND close_time <> 0 AND close_time <= ?", nowUnix).
		Update("status", "CLOSED").Error
}

// ปิดโพสต์ที่สต็อกหมด (เดิมใช้ได้อยู่ ไม่ต้องแก้)
func (r *repositoryImpl) CloseDepletedAll() error {
	return r.db.Exec(`
		UPDATE posts p
		SET status = 'CLOSED'
		WHERE p.status = 'OPEN'
		  AND p.quantity IS NOT NULL
		  AND (
			p.quantity <= 0
			OR p.quantity <= (
				SELECT COUNT(*) FROM bookings b
				WHERE b.post_id = p.post_id AND b.status = 'COMPLETED'
			)
		  )
	`).Error
}

// ----------------- Post -----------------
func (r *repositoryImpl) CreatePost(p *entities.Post) error {
	return r.db.Create(p).Error
}

func (r *repositoryImpl) UpdatePost(p *entities.Post) error {
	return r.db.Save(p).Error
}

func (r *repositoryImpl) DeletePostHard(postID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("post_id = ?", postID).Delete(&entities.PostDetail{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&entities.Post{}, "post_id = ?", postID).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *repositoryImpl) FindPostByID(postID uint) (*entities.Post, error) {
	var p entities.Post
	if err := r.db.First(&p, "post_id = ?", postID).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *repositoryImpl) ListPosts(q dto.ListPostsQuery, uid uint) ([]entities.Post, int64, error) {
	tx := r.db.Model(&entities.Post{})

	// keyword
	if strings.TrimSpace(q.Q) != "" {
		like := "%" + strings.TrimSpace(q.Q) + "%"
		tx = tx.Where(r.db.
			Where("title ILIKE ?", like).
			Or("description ILIKE ?", like))
	}

	// status
	if q.Status != nil && *q.Status != "" {
		tx = tx.Where("status = ?", strings.ToUpper(*q.Status))
	}

	// is_giveaway
	if q.IsGiveaway != nil {
		tx = tx.Where("is_giveaway = ?", *q.IsGiveaway)
	}

	// เฉพาะของฉัน
	if q.Mine != nil && *q.Mine {
		tx = tx.Where("provider_id = ?", uid)
	}

	// category (JSON array contains)
	if q.Category != nil && strings.TrimSpace(*q.Category) != "" {
		// ใช้ @> กับ jsonb (Postgres): categories @> '["ของคาว"]'
		val, _ := json.Marshal([]string{strings.TrimSpace(*q.Category)})
		tx = tx.Where("categories @> ?", string(val))
	}

	// provider_id
	if q.ProviderID != nil {
		tx = tx.Where("provider_id = ?", *q.ProviderID)
	} else if q.Mine != nil && *q.Mine {
		tx = tx.Where("provider_id = ?", uid)
	}

	// sort
	switch q.Sort {
	case "created_at":
		tx = tx.Order("created_at ASC")
	case "-created_at", "":
		tx = tx.Order("created_at DESC")
	case "title":
		tx = tx.Order("title ASC")
	case "-title":
		tx = tx.Order("title DESC")
	default:
		tx = tx.Order("created_at DESC")
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}

	// post_type
	if q.PostType != nil && strings.TrimSpace(*q.PostType) != "" {
		tx = tx.Where("post_type = ?", strings.ToUpper(strings.TrimSpace(*q.PostType)))
	}

	var rows []entities.Post
	if err := tx.Limit(q.PageSize).Offset((q.Page - 1) * q.PageSize).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// -------------- PostDetail --------------
func (r *repositoryImpl) CreateDetail(d *entities.PostDetail) error {
	return r.db.Create(d).Error
}

func (r *repositoryImpl) UpdateDetail(d *entities.PostDetail) error {
	return r.db.Save(d).Error
}

func (r *repositoryImpl) DeleteDetail(postID, detailID uint) error {
	return r.db.Delete(&entities.PostDetail{}, "post_id = ? AND post_detail_id = ?", postID, detailID).Error
}

func (r *repositoryImpl) ListDetails(postID uint) ([]entities.PostDetail, error) {
	var out []entities.PostDetail
	err := r.db.Where("post_id = ?", postID).Order("created_at ASC").Find(&out).Error
	return out, err
}

func (r *repositoryImpl) FindDetail(postID, detailID uint) (*entities.PostDetail, error) {
	var d entities.PostDetail
	if err := r.db.First(&d, "post_id = ? AND post_detail_id = ?", postID, detailID).Error; err != nil {
		return nil, err
	}
	return &d, nil
}
