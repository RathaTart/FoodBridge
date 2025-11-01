package entities

import (
	"time"

	"gorm.io/datatypes"
)

type PostStatus string

const (
	PostStatusOpen   PostStatus = "OPEN"
	PostStatusClosed PostStatus = "CLOSED"
)

// PostType เก็บชนิดโพสต์ 2 แบบ (แมปกับ enum post_type ใน DB)
// - PROVIDE   = โพสต์แจก/ขาย (ฟิลด์ด้านล่างมีผลตามปกติ)
// - COMMUNITY = โพสต์ชุมชน (จะไม่ใช้ open/close/quantity/price/phone และห้ามจอง)
type Post struct {
	PostID     uint `gorm:"primaryKey"`
	ProviderID uint `gorm:"index;not null"` // FK -> users.user_id

	// ชนิดโพสต์ (ใช้ enum post_type ใน DB)
	PostType string `gorm:"type:post_type;not null;default:'PROVIDE';index" json:"post_type"`

	// Core
	Title       string `gorm:"type:varchar(200);not null"`
	Description string `gorm:"type:text"`

	// โหมดประกาศ (ใช้เฉพาะเมื่อ PostType=PROVIDE)
	// true = แจก/ฟรี/ลดราคา, false = ไม่ได้ประกาศแจก (COMMUNITY จะถูกบังคับให้ false เสมอ)
	IsGiveaway bool `gorm:"not null;default:false"`

	// ราคา/จำนวน (ใช้เฉพาะเมื่อ PostType=PROVIDE; ถ้า COMMUNITY ควรเป็น nil)
	Price    *int `gorm:""`
	Quantity *int `gorm:""`

	// เวลาเปิด–ปิด (ใช้เฉพาะเมื่อ PostType=PROVIDE)
	OpenTime  *time.Time
	CloseTime *time.Time

	// สถานะ (เจ้าของสามารถปิดโพสต์เองได้)
	Status PostStatus `gorm:"type:varchar(20);not null;default:'OPEN';index"`

	// พิกัด/ที่อยู่ + โทรศัพท์ติดต่อโพสต์นี้
	Address  string   `gorm:"type:varchar(255)"`
	Province string   `gorm:"type:varchar(100)" json:"province"`
	District string   `gorm:"type:varchar(100)" json:"district"`
	Lat      *float64 `gorm:"index"`
	Lng      *float64 `gorm:"index"`
	Phone    string   `gorm:"type:varchar(32)"`

	// หมวดหมู่อาหาร (หลายค่า) – เก็บเป็น JSON array ของ string
	Categories datatypes.JSON `gorm:"type:jsonb"` // เช่น ["ของคาว","ของหวาน"]

	// สื่อ (เก็บลิงก์ไฟล์รูป)
	Images datatypes.JSON `gorm:"type:jsonb"` // []string

	CreatedAt    time.Time
	UpdatedAt    time.Time
	LikeCount    int `gorm:"default:0"`
	CommentCount int `gorm:"default:0"` // visible comments only
}
