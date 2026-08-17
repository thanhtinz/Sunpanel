package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
)

// AuditEntry mô tả một hành động cần ghi vào nhật ký kiểm toán.
type AuditEntry struct {
	UserID   uint
	Username string
	// Action là mã hành động dạng "resource.verb", ví dụ "user.create".
	Action string
	// Resource là đối tượng bị tác động.
	Resource string
	IP       string
	Success  bool
	// Detail là thông tin bổ sung; tuyệt đối không đưa mật khẩu hay token vào đây.
	Detail map[string]any
}

// AuditService ghi nhật ký kiểm toán.
type AuditService struct {
	db *gorm.DB
}

// NewAuditService tạo service nhật ký kiểm toán.
func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

// Record ghi một hành động vào nhật ký.
//
// Hàm này cố ý không trả về lỗi: nhật ký hỏng không được phép làm hỏng thao tác
// nghiệp vụ mà nó đang ghi lại. Lỗi được đẩy vào log hệ thống để quản trị viên biết.
func (s *AuditService) Record(ctx context.Context, entry AuditEntry) {
	log := model.AuditLog{
		UserID:   entry.UserID,
		Username: truncate(entry.Username, 64),
		Action:   entry.Action,
		Resource: truncate(entry.Resource, 512),
		IP:       entry.IP,
		Success:  entry.Success,
	}

	if len(entry.Detail) > 0 {
		if data, err := json.Marshal(entry.Detail); err == nil {
			log.Detail = string(data)
		}
	}

	if err := s.db.WithContext(ctx).Create(&log).Error; err != nil {
		slog.Error("không ghi được nhật ký kiểm toán",
			"action", entry.Action, "resource", entry.Resource, "error", err)
	}
}

// AuditQuery là bộ lọc khi tra cứu nhật ký kiểm toán.
type AuditQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"pageSize"`
	Action   string `form:"action"`
	Username string `form:"username"`
}

// Page là một trang kết quả kèm tổng số bản ghi.
type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// List tra cứu nhật ký kiểm toán theo trang, mới nhất trước.
func (s *AuditService) List(ctx context.Context, q AuditQuery) (*Page[model.AuditLog], error) {
	page, pageSize := normalizePaging(q.Page, q.PageSize)

	query := s.db.WithContext(ctx).Model(&model.AuditLog{})
	if q.Action != "" {
		query = query.Where("action = ?", q.Action)
	}
	if q.Username != "" {
		query = query.Where("username = ?", q.Username)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	var items []model.AuditLog
	err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&items).Error
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	return &Page[model.AuditLog]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// ListLogins tra cứu nhật ký đăng nhập theo trang, mới nhất trước.
func (s *AuditService) ListLogins(ctx context.Context, q AuditQuery) (*Page[model.LoginLog], error) {
	page, pageSize := normalizePaging(q.Page, q.PageSize)

	query := s.db.WithContext(ctx).Model(&model.LoginLog{})
	if q.Username != "" {
		query = query.Where("username = ?", q.Username)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	var items []model.LoginLog
	err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&items).Error
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	return &Page[model.LoginLog]{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// normalizePaging kẹp tham số phân trang vào khoảng hợp lệ, tránh việc một yêu
// cầu xin pageSize khổng lồ làm nghẽn máy chủ.
func normalizePaging(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	switch {
	case pageSize < 1:
		pageSize = 20
	case pageSize > 200:
		pageSize = 200
	}
	return page, pageSize
}
