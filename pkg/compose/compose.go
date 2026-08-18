// Package compose điều khiển Docker Compose để cài và gỡ ứng dụng.
//
// Panel dùng công cụ compose có sẵn trên máy thay vì tự dựng lại việc điều phối
// nhiều container: một ứng dụng thật thường gồm vài container, mạng và volume
// phụ thuộc lẫn nhau, và compose đã giải bài toán đó từ lâu.
package compose

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// Các lỗi dùng chung.
var (
	// ErrUnavailable là máy chủ không có Docker Compose.
	ErrUnavailable = errors.New("compose: không tìm thấy Docker Compose")
	// ErrCommandFailed là lệnh compose kết thúc với mã lỗi.
	ErrCommandFailed = errors.New("compose: lệnh thất bại")
)

// Thời gian tối đa cho các thao tác.
const (
	// detectTimeout là thời gian dò công cụ compose.
	detectTimeout = 10 * time.Second
	// upTimeout là thời gian tối đa cho một lần cài.
	//
	// Rộng rãi vì lần cài đầu phải tải image, mà image của một ứng dụng đầy đủ
	// có thể tới vài trăm megabyte trên đường truyền chậm.
	upTimeout = 30 * time.Minute
	// shortTimeout dùng cho các thao tác không phải tải image.
	shortTimeout = 5 * time.Minute
)

// Compose chạy các lệnh Docker Compose.
type Compose struct {
	host host.Host
	// binary và baseArgs là cách gọi compose trên máy này, dò một lần lúc dùng.
	binary     string
	baseArgs   []string
	detectOnce sync.Once
	available  bool
}

// New tạo trình điều khiển compose.
func New(h host.Host) *Compose { return &Compose{host: h} }

// detect tìm cách gọi compose trên máy hiện tại.
//
// Bản mới là plugin "docker compose"; bản cũ là chương trình riêng
// "docker-compose". Nhiều máy chủ đang chạy vẫn dùng bản cũ, nên phải thử cả hai.
func (c *Compose) detect(ctx context.Context) {
	c.detectOnce.Do(func() {
		ctx, cancel := context.WithTimeout(ctx, detectTimeout)
		defer cancel()

		candidates := []struct {
			binary string
			args   []string
		}{
			{"docker", []string{"compose"}},
			{"docker-compose", nil},
		}
		for _, candidate := range candidates {
			args := append(append([]string{}, candidate.args...), "version")
			res, err := c.host.Exec(ctx, host.Command{Path: candidate.binary, Args: args})
			if err == nil && res.OK() {
				c.binary, c.baseArgs, c.available = candidate.binary, candidate.args, true
				return
			}
		}
	})
}

// Available cho biết máy chủ có dùng được compose không.
func (c *Compose) Available(ctx context.Context) bool {
	c.detect(ctx)
	return c.available
}

// Version trả về chuỗi phiên bản của công cụ compose.
func (c *Compose) Version(ctx context.Context) (string, error) {
	res, err := c.run(ctx, "", detectTimeout, "version", "--short")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(res), nil
}

// run chạy một lệnh compose trong thư mục dự án.
func (c *Compose) run(
	ctx context.Context, dir string, timeout time.Duration, args ...string,
) (string, error) {
	c.detect(ctx)
	if !c.available {
		return "", ErrUnavailable
	}

	full := append(append([]string{}, c.baseArgs...), args...)
	res, err := c.host.Exec(ctx, host.Command{
		Path: c.binary, Args: full, Dir: dir, Timeout: timeout,
	})
	if err != nil {
		return "", fmt.Errorf("chạy %s %s: %w", c.binary, strings.Join(full, " "), err)
	}
	if !res.OK() {
		// Đầu ra lỗi của compose là thứ duy nhất cho biết vì sao ứng dụng không
		// lên được, nên phải giữ nguyên chứ không nuốt đi.
		return res.Stdout, fmt.Errorf("%w: %s", ErrCommandFailed, trimOutput(res.Stderr))
	}
	return res.Stdout + res.Stderr, nil
}

// Up tải image và khởi động toàn bộ dịch vụ của một dự án.
func (c *Compose) Up(ctx context.Context, dir string) (string, error) {
	return c.run(ctx, dir, upTimeout, "up", "-d", "--remove-orphans")
}

// Down dừng và gỡ mọi thứ của một dự án.
//
// removeVolumes quyết định có xóa luôn dữ liệu hay không — mặc định giữ lại, vì
// gỡ nhầm một ứng dụng còn cứu được, mất dữ liệu thì không.
func (c *Compose) Down(ctx context.Context, dir string, removeVolumes bool) (string, error) {
	args := []string{"down", "--remove-orphans"}
	if removeVolumes {
		args = append(args, "--volumes")
	}
	return c.run(ctx, dir, shortTimeout, args...)
}

// Start khởi động lại các dịch vụ đã dừng.
func (c *Compose) Start(ctx context.Context, dir string) (string, error) {
	return c.run(ctx, dir, shortTimeout, "start")
}

// Stop dừng các dịch vụ mà không gỡ container.
func (c *Compose) Stop(ctx context.Context, dir string) (string, error) {
	return c.run(ctx, dir, shortTimeout, "stop")
}

// Restart khởi động lại các dịch vụ.
func (c *Compose) Restart(ctx context.Context, dir string) (string, error) {
	return c.run(ctx, dir, shortTimeout, "restart")
}

// Logs đọc nhật ký gần nhất của toàn bộ dự án.
func (c *Compose) Logs(ctx context.Context, dir string, lines int) (string, error) {
	if lines <= 0 {
		lines = 200
	}
	return c.run(ctx, dir, shortTimeout, "logs", "--tail", fmt.Sprint(lines), "--no-color")
}

// Config kiểm tra tệp compose và trả về cấu hình sau khi thay biến.
//
// Dùng để kiểm tra trước khi cài: một tệp compose sai cú pháp phải bị phát hiện
// trước khi panel ghi nhận là đã cài ứng dụng.
func (c *Compose) Config(ctx context.Context, dir string) (string, error) {
	return c.run(ctx, dir, shortTimeout, "config")
}

// PS liệt kê trạng thái các dịch vụ dưới dạng JSON.
func (c *Compose) PS(ctx context.Context, dir string) (string, error) {
	return c.run(ctx, dir, shortTimeout, "ps", "--format", "json", "--all")
}

// maxOutput giới hạn độ dài thông báo lỗi giữ lại.
const maxOutput = 4000

func trimOutput(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxOutput {
		return value
	}
	return value[:maxOutput] + "…"
}
