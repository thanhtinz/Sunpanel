// Package dockerx là client tối giản cho Docker Engine API.
//
// Panel nói chuyện thẳng với socket của Docker thay vì dùng SDK chính thức. SDK
// kéo theo hàng chục phụ thuộc (OpenTelemetry, gRPC, buildkit) và làm binary
// phình lên đáng kể, trong khi panel chỉ cần một phần nhỏ của API. Đổi lại,
// package này phải tự khai báo các kiểu dữ liệu nó dùng.
package dockerx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Các lỗi dùng chung.
var (
	// ErrUnavailable là không kết nối được tới Docker.
	ErrUnavailable = errors.New("dockerx: không kết nối được tới Docker")
	// ErrNotFound là không tìm thấy đối tượng.
	ErrNotFound = errors.New("dockerx: không tìm thấy đối tượng")
	// ErrConflict là thao tác xung đột với trạng thái hiện tại.
	ErrConflict = errors.New("dockerx: thao tác xung đột với trạng thái hiện tại")
)

// Giới hạn khi làm việc với Docker.
const (
	// defaultSocket là đường dẫn socket mặc định của Docker trên Linux.
	defaultSocket = "/var/run/docker.sock"
	// apiVersion là phiên bản API tối thiểu mà panel yêu cầu. Docker giữ tương
	// thích ngược, nên khai báo một phiên bản cũ vừa đủ sẽ chạy được với cả các
	// bản daemon mới hơn.
	apiVersion = "v1.41"
	// requestTimeout là thời gian tối đa cho một lời gọi thông thường.
	requestTimeout = 30 * time.Second
	// pullTimeout dài hơn nhiều vì tải image có thể mất vài phút.
	pullTimeout = 30 * time.Minute
)

// Client gọi Docker Engine API.
type Client struct {
	http   *http.Client
	socket string
}

// NewClient tạo client nói chuyện với socket của Docker.
//
// Biến môi trường DOCKER_HOST được tôn trọng ở dạng unix:// để panel chạy được
// trong môi trường đặt socket ở nơi khác.
func NewClient() *Client {
	socket := defaultSocket
	if host := os.Getenv("DOCKER_HOST"); strings.HasPrefix(host, "unix://") {
		socket = strings.TrimPrefix(host, "unix://")
	}

	return &Client{
		socket: socket,
		http: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var dialer net.Dialer
					return dialer.DialContext(ctx, "unix", socket)
				},
			},
		},
	}
}

// Available cho biết Docker có đang chạy và panel có quyền truy cập không.
func (c *Client) Available(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return c.do(ctx, http.MethodGet, "/_ping", nil, nil) == nil
}

// SocketPath trả về đường dẫn socket đang dùng, phục vụ chẩn đoán.
func (c *Client) SocketPath() string { return c.socket }

// do gửi một yêu cầu tới Docker và giải mã phản hồi JSON vào out.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, out any) error {
	// Host trong URL chỉ là chỗ giữ chỗ; kết nối thật đi qua unix socket.
	req, err := http.NewRequestWithContext(ctx, method, "http://docker/"+apiVersion+path, body)
	if err != nil {
		return fmt.Errorf("dựng yêu cầu: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer func() { _ = res.Body.Close() }()

	if err := checkStatus(res); err != nil {
		return err
	}
	if out == nil {
		return nil
	}

	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("đọc phản hồi Docker: %w", err)
	}
	return nil
}

// stream gửi yêu cầu và trả về thân phản hồi để bên gọi tự đọc.
//
// Bên gọi có trách nhiệm đóng. Dùng cho log và các luồng dữ liệu dài.
func (c *Client) stream(ctx context.Context, method, path string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, method, "http://docker/"+apiVersion+path, nil)
	if err != nil {
		return nil, fmt.Errorf("dựng yêu cầu: %w", err)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if err := checkStatus(res); err != nil {
		_ = res.Body.Close()
		return nil, err
	}
	return res.Body, nil
}

// checkStatus chuyển mã trạng thái HTTP thành lỗi có ngữ nghĩa.
func checkStatus(res *http.Response) error {
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}

	// Docker trả lỗi dạng {"message": "..."}; lấy ra để thông báo có ích hơn là
	// một con số trạng thái trần trụi.
	var payload struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(io.LimitReader(res.Body, 8<<10)).Decode(&payload)

	message := strings.TrimSpace(payload.Message)
	if message == "" {
		message = res.Status
	}

	switch res.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("%w: %s", ErrNotFound, message)
	case http.StatusConflict, http.StatusNotModified:
		return fmt.Errorf("%w: %s", ErrConflict, message)
	default:
		return fmt.Errorf("docker trả lỗi %d: %s", res.StatusCode, message)
	}
}

// query dựng chuỗi truy vấn từ các cặp khóa-giá trị.
func query(pairs map[string]string) string {
	if len(pairs) == 0 {
		return ""
	}
	values := url.Values{}
	for key, value := range pairs {
		values.Set(key, value)
	}
	return "?" + values.Encode()
}

// readProgressStream đọc luồng tiến trình JSON của Docker và trả về lỗi nếu có.
//
// Đây là điểm dễ sai nhất khi làm việc với Docker: các endpoint tải và build trả
// về 200 OK ngay lập tức rồi báo tiến trình theo từng dòng JSON. Lỗi nằm trong
// một dòng có khóa "error" — bỏ qua luồng này nghĩa là mọi lần tải thất bại đều
// bị coi là thành công, và người dùng thấy "cài đặt xong" trong khi không có gì
// được cài.
func readProgressStream(r io.Reader) error {
	decoder := json.NewDecoder(r)

	for {
		var message struct {
			Error       string `json:"error"`
			ErrorDetail struct {
				Message string `json:"message"`
			} `json:"errorDetail"`
		}

		if err := decoder.Decode(&message); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("đọc luồng tiến trình: %w", err)
		}

		if detail := message.ErrorDetail.Message; detail != "" {
			return fmt.Errorf("docker: %s", detail)
		}
		if message.Error != "" {
			return fmt.Errorf("docker: %s", message.Error)
		}
	}
}
