// Package uptime kiểm tra một địa chỉ HTTP có còn sống không.
//
// Panel đã biết máy chủ khỏe hay yếu, nhưng thứ người dùng của bạn nhìn thấy
// là website có trả lời hay không — hai chuyện đó lệch nhau thường xuyên: máy
// nhàn rỗi trong khi ứng dụng phía sau đã chết từ nửa tiếng trước.
package uptime

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// maxBodyBytes là lượng nội dung tối đa đọc về khi cần tìm từ khóa.
//
// Đủ để bắt được tiêu đề trang hay một chuỗi trạng thái, và đủ nhỏ để việc theo
// dõi không tự nó thành tải: một trang nặng vài MB được đọc mỗi phút, nhân với
// vài chục mục theo dõi, là băng thông tiêu tốn vào việc không cần thiết.
const maxBodyBytes = 64 << 10

// Target là một mục cần theo dõi.
type Target struct {
	URL string
	// Timeout là thời gian chờ tối đa cho một lần kiểm tra.
	Timeout time.Duration
	// ExpectedStatus là mã trạng thái bắt buộc; 0 nghĩa là chấp nhận mọi mã
	// dưới 400.
	ExpectedStatus int
	// Keyword là chuỗi phải xuất hiện trong nội dung trả về.
	Keyword string
	// SkipTLSVerify bỏ qua việc kiểm chứng chỉ.
	//
	// Có mặt vì rất nhiều dịch vụ nội bộ chạy chứng chỉ tự ký; không có nó thì
	// người dùng sẽ theo dõi bằng http:// và mất luôn phần kiểm tra TLS.
	SkipTLSVerify bool
}

// Result là kết quả một lần kiểm tra.
type Result struct {
	Up         bool          `json:"up"`
	StatusCode int           `json:"statusCode"`
	Latency    time.Duration `json:"-"`
	LatencyMs  int64         `json:"latencyMs"`
	// Error là lý do thất bại, viết cho người đọc chứ không phải cho máy.
	Error string `json:"error,omitempty"`
	// CertExpiresIn là số ngày còn lại của chứng chỉ TLS; -1 nghĩa là không có.
	CertExpiresIn int `json:"certExpiresIn"`
}

// Checker thực hiện các lần kiểm tra.
type Checker struct {
	client         *http.Client
	insecureClient *http.Client
}

// NewChecker tạo bộ kiểm tra dùng chung.
//
// Hai client riêng vì việc bỏ qua kiểm chứng chỉ là thuộc tính của kết nối chứ
// không phải của yêu cầu; dùng chung một client rồi đổi cấu hình TLS giữa chừng
// sẽ ảnh hưởng cả những mục theo dõi khác đang chạy song song.
func NewChecker() *Checker {
	return &Checker{
		client:         &http.Client{},
		insecureClient: &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}, // #nosec G402 -- người dùng chọn có ý thức cho dịch vụ nội bộ
	}
}

// Check kiểm tra một mục và trả về kết quả.
//
// Không bao giờ trả về lỗi: "không kết nối được" chính là kết quả cần ghi lại,
// chứ không phải một sự cố của bên gọi.
func (c *Checker) Check(ctx context.Context, target Target) Result {
	timeout := target.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := Result{CertExpiresIn: -1}
	started := time.Now()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.URL, nil)
	if err != nil {
		result.Error = "địa chỉ không hợp lệ"
		return result
	}
	// Một số nơi chặn yêu cầu không có User-Agent, và một cú 403 vì lý do đó
	// trông y hệt như website đang hỏng.
	request.Header.Set("User-Agent", "SunPanel-Uptime/1")

	client := c.client
	if target.SkipTLSVerify {
		client = c.insecureClient
	}

	response, err := client.Do(request)
	result.Latency = time.Since(started)
	result.LatencyMs = result.Latency.Milliseconds()
	if err != nil {
		result.Error = describeError(err)
		return result
	}
	defer func() { _ = response.Body.Close() }()

	result.StatusCode = response.StatusCode
	if response.TLS != nil && len(response.TLS.PeerCertificates) > 0 {
		remaining := time.Until(response.TLS.PeerCertificates[0].NotAfter)
		result.CertExpiresIn = int(remaining.Hours() / 24)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxBodyBytes))
	if err != nil {
		result.Error = "đọc nội dung thất bại"
		return result
	}

	switch {
	case target.ExpectedStatus > 0 && response.StatusCode != target.ExpectedStatus:
		result.Error = fmt.Sprintf("mã %d, mong %d", response.StatusCode, target.ExpectedStatus)
	case target.ExpectedStatus == 0 && response.StatusCode >= 400:
		result.Error = fmt.Sprintf("mã %d", response.StatusCode)
	case target.Keyword != "" && !strings.Contains(string(body), target.Keyword):
		result.Error = "không thấy từ khóa trong nội dung"
	default:
		result.Up = true
	}
	return result
}

// describeError đổi lỗi mạng thành câu người đọc hiểu được.
//
// Thông báo gốc của Go dài và lặp lại nguyên cả địa chỉ; trên một bảng theo dõi
// thì điều duy nhất cần biết là hỏng ở khâu nào.
func describeError(err error) string {
	message := err.Error()
	switch {
	case strings.Contains(message, "context deadline exceeded"):
		return "quá thời gian chờ"
	case strings.Contains(message, "no such host"):
		return "không phân giải được tên miền"
	case strings.Contains(message, "connection refused"):
		return "máy chủ từ chối kết nối"
	case strings.Contains(message, "certificate"):
		return "chứng chỉ TLS không hợp lệ"
	default:
		return message
	}
}
