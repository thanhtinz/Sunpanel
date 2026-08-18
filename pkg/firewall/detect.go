package firewall

import (
	"context"
	"log/slog"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// Detect chọn công cụ tường lửa phù hợp với máy chủ.
//
// Thứ tự thử phản ánh mức phổ biến trên các bản Linux dùng làm máy chủ: ufw là
// mặc định của Debian và Ubuntu, firewalld của họ RHEL. Máy có cả hai thì ưu
// tiên công cụ đang thực sự bật, vì đó là cái đang điều khiển lưu lượng.
func Detect(ctx context.Context, h host.Host) Manager {
	candidates := []Manager{NewUFW(h), NewFirewalld(h)}

	var firstAvailable Manager
	for _, candidate := range candidates {
		status, err := candidate.Status(ctx)
		if err != nil || !status.Available {
			continue
		}
		if status.Enabled {
			slog.Info("phát hiện tường lửa đang bật", "backend", candidate.Name())
			return candidate
		}
		if firstAvailable == nil {
			firstAvailable = candidate
		}
	}

	if firstAvailable != nil {
		slog.Info("phát hiện tường lửa đã cài nhưng chưa bật", "backend", firstAvailable.Name())
		return firstAvailable
	}

	slog.Info("không tìm thấy công cụ tường lửa nào khả dụng")
	return NewUnavailable()
}

// Unavailable là trình quản lý giả dùng khi máy chủ không có công cụ tường lửa nào.
//
// Trả về một trình quản lý luôn báo lỗi rõ ràng, thay vì nil — nil sẽ khiến mọi
// điểm gọi phải tự kiểm tra và một chỗ quên là panic.
type Unavailable struct{}

var _ Manager = (*Unavailable)(nil)

// NewUnavailable tạo trình quản lý báo không khả dụng.
func NewUnavailable() *Unavailable { return &Unavailable{} }

// Name trả về tên rỗng vì không có công cụ nào.
func (u *Unavailable) Name() string { return "none" }

// Status luôn báo không khả dụng.
func (u *Unavailable) Status(context.Context) (Status, error) {
	return Status{Backend: "none"}, nil
}

// Enable trả về lỗi không khả dụng.
func (u *Unavailable) Enable(context.Context) error { return ErrNoBackend }

// Disable trả về lỗi không khả dụng.
func (u *Unavailable) Disable(context.Context) error { return ErrNoBackend }

// ListRules trả về lỗi không khả dụng.
func (u *Unavailable) ListRules(context.Context) ([]Rule, error) { return nil, ErrNoBackend }

// AddRule trả về lỗi không khả dụng.
func (u *Unavailable) AddRule(context.Context, Rule) error { return ErrNoBackend }

// DeleteRule trả về lỗi không khả dụng.
func (u *Unavailable) DeleteRule(context.Context, string) error { return ErrNoBackend }
