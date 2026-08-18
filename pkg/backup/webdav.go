package backup

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// WebDAVConfig là cấu hình một nơi lưu trữ WebDAV.
type WebDAVConfig struct {
	// URL là địa chỉ thư mục lưu trữ, ví dụ
	// https://cloud.vi-du.vn/remote.php/dav/files/nguoidung/sao-luu
	URL      string
	Username string
	Password string
}

// WebDAVDestination lưu bản sao lưu lên máy chủ WebDAV.
//
// WebDAV được hỗ trợ vì đây là cách đơn giản nhất để đẩy bản sao lên Nextcloud
// hay một NAS trong nhà — những nơi mà người dùng cá nhân thực sự có sẵn.
type WebDAVDestination struct {
	config WebDAVConfig
	client *http.Client
}

// NewWebDAV tạo nơi lưu trữ WebDAV.
func NewWebDAV(config WebDAVConfig) *WebDAVDestination {
	return &WebDAVDestination{
		config: config,
		client: &http.Client{Timeout: 2 * time.Hour},
	}
}

// Kind trả về loại nơi lưu trữ.
func (d *WebDAVDestination) Kind() Kind { return WebDAV }

// urlFor dựng địa chỉ đầy đủ của một tệp.
func (d *WebDAVDestination) urlFor(name string) (string, error) {
	base := strings.TrimRight(d.config.URL, "/")
	if name == "" {
		return base + "/", nil
	}
	if err := ValidateName(name); err != nil {
		return "", err
	}
	return base + "/" + url.PathEscape(name), nil
}

// request dựng một yêu cầu đã kèm thông tin đăng nhập.
func (d *WebDAVDestination) request(
	ctx context.Context, method, target string, body io.Reader,
) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("dựng yêu cầu WebDAV: %w", err)
	}
	req.SetBasicAuth(d.config.Username, d.config.Password)
	return req, nil
}

// Put tải một tệp lên WebDAV.
func (d *WebDAVDestination) Put(ctx context.Context, name string, r io.Reader, size int64) error {
	target, err := d.urlFor(name)
	if err != nil {
		return err
	}

	req, err := d.request(ctx, http.MethodPut, target, r)
	if err != nil {
		return err
	}
	// Nhiều máy chủ WebDAV từ chối yêu cầu không biết trước độ dài, nên phải khai
	// báo — nhờ vậy cũng không phải nạp cả tệp vào bộ nhớ như với S3.
	req.ContentLength = size

	res, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("tải tệp lên WebDAV: %w", err)
	}
	defer res.Body.Close()
	return expectOK(res, "tải tệp lên WebDAV")
}

// Get tải một tệp từ WebDAV về.
func (d *WebDAVDestination) Get(ctx context.Context, name string) (io.ReadCloser, error) {
	target, err := d.urlFor(name)
	if err != nil {
		return nil, err
	}

	req, err := d.request(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}

	res, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tải tệp từ WebDAV: %w", err)
	}
	if res.StatusCode == http.StatusNotFound {
		res.Body.Close()
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	if err := expectOK(res, "tải tệp từ WebDAV"); err != nil {
		res.Body.Close()
		return nil, err
	}
	return res.Body, nil
}

// multistatus là phản hồi của lệnh PROPFIND.
type multistatus struct {
	XMLName   xml.Name `xml:"multistatus"`
	Responses []struct {
		Href  string `xml:"href"`
		Props []struct {
			Length       int64  `xml:"prop>getcontentlength"`
			LastModified string `xml:"prop>getlastmodified"`
			IsCollection string `xml:"prop>resourcetype>collection"`
		} `xml:"propstat"`
	} `xml:"response"`
}

// List liệt kê các tệp sao lưu trên WebDAV.
func (d *WebDAVDestination) List(ctx context.Context) ([]Object, error) {
	target, err := d.urlFor("")
	if err != nil {
		return nil, err
	}

	const body = `<?xml version="1.0"?>
<d:propfind xmlns:d="DAV:"><d:prop>
  <d:getcontentlength/><d:getlastmodified/><d:resourcetype/>
</d:prop></d:propfind>`

	req, err := d.request(ctx, "PROPFIND", target, bytes.NewReader([]byte(body)))
	if err != nil {
		return nil, err
	}
	// Depth 1 là "thư mục này và các mục ngay bên trong"; thiếu tiêu đề này thì
	// nhiều máy chủ mặc định duyệt đệ quy toàn bộ cây.
	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml")

	res, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("liệt kê tệp WebDAV: %w", err)
	}
	defer res.Body.Close()
	if err := expectOK(res, "liệt kê tệp WebDAV"); err != nil {
		return nil, err
	}

	var parsed multistatus
	if err := xml.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("đọc danh sách tệp WebDAV: %w", err)
	}

	basePath := strings.TrimRight(mustPath(d.config.URL), "/")
	var out []Object
	for _, response := range parsed.Responses {
		href, err := url.PathUnescape(response.Href)
		if err != nil {
			continue
		}
		// Chính thư mục cũng nằm trong kết quả; bỏ nó đi.
		if strings.TrimRight(href, "/") == basePath {
			continue
		}

		object := Object{Name: path.Base(strings.TrimRight(href, "/"))}
		for _, prop := range response.Props {
			if prop.Length > 0 {
				object.SizeBytes = prop.Length
			}
			if prop.LastModified != "" {
				if parsed, err := time.Parse(time.RFC1123, prop.LastModified); err == nil {
					object.ModTime = parsed
				}
			}
		}
		if err := ValidateName(object.Name); err != nil {
			continue
		}
		out = append(out, object)
	}

	sortObjects(out)
	return out, nil
}

// Delete xóa một tệp trên WebDAV.
func (d *WebDAVDestination) Delete(ctx context.Context, name string) error {
	target, err := d.urlFor(name)
	if err != nil {
		return err
	}

	req, err := d.request(ctx, http.MethodDelete, target, nil)
	if err != nil {
		return err
	}

	res, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("xóa tệp WebDAV: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil
	}
	return expectOK(res, "xóa tệp WebDAV")
}

// Check thử liệt kê thư mục để xác nhận thông tin đăng nhập đúng.
func (d *WebDAVDestination) Check(ctx context.Context) error {
	_, err := d.List(ctx)
	return err
}

// mustPath lấy phần đường dẫn của một URL, trả về chuỗi rỗng nếu URL hỏng.
func mustPath(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Path
}
