package dto

import "time"

// ใช้ RFC3339 เช่น "2026-09-19T18:00:00Z"
type CreatePostRequest struct {
	Title       string `json:"title"        form:"title"        validate:"required"`
	Description string `json:"description"  form:"description"`
	IsGiveaway  bool   `json:"is_giveaway"  form:"is_giveaway"`
	Price       *int   `json:"price"        form:"price"`
	Quantity    *int   `json:"quantity"     form:"quantity"`

	// time_format ต้องเป็นสตริง literal ใน tag เท่านั้น (ห้ามต่อสตริง)
	OpenTime  *time.Time `json:"open_time"    form:"open_time"    time_format:"2006-01-02T15:04:05Z07:00"`
	CloseTime *time.Time `json:"close_time"   form:"close_time"   time_format:"2006-01-02T15:04:05Z07:00"`

	Address  string   `json:"address"      form:"address"`
	Province string   `json:"province"     form:"province"`
	District string   `json:"district"     form:"district"`
	Lat      *float64 `json:"lat"          form:"lat"`
	Lng      *float64 `json:"lng"          form:"lng"`
	Phone    string   `json:"phone"        form:"phone"`

	// ใน form-data ให้ส่งคีย์ซ้ำหลายแถว เช่น images, images, ...
	Categories []string `json:"categories"   form:"categories"`
	Images     []string `json:"images"       form:"images"`

	PostType string `json:"post_type"    form:"post_type"    validate:"omitempty,oneof=PROVIDE COMMUNITY"`
}

type UpdatePostRequest struct {
	Title       *string `json:"title"        form:"title"`
	Description *string `json:"description"  form:"description"`
	IsGiveaway  *bool   `json:"is_giveaway"  form:"is_giveaway"`
	Price       *int    `json:"price"        form:"price"`
	Quantity    *int    `json:"quantity"     form:"quantity"`

	OpenTime  *time.Time `json:"open_time"    form:"open_time"    time_format:"2006-01-02T15:04:05Z07:00"`
	CloseTime *time.Time `json:"close_time"   form:"close_time"   time_format:"2006-01-02T15:04:05Z07:00"`

	Status     *string   `json:"status"       form:"status"`
	Address    *string   `json:"address"      form:"address"`
	Province   *string   `json:"province"     form:"province"`
	District   *string   `json:"district"    form:"district"`
	Lat        *float64  `json:"lat"          form:"lat"`
	Lng        *float64  `json:"lng"          form:"lng"`
	Phone      *string   `json:"phone"        form:"phone"`
	Categories *[]string `json:"categories"   form:"categories"`
	Images     *[]string `json:"images"       form:"images"`
	PostType   *string   `json:"post_type"    form:"post_type"    validate:"omitempty,oneof=PROVIDE COMMUNITY"`
}

type PostResponse struct {
	PostID      uint   `json:"post_id"`
	ProviderID  uint   `json:"provider_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	PostType    string `json:"post_type"`

	IsGiveaway bool `json:"is_giveaway"`
	Price      *int `json:"price"`
	Quantity   *int `json:"quantity"`

	OpenTime  *int64 `json:"open_time,omitempty"`
	CloseTime *int64 `json:"close_time,omitempty"`

	Status   string   `json:"status"`
	Address  string   `json:"address"`
	Province string   `json:"province"`
	District string   `json:"district"`
	Lat      *float64 `json:"lat"`
	Lng      *float64 `json:"lng"`
	Phone    string   `json:"phone"`

	Categories []string `json:"categories"`
	Images     []string `json:"images"`

	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

type ListPostsQuery struct {
	Page       int     `query:"page"`
	PageSize   int     `query:"page_size"`
	Q          string  `query:"q"`           // title/description
	Status     *string `query:"status"`      // OPEN/CLOSED
	Mine       *bool   `query:"mine"`        // เฉพาะของฉัน (provider_id == uid)
	IsGiveaway *bool   `query:"is_giveaway"` // filter by mode
	Category   *string `query:"category"`    // เช่น ของคาว
	Sort       string  `query:"sort"`        // created_at|-created_at|title|-title
	PostType   *string `query:"post_type"`
	ProviderID *uint   `query:"-"`
}

// ====== PostDetail ======
type CreatePostDetailRequest struct {
	ItemName string     `json:"item_name" validate:"required"`
	Qty      int        `json:"qty"`
	Unit     string     `json:"unit"`
	Note     string     `json:"note"`
	ExpireAt *time.Time `json:"expire_at"`
}
type UpdatePostDetailRequest struct {
	ItemName *string    `json:"item_name"`
	Qty      *int       `json:"qty"`
	Unit     *string    `json:"unit"`
	Note     *string    `json:"note"`
	ExpireAt *time.Time `json:"expire_at"`
}

type PostDetailResponse struct {
	PostDetailID uint   `json:"post_detail_id"`
	PostID       uint   `json:"post_id"`
	ItemName     string `json:"item_name"`
	Qty          int    `json:"qty"`
	Unit         string `json:"unit"`
	Note         string `json:"note"`
	ExpireAt     *int64 `json:"expire_at,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}
