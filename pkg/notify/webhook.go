package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// webhookChannel gửi thông báo tới một địa chỉ HTTP.
//
// Kênh này để nối vào hệ thống có sẵn của người dùng — Slack, Discord, hay một
// dịch vụ trực tổng đài — mà panel không cần biết trước từng loại.
type webhookChannel struct {
	url    string
	method string
	// headers là các tiêu đề bổ sung, thường dùng để mang khóa xác thực.
	headers map[string]string
	client  *http.Client
}

func newWebhook(config map[string]string) (Channel, error) {
	url, err := require(config, "url")
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("%w: địa chỉ webhook phải bắt đầu bằng http:// hoặc https://", ErrInvalidConfig)
	}

	method := strings.ToUpper(strings.TrimSpace(config["method"]))
	if method == "" {
		method = http.MethodPost
	}

	headers := map[string]string{}
	// Tiêu đề được khai báo dạng "Tên: giá trị", mỗi dòng một tiêu đề.
	for _, line := range strings.Split(config["headers"], "\n") {
		name, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if name != "" && value != "" {
			headers[name] = value
		}
	}

	return &webhookChannel{
		url: url, method: method, headers: headers,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Kind trả về loại kênh.
func (c *webhookChannel) Kind() Kind { return Webhook }

// Send gửi thông báo tới webhook.
func (c *webhookChannel) Send(ctx context.Context, message Message) error {
	payload, err := json.Marshal(map[string]any{
		"title":    message.Title,
		"body":     message.Body,
		"level":    message.Level,
		"hostname": message.Hostname,
		"at":       message.At,
		"fields":   message.Fields,
		// text gộp sẵn toàn bộ nội dung: Slack và Discord đều nhận trường này,
		// nên webhook của chúng dùng được ngay mà không cần cấu hình thêm.
		"text": message.PlainText(),
	})
	if err != nil {
		return fmt.Errorf("%w: dựng nội dung: %w", ErrSendFailed, err)
	}

	req, err := http.NewRequestWithContext(ctx, c.method, c.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("%w: dựng yêu cầu: %w", ErrSendFailed, err)
	}
	req.Header.Set("Content-Type", "application/json")
	for name, value := range c.headers {
		req.Header.Set(name, value)
	}

	res, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: gọi webhook: %w", ErrSendFailed, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 1000))
		return fmt.Errorf("%w: webhook trả %s: %s", ErrSendFailed, res.Status, strings.TrimSpace(string(detail)))
	}
	return nil
}
