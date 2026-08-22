// Package web nhúng giao diện đã build vào binary và phục vụ nó.
//
// Nhờ vậy toàn bộ panel là một tệp thực thi duy nhất: không cần cài Node,
// không cần triển khai thư mục tĩnh riêng, chỉ cần copy một tệp là chạy được.
package web

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// distFS chứa kết quả build của frontend.
//
// Tệp .gitkeep bảo đảm thư mục luôn tồn tại để lệnh go:embed không thất bại khi
// chưa ai chạy build frontend — lập trình viên backend vẫn làm việc được ngay.
//
//go:embed all:dist
var distFS embed.FS

// indexFile là trang đơn của giao diện.
const indexFile = "index.html"

// init khai kiểu nội dung cho tệp khai báo ứng dụng.
//
// Thư viện chuẩn không biết đuôi .webmanifest và trả về text/plain; trình duyệt
// bỏ qua tệp khai báo có kiểu đó, và panel mất luôn khả năng cài thành ứng dụng
// mà không báo lỗi ở đâu cả.
func init() {
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

// Register gắn các tuyến phục vụ giao diện vào engine.
//
// entryPath được chèn vào HTML để mã phía trình duyệt biết tiền tố bí mật của
// mình mà gọi API cho đúng.
func Register(engine *gin.Engine, entryPath string) error {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return fmt.Errorf("mở thư mục giao diện đã nhúng: %w", err)
	}

	index, err := fs.ReadFile(sub, indexFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Chưa build frontend: backend vẫn chạy và phục vụ API bình thường,
			// chỉ hiển thị hướng dẫn thay cho giao diện.
			engine.NoRoute(func(c *gin.Context) {
				if isAPIPath(c.Request.URL.Path) {
					c.AbortWithStatus(http.StatusNotFound)
					return
				}
				placeholderHandler(c)
			})
			return nil
		}
		return fmt.Errorf("đọc %s: %w", indexFile, err)
	}

	page := injectBase(string(index), entryPath)
	fileServer := http.FileServer(http.FS(sub))

	engine.NoRoute(func(c *gin.Context) {
		// Đường dẫn API không tồn tại phải trả 404 chứ không phải trang giao diện,
		// nếu không thì lỗi gõ nhầm endpoint sẽ hiện ra dưới dạng HTML khó hiểu.
		if isAPIPath(c.Request.URL.Path) {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}

		name := strings.TrimPrefix(path.Clean(c.Request.URL.Path), "/")
		if name != "" && name != indexFile {
			if f, err := sub.Open(name); err == nil {
				_ = f.Close()
				switch {
				// Tài nguyên build có tên chứa mã băm nội dung nên vĩnh viễn không đổi.
				case strings.HasPrefix(name, "assets/"):
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				// Hai tệp này quyết định ứng dụng đã cài chạy phiên bản nào; để
				// trình duyệt giữ lại bản cũ nghĩa là một bản cập nhật panel không
				// bao giờ tới được máy người dùng.
				case name == "sw.js" || name == "manifest.webmanifest":
					c.Header("Cache-Control", "no-cache")
				}
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}

		// Mọi đường dẫn còn lại thuộc về bộ định tuyến phía trình duyệt.
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(page))
	})

	return nil
}

// injectBase đặt thẻ <base> để tài nguyên và đường dẫn tương đối phân giải đúng
// khi panel chạy sau một đường dẫn bí mật.
func injectBase(html, entryPath string) string {
	base := "/"
	if trimmed := strings.Trim(entryPath, "/"); trimmed != "" {
		base = "/" + trimmed + "/"
	}

	tag := fmt.Sprintf(`<base href="%s">`, base)
	if idx := strings.Index(html, "<head>"); idx >= 0 {
		return html[:idx+len("<head>")] + tag + html[idx+len("<head>"):]
	}
	return tag + html
}

// isAPIPath cho biết đường dẫn có thuộc về API hay không.
func isAPIPath(p string) bool {
	return strings.HasPrefix(p, "/api/")
}

func placeholderHandler(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(placeholderHTML))
}

const placeholderHTML = `<!doctype html>
<html lang="vi"><head><meta charset="utf-8"><title>SunPanel</title></head>
<body style="font-family:system-ui;max-width:40rem;margin:4rem auto;line-height:1.6">
<h1>SunPanel</h1>
<p>Giao diện chưa được build vào binary này.</p>
<pre style="background:#f4f4f5;padding:1rem;border-radius:.5rem">make frontend &amp;&amp; make build</pre>
<p>API vẫn hoạt động bình thường tại <code>/api/v1</code>.</p>
</body></html>`
