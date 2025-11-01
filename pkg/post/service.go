package post

import "github.com/RathaTart/FoodBridge/dto"
import "time"

type Service interface {
	// Post
	Create(uid uint, req dto.CreatePostRequest) (*dto.PostResponse, error)
	Update(uid, postID uint, req dto.UpdatePostRequest) (*dto.PostResponse, error)
	Delete(uid, postID uint) error
	GetByID(uid, postID uint) (*dto.PostResponse, error) // uid ใช้สำหรับ rule "mine" บางกรณี
	List(uid uint, q dto.ListPostsQuery) (*dto.PagedResult[dto.PostResponse], error)

	// PostDetail (ผูกกับ PostID)
	CreateDetail(uid, postID uint, req dto.CreatePostDetailRequest) (*dto.PostDetailResponse, error)
	UpdateDetail(uid, postID, detailID uint, req dto.UpdatePostDetailRequest) (*dto.PostDetailResponse, error)
	DeleteDetail(uid, postID, detailID uint) error
	ListDetails(uid, postID uint) ([]dto.PostDetailResponse, error)

	// Auto-close sweep
	AutoCloseSweep(nowUnix int64) error // กวาดปิดโพสต์ตามกฎเวลา/สต็อก
}

type Config struct {
	HoldTTL      time.Duration
	QRTokenTTL   time.Duration
	QRSecret     []byte
	DayTZ        *time.Location // default: time.Local (Asia/Bangkok for you)
	DailyLimit   int            // default: 1
}
