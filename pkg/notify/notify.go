// Package notify gửi cảnh báo tới các kênh thông báo.
//
// Panel biết trước ai đó khi máy chủ sắp hết đĩa hay chứng chỉ sắp hết hạn,
// nhưng thông tin đó vô dụng nếu nó chỉ nằm trong giao diện mà không ai mở.
// Vì vậy cảnh báo phải đi ra ngoài: email, Telegram hoặc webhook.
package notify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Các lỗi dùng chung.
var (
	// ErrUnsupportedKind là loại kênh không được hỗ trợ.
	ErrUnsupportedKind = errors.New("notify: loại kênh thông báo không được hỗ trợ")
	// ErrInvalidConfig là cấu hình kênh không hợp lệ.
	ErrInvalidConfig = errors.New("notify: cấu hình kênh không hợp lệ")
	// ErrSendFailed là gửi thông báo thất bại.
	ErrSendFailed = errors.New("notify: gửi thông báo thất bại")
)

// Kind là loại kênh thông báo.
type Kind string

// Các loại kênh được hỗ trợ.
const (
	// Email gửi qua máy chủ SMTP.
	Email Kind = "email"
	// Telegram gửi qua bot Telegram.
	Telegram Kind = "telegram"
	// Webhook gửi một yêu cầu HTTP POST dạng JSON.
	Webhook Kind = "webhook"
)

// Valid cho biết loại kênh có được hỗ trợ hay không.
func (k Kind) Valid() bool { return k == Email || k == Telegram || k == Webhook }

// Level là mức nghiêm trọng của một thông báo.
type Level string

// Các mức nghiêm trọng.
const (
	// LevelInfo là thông tin, không cần hành động.
	LevelInfo Level = "info"
	// LevelWarning là cảnh báo, nên xem sớm.
	LevelWarning Level = "warning"
	// LevelCritical là sự cố, cần xử lý ngay.
	LevelCritical Level = "critical"
)

// Icon trả về biểu tượng đại diện cho mức nghiêm trọng.
//
// Dùng trong tin nhắn Telegram và tiêu đề email: người nhận lướt qua danh sách
// thông báo và cần nhận ra ngay cái nào khẩn cấp.
func (l Level) Icon() string {
	switch l {
	case LevelCritical:
		return "🔴"
	case LevelWarning:
		return "🟠"
	default:
		return "🔵"
	}
}

// Message là một thông báo cần gửi.
type Message struct {
	// Title là tiêu đề ngắn gọn.
	Title string
	// Body là nội dung chi tiết.
	Body string
	// Level là mức nghiêm trọng.
	Level Level
	// Hostname là tên máy chủ phát sinh thông báo.
	//
	// Người quản trị nhiều máy chủ nhận thông báo từ tất cả chúng vào cùng một
	// nơi; thiếu tên máy chủ thì thông báo không nói được vấn đề nằm ở đâu.
	Hostname string
	// At là thời điểm phát sinh.
	At time.Time
	// Fields là các cặp thông tin bổ sung, hiển thị dạng danh sách.
	Fields map[string]string
}

// PlainText dựng nội dung dạng văn bản thuần của thông báo.
func (m Message) PlainText() string {
	var builder strings.Builder
	builder.WriteString(m.Title)
	builder.WriteString("\n\n")
	if m.Body != "" {
		builder.WriteString(m.Body)
		builder.WriteString("\n\n")
	}

	if m.Hostname != "" {
		fmt.Fprintf(&builder, "Máy chủ: %s\n", m.Hostname)
	}
	if !m.At.IsZero() {
		fmt.Fprintf(&builder, "Thời điểm: %s\n", m.At.Format("15:04:05 02/01/2006"))
	}

	for _, key := range sortedKeys(m.Fields) {
		fmt.Fprintf(&builder, "%s: %s\n", key, m.Fields[key])
	}
	return builder.String()
}

// Channel là một kênh gửi thông báo.
type Channel interface {
	// Kind là loại kênh.
	Kind() Kind
	// Send gửi một thông báo.
	Send(ctx context.Context, message Message) error
}

// New dựng kênh thông báo từ cấu hình.
func New(kind Kind, config map[string]string) (Channel, error) {
	switch kind {
	case Email:
		return newEmail(config)
	case Telegram:
		return newTelegram(config)
	case Webhook:
		return newWebhook(config)
	}
	return nil, fmt.Errorf("%w: %q", ErrUnsupportedKind, kind)
}

// sortedKeys trả về các khóa đã sắp xếp, để thông báo có thứ tự ổn định.
func sortedKeys(fields map[string]string) []string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// require lấy một giá trị bắt buộc từ cấu hình.
func require(config map[string]string, key string) (string, error) {
	value := strings.TrimSpace(config[key])
	if value == "" {
		return "", fmt.Errorf("%w: thiếu %s", ErrInvalidConfig, key)
	}
	return value, nil
}
