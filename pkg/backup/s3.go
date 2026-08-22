package backup

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// S3Config là cấu hình một nơi lưu trữ tương thích S3.
type S3Config struct {
	// Endpoint là địa chỉ dịch vụ, ví dụ https://s3.ap-southeast-1.amazonaws.com
	// hoặc https://minio.noi-bo.vn.
	Endpoint string
	Region   string
	Bucket   string
	// Prefix là thư mục con trong bucket; để trống thì lưu ở gốc.
	Prefix          string
	AccessKeyID     string
	SecretAccessKey string
	// PathStyle dùng dạng địa chỉ endpoint/bucket thay vì bucket.endpoint.
	//
	// MinIO và phần lớn dịch vụ tự dựng chỉ hỗ trợ dạng này.
	PathStyle bool
}

// S3Destination lưu bản sao lưu lên dịch vụ tương thích S3.
//
// Tự ký yêu cầu theo SigV4 thay vì dùng SDK của AWS: SDK kéo theo hàng chục
// gói phụ thuộc và vài chục megabyte vào binary, trong khi panel chỉ cần đúng
// bốn thao tác PUT, GET, LIST và DELETE.
type S3Destination struct {
	config S3Config
	client *http.Client
}

// NewS3 tạo nơi lưu trữ S3.
func NewS3(config S3Config) *S3Destination {
	if config.Region == "" {
		config.Region = "us-east-1"
	}
	return &S3Destination{
		config: config,
		// Sao lưu có thể là tệp lớn trên đường truyền chậm, nên hạn thời gian phải
		// rộng; nhưng vẫn phải có, nếu không một kết nối treo sẽ giữ tiến trình mãi.
		client: &http.Client{Timeout: 2 * time.Hour},
	}
}

// Kind trả về loại nơi lưu trữ.
func (d *S3Destination) Kind() Kind { return S3 }

// objectKey dựng khóa đầy đủ của một tệp trong bucket.
func (d *S3Destination) objectKey(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	return joinPath(d.config.Prefix, name), nil
}

// endpointURL dựng địa chỉ đầy đủ cho một khóa.
func (d *S3Destination) endpointURL(key string) (*url.URL, error) {
	base, err := url.Parse(strings.TrimRight(d.config.Endpoint, "/"))
	if err != nil {
		return nil, fmt.Errorf("địa chỉ S3 không hợp lệ: %w", err)
	}

	if d.config.PathStyle {
		base.Path = "/" + d.config.Bucket
		if key != "" {
			base.Path += "/" + key
		}
		return base, nil
	}

	base.Host = d.config.Bucket + "." + base.Host
	base.Path = "/"
	if key != "" {
		base.Path += key
	}
	return base, nil
}

// Put tải một tệp lên S3.
func (d *S3Destination) Put(ctx context.Context, name string, r io.Reader, size int64) error {
	key, err := d.objectKey(name)
	if err != nil {
		return err
	}

	// SigV4 cần băm của toàn bộ nội dung để ký, nên phải đọc hết vào bộ nhớ.
	// Đây là giới hạn có thật: bản sao lưu rất lớn sẽ tốn bộ nhớ tương ứng, và
	// khi đó nên dùng nơi lưu trữ trên đĩa rồi tự đồng bộ.
	body, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("đọc dữ liệu sao lưu: %w", err)
	}
	_ = size

	res, err := d.do(ctx, http.MethodPut, key, nil, body)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	return expectOK(res, "tải tệp lên S3")
}

// Get tải một tệp từ S3 về.
func (d *S3Destination) Get(ctx context.Context, name string) (io.ReadCloser, error) {
	key, err := d.objectKey(name)
	if err != nil {
		return nil, err
	}

	res, err := d.do(ctx, http.MethodGet, key, nil, nil)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusNotFound {
		_ = res.Body.Close()
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	if err := expectOK(res, "tải tệp từ S3"); err != nil {
		_ = res.Body.Close()
		return nil, err
	}
	return res.Body, nil
}

// listBucketResult là phản hồi của ListObjectsV2.
type listBucketResult struct {
	XMLName  xml.Name `xml:"ListBucketResult"`
	Contents []struct {
		Key          string    `xml:"Key"`
		Size         int64     `xml:"Size"`
		LastModified time.Time `xml:"LastModified"`
	} `xml:"Contents"`
	IsTruncated           bool   `xml:"IsTruncated"`
	NextContinuationToken string `xml:"NextContinuationToken"`
}

// List liệt kê các tệp sao lưu trong bucket.
func (d *S3Destination) List(ctx context.Context) ([]Object, error) {
	var out []Object
	token := ""

	for {
		query := url.Values{}
		query.Set("list-type", "2")
		if prefix := strings.Trim(d.config.Prefix, "/"); prefix != "" {
			query.Set("prefix", prefix+"/")
		}
		if token != "" {
			query.Set("continuation-token", token)
		}

		res, err := d.do(ctx, http.MethodGet, "", query, nil)
		if err != nil {
			return nil, err
		}
		if err := expectOK(res, "liệt kê tệp trên S3"); err != nil {
			_ = res.Body.Close()
			return nil, err
		}

		var parsed listBucketResult
		err = xml.NewDecoder(res.Body).Decode(&parsed)
		_ = res.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("đọc danh sách tệp S3: %w", err)
		}

		for _, item := range parsed.Contents {
			out = append(out, Object{
				Name:      path.Base(item.Key),
				SizeBytes: item.Size,
				ModTime:   item.LastModified,
			})
		}

		// Bucket có nhiều hơn 1000 tệp được trả về theo nhiều trang; bỏ qua các
		// trang sau nghĩa là chính sách giữ bản sao sẽ không bao giờ dọn tới chúng.
		if !parsed.IsTruncated || parsed.NextContinuationToken == "" {
			break
		}
		token = parsed.NextContinuationToken
	}

	sortObjects(out)
	return out, nil
}

// Delete xóa một tệp trên S3.
func (d *S3Destination) Delete(ctx context.Context, name string) error {
	key, err := d.objectKey(name)
	if err != nil {
		return err
	}

	res, err := d.do(ctx, http.MethodDelete, key, nil, nil)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	// S3 trả 204 khi xóa xong, và cũng trả 204 khi tệp vốn không tồn tại.
	return expectOK(res, "xóa tệp trên S3")
}

// Check thử liệt kê bucket để xác nhận thông tin đăng nhập đúng.
func (d *S3Destination) Check(ctx context.Context) error {
	_, err := d.List(ctx)
	return err
}

// do ký và gửi một yêu cầu tới S3.
func (d *S3Destination) do(
	ctx context.Context, method, key string, query url.Values, body []byte,
) (*http.Response, error) {
	target, err := d.endpointURL(key)
	if err != nil {
		return nil, err
	}
	if query != nil {
		target.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("dựng yêu cầu S3: %w", err)
	}
	if body != nil {
		req.ContentLength = int64(len(body))
	}

	if err := d.sign(req, body); err != nil {
		return nil, err
	}

	res, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gọi S3: %w", err)
	}
	return res, nil
}

// sign ký yêu cầu theo AWS Signature Version 4.
func (d *S3Destination) sign(req *http.Request, body []byte) error {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	payloadHash := sha256Hex(body)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)
	req.Header.Set("Host", req.URL.Host)

	// Chuỗi ký chỉ gồm ba tiêu đề này, và chúng phải được liệt kê theo đúng thứ
	// tự bảng chữ cái — sai thứ tự thì chữ ký không khớp và S3 trả 403.
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := fmt.Sprintf("host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.URL.Host, payloadHash, amzDate)

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI(req.URL.Path),
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
	}, "\n")

	scope := strings.Join([]string{dateStamp, d.config.Region, "s3", "aws4_request"}, "/")
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", amzDate, scope, sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	key := hmacSHA256([]byte("AWS4"+d.config.SecretAccessKey), dateStamp)
	key = hmacSHA256(key, d.config.Region)
	key = hmacSHA256(key, "s3")
	key = hmacSHA256(key, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(key, stringToSign))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		d.config.AccessKeyID, scope, signedHeaders, signature,
	))
	return nil
}

// canonicalURI mã hóa đường dẫn theo cách SigV4 yêu cầu.
//
// Dấu gạch chéo phải giữ nguyên, còn mọi ký tự đặc biệt khác phải được mã hóa —
// url.PathEscape mã hóa cả dấu gạch chéo nên không dùng thẳng được.
func canonicalURI(rawPath string) string {
	if rawPath == "" {
		return "/"
	}
	segments := strings.Split(rawPath, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

// expectOK biến phản hồi lỗi thành error kèm nội dung máy chủ trả về.
func expectOK(res *http.Response, action string) error {
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	// Thông báo lỗi của S3 nằm trong thân phản hồi dạng XML và thường nói rõ vì
	// sao — sai khóa, sai vùng, hay không có quyền.
	detail, _ := io.ReadAll(io.LimitReader(res.Body, 2000))
	return fmt.Errorf("%s thất bại (%s): %s", action, res.Status, strings.TrimSpace(string(detail)))
}
