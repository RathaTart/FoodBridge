package post

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/RathaTart/FoodBridge/dto"
	"github.com/RathaTart/FoodBridge/entities"
	"gorm.io/datatypes"
)

// ============ service ============

type serviceImpl struct {
	repo Repository
}

func NewService(dbRepo Repository) Service {
	return &serviceImpl{repo: dbRepo}
}

// ============ helpers ============

func stringsToJSON(ss []string) datatypes.JSON {
	if ss == nil {
		return datatypes.JSON([]byte("[]"))
	}
	b, _ := json.Marshal(ss)
	return datatypes.JSON(b)
}
func jsonToStrings(j datatypes.JSON) []string {
	var out []string
	_ = json.Unmarshal([]byte(j), &out)
	return out
}

var allowedCats = map[string]struct{}{
	"ของคาว": {}, "ของหวาน": {}, "ผักสด": {}, "ของสด": {},
}

func normalizeCategories(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, s := range in {
		v := strings.TrimSpace(s)
		if v == "" {
			continue
		}
		if _, ok := allowedCats[v]; !ok {
			return nil, errors.New("invalid category: " + v)
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out, nil
}
func toUnixPtr(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	u := t.Unix()
	return &u
}

func toPostResp(p *entities.Post) *dto.PostResponse {
	return &dto.PostResponse{
		PostID:     p.PostID,
		ProviderID: p.ProviderID,
		Title:      p.Title, Description: p.Description,
		PostType:   p.PostType,
		IsGiveaway: p.IsGiveaway,
		Price:      p.Price, Quantity: p.Quantity,
		OpenTime: toUnixPtr(p.OpenTime), CloseTime: toUnixPtr(p.CloseTime),
		Status:   string(p.Status),
		Address:  p.Address,
		Province: p.Province,
		District: p.District, 
		Lat:      p.Lat, Lng: p.Lng, Phone: p.Phone,
		Categories: jsonToStrings(p.Categories),
		Images:     jsonToStrings(p.Images),
		CreatedAt:  p.CreatedAt.Unix(), UpdatedAt: p.UpdatedAt.Unix(),
	}
}

func ownerOnly(uid uint, providerID uint) error {
	if uid != providerID {
		return errors.New("forbidden: only owner can modify")
	}
	return nil
}

func toDetailResp(d *entities.PostDetail) *dto.PostDetailResponse {
	return &dto.PostDetailResponse{
		PostDetailID: d.PostDetailID,
		PostID:       d.PostID,
		ItemName:     d.ItemName,
		Qty:          d.Qty,
		Unit:         d.Unit,
		Note:         d.Note,
		ExpireAt:     toUnixPtr(d.ExpireAt), // ใช้ helper เดิมในไฟล์
		CreatedAt:    d.CreatedAt.Unix(),
		UpdatedAt:    d.UpdatedAt.Unix(),
	}
}

func (s *serviceImpl) AutoCloseSweep(nowUnix int64) error {
	// 1) ปิดโพสต์ที่หมดเวลา
	if err := s.repo.CloseExpired(nowUnix); err != nil {
		return err
	}
	// 2) ปิดโพสต์ที่สต็อกหมด (booking COMPLETED >= quantity)
	if err := s.repo.CloseDepletedAll(); err != nil {
		return err
	}
	return nil
}

// ============ POST ============

func (s *serviceImpl) Create(uid uint, req dto.CreatePostRequest) (*dto.PostResponse, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, errors.New("title is required")
	}

	// categories
	cats, err := normalizeCategories(req.Categories)
	if err != nil {
		return nil, err
	}

	// post_type (PROVIDE | COMMUNITY)
	pt := strings.ToUpper(strings.TrimSpace(req.PostType))
	if pt == "" {
		pt = "PROVIDE"
	}
	if pt != "PROVIDE" && pt != "COMMUNITY" {
		return nil, errors.New("invalid post_type")
	}

	// lat/lng validation (ถ้ามี)
	if req.Lat != nil && (*req.Lat < -90 || *req.Lat > 90) {
		return nil, errors.New("lat must be between -90 and 90")
	}
	if req.Lng != nil && (*req.Lng < -180 || *req.Lng > 180) {
		return nil, errors.New("lng must be between -180 and 180")
	}

	p := &entities.Post{
		ProviderID:  uid,
		Title:       title,
		Description: strings.TrimSpace(req.Description),

		PostType: pt,

		Status:   entities.PostStatusOpen,
		Address:  strings.TrimSpace(req.Address),
		Province: req.Province,
		District: req.District,
		Lat:      req.Lat, Lng: req.Lng,
		Phone: strings.TrimSpace(req.Phone),

		Categories: stringsToJSON(cats),
		Images:     stringsToJSON(req.Images),
	}

	if pt == "COMMUNITY" {
		// โพสต์ชุมชน: ignore/ล้างฟิลด์พาณิชย์
		p.IsGiveaway = false
		p.Price = nil
		p.Quantity = nil
		p.OpenTime = nil
		p.CloseTime = nil
		p.Phone = strings.TrimSpace(req.Phone) // จะล้างก็ได้ ถ้าไม่ต้องการให้แสดง
	} else {
		// PROVIDE: validation เฉพาะเมื่อ is_giveaway = true
		p.IsGiveaway = req.IsGiveaway
		p.Price = req.Price
		p.Quantity = req.Quantity
		p.OpenTime = req.OpenTime
		p.CloseTime = req.CloseTime

		if p.IsGiveaway {
			if p.OpenTime == nil || p.CloseTime == nil {
				return nil, errors.New("open_time and close_time are required when is_giveaway=true")
			}
			if p.Price == nil || *p.Price < 0 {
				return nil, errors.New("price must be >= 0 when is_giveaway=true")
			}
			if p.Quantity == nil || *p.Quantity < 0 {
				return nil, errors.New("quantity must be >= 0 when is_giveaway=true")
			}
		}
	}

	if err := s.repo.CreatePost(p); err != nil {
		return nil, err
	}
	return toPostResp(p), nil
}

func (s *serviceImpl) Update(uid, postID uint, req dto.UpdatePostRequest) (*dto.PostResponse, error) {
	p, err := s.repo.FindPostByID(postID)
	if err != nil {
		return nil, err
	}
	if err := ownerOnly(uid, p.ProviderID); err != nil {
		return nil, err
	}

	// เปลี่ยนชนิดโพสต์ได้ (PROVIDE | COMMUNITY)
	if req.PostType != nil {
		pt := strings.ToUpper(strings.TrimSpace(*req.PostType))
		if pt != "PROVIDE" && pt != "COMMUNITY" {
			return nil, errors.New("invalid post_type")
		}
		p.PostType = pt
	}

	// แก้คอร์
	if req.Title != nil {
		if v := strings.TrimSpace(*req.Title); v != "" {
			p.Title = v
		}
	}
	if req.Description != nil {
		p.Description = strings.TrimSpace(*req.Description)
	}

	// ตำแหน่งที่ตั้ง
	if req.Lat != nil {
		if *req.Lat < -90 || *req.Lat > 90 {
			return nil, errors.New("lat must be between -90 and 90")
		}
		p.Lat = req.Lat
	}
	if req.Lng != nil {
		if *req.Lng < -180 || *req.Lng > 180 {
			return nil, errors.New("lng must be between -180 and 180")
		}
		p.Lng = req.Lng
	}
	if req.Address != nil {
		p.Address = strings.TrimSpace(*req.Address)
	}
	if req.Province != nil {
		p.Province = *req.Province
	}
	if req.District != nil {
		p.District = *req.District
	}

	if req.Phone != nil {
		p.Phone = strings.TrimSpace(*req.Phone)
	}

	// หมวดหมู่/รูป
	if req.Categories != nil {
		cats, err := normalizeCategories(*req.Categories)
		if err != nil {
			return nil, err
		}
		p.Categories = stringsToJSON(cats)
	}
	if req.Images != nil {
		p.Images = stringsToJSON(*req.Images)
	}

	// สถานะโพสต์
	if req.Status != nil {
		st := strings.ToUpper(*req.Status)
		if st != string(entities.PostStatusOpen) && st != string(entities.PostStatusClosed) {
			return nil, errors.New("invalid status")
		}
		p.Status = entities.PostStatus(st)
	}

	// ===== พฤติกรรมตามชนิดโพสต์ =====
	if p.PostType == "COMMUNITY" {
		// ล้างฟิลด์พาณิชย์ + บังคับปิดแจก
		p.IsGiveaway = false
		p.Price = nil
		p.Quantity = nil
		p.OpenTime = nil
		p.CloseTime = nil
		// p.Phone = "" // ถ้าต้องการล้าง กดบรรทัดนี้เพิ่มได้
	} else { // PROVIDE
		// toggle is_giveaway
		if req.IsGiveaway != nil {
			p.IsGiveaway = *req.IsGiveaway
		}
		// อัปเดตฟิลด์พาณิชย์
		if req.Price != nil {
			if *req.Price < 0 {
				return nil, errors.New("price must be >= 0")
			}
			p.Price = req.Price
		}
		if req.Quantity != nil {
			if *req.Quantity < 0 {
				return nil, errors.New("quantity must be >= 0")
			}
			p.Quantity = req.Quantity
		}
		if req.OpenTime != nil {
			p.OpenTime = req.OpenTime
		}
		if req.CloseTime != nil {
			p.CloseTime = req.CloseTime
		}
		// ถ้าอยู่ในโหมดแจก ต้องมีค่าบังคับ
		if p.IsGiveaway {
			if p.OpenTime == nil || p.CloseTime == nil {
				return nil, errors.New("open_time and close_time are required when is_giveaway=true")
			}
			if p.Price == nil || *p.Price < 0 {
				return nil, errors.New("price must be >= 0 when is_giveaway=true")
			}
			if p.Quantity == nil || *p.Quantity < 0 {
				return nil, errors.New("quantity must be >= 0 when is_giveaway=true")
			}
		}
	}

	if err := s.repo.UpdatePost(p); err != nil {
		return nil, err
	}
	return toPostResp(p), nil
}

func (s *serviceImpl) Delete(uid, postID uint) error {
	p, err := s.repo.FindPostByID(postID)
	if err != nil {
		return err
	}
	if err := ownerOnly(uid, p.ProviderID); err != nil {
		return err
	}
	return s.repo.DeletePostHard(postID)
}

func (s *serviceImpl) GetByID(uid, postID uint) (*dto.PostResponse, error) {
	p, err := s.repo.FindPostByID(postID)
	if err != nil {
		return nil, err
	}
	return toPostResp(p), nil
}

func (s *serviceImpl) List(uid uint, q dto.ListPostsQuery) (*dto.PagedResult[dto.PostResponse], error) {
	rows, total, err := s.repo.ListPosts(q, uid)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PostResponse, 0, len(rows))
	for i := range rows {
		out = append(out, *toPostResp(&rows[i]))
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
	return &dto.PagedResult[dto.PostResponse]{
		Items: out, Total: total, Page: q.Page, PageSize: q.PageSize,
	}, nil
}

// ============ POST DETAIL ============

func (s *serviceImpl) CreateDetail(uid, postID uint, req dto.CreatePostDetailRequest) (*dto.PostDetailResponse, error) {
	p, err := s.repo.FindPostByID(postID)
	if err != nil {
		return nil, err
	}
	if err := ownerOnly(uid, p.ProviderID); err != nil {
		return nil, err
	}

	if strings.TrimSpace(req.ItemName) == "" {
		return nil, errors.New("item_name is required")
	}
	d := &entities.PostDetail{
		PostID:   postID,
		ItemName: strings.TrimSpace(req.ItemName),
		Qty:      req.Qty,
		Unit:     strings.TrimSpace(req.Unit),
		Note:     strings.TrimSpace(req.Note),
		ExpireAt: req.ExpireAt,
	}
	if err := s.repo.CreateDetail(d); err != nil {
		return nil, err
	}
	return toDetailResp(d), nil
}

func (s *serviceImpl) UpdateDetail(uid, postID, detailID uint, req dto.UpdatePostDetailRequest) (*dto.PostDetailResponse, error) {
	p, err := s.repo.FindPostByID(postID)
	if err != nil {
		return nil, err
	}
	if err := ownerOnly(uid, p.ProviderID); err != nil {
		return nil, err
	}

	d, err := s.repo.FindDetail(postID, detailID)
	if err != nil {
		return nil, err
	}

	if req.ItemName != nil {
		d.ItemName = strings.TrimSpace(*req.ItemName)
	}
	if req.Qty != nil {
		d.Qty = *req.Qty
	}
	if req.Unit != nil {
		d.Unit = strings.TrimSpace(*req.Unit)
	}
	if req.Note != nil {
		d.Note = strings.TrimSpace(*req.Note)
	}
	if req.ExpireAt != nil {
		d.ExpireAt = req.ExpireAt
	}

	if err := s.repo.UpdateDetail(d); err != nil {
		return nil, err
	}
	return toDetailResp(d), nil
}

func (s *serviceImpl) DeleteDetail(uid, postID, detailID uint) error {
	p, err := s.repo.FindPostByID(postID)
	if err != nil {
		return err
	}
	if err := ownerOnly(uid, p.ProviderID); err != nil {
		return err
	}
	return s.repo.DeleteDetail(postID, detailID)
}

func (s *serviceImpl) ListDetails(uid, postID uint) ([]dto.PostDetailResponse, error) {
	dets, err := s.repo.ListDetails(postID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PostDetailResponse, 0, len(dets))
	for i := range dets {
		out = append(out, *toDetailResp(&dets[i]))
	}
	return out, nil
}
