package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// RemoteHost điều khiển một máy chủ khác thông qua agent.
//
// Đây chính là lý do lớp host được tách thành interface ngay từ đầu: toàn bộ
// business logic của panel nhận host.Host, nên chỉ cần đưa RemoteHost vào là
// mọi tính năng chạy được trên máy chủ ở xa mà không phải sửa dòng nào.
type RemoteHost struct {
	name    string
	baseURL string
	token   string
	client  *http.Client
	fs      *remoteFS
}

var _ host.Host = (*RemoteHost)(nil)

// Options là tùy chọn kết nối tới agent.
type Options struct {
	// Name là tên hiển thị của node.
	Name string
	// BaseURL là địa chỉ agent, ví dụ https://10.0.0.5:9528
	BaseURL string
	// Token là bí mật xác thực với agent.
	Token string
	// SkipVerify bỏ qua việc kiểm chứng chỉ của agent.
	//
	// Cần cho agent dùng chứng chỉ tự ký — trường hợp phổ biến nhất khi nối các
	// máy chủ trong cùng mạng nội bộ. Kênh vẫn được mã hóa; thứ mất đi là khả
	// năng chống kẻ đứng giữa, nên chỉ nên bật trong mạng tin cậy.
	SkipVerify bool
	// Timeout là hạn thời gian mặc định cho mỗi lời gọi.
	Timeout time.Duration
}

// NewRemoteHost tạo host điều khiển máy chủ ở xa.
func NewRemoteHost(options Options) *RemoteHost {
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: options.SkipVerify},
	}
	remote := &RemoteHost{
		name:    options.Name,
		baseURL: strings.TrimRight(options.BaseURL, "/"),
		token:   options.Token,
		client:  &http.Client{Timeout: timeout, Transport: transport},
	}
	remote.fs = &remoteFS{remote: remote}
	return remote
}

// Name trả về tên node.
func (h *RemoteHost) Name() string {
	if h.name == "" {
		return "remote"
	}
	return h.name
}

// Close giải phóng tài nguyên.
func (h *RemoteHost) Close() error {
	h.client.CloseIdleConnections()
	return nil
}

// call gửi một yêu cầu tới agent và giải mã phản hồi.
func (h *RemoteHost) call(ctx context.Context, path string, payload, out any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("dựng yêu cầu: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	method := http.MethodGet
	if payload != nil {
		method = http.MethodPost
	}

	req, err := http.NewRequestWithContext(ctx, method, h.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("dựng yêu cầu: %w", err)
	}
	req.Header.Set(HeaderToken, h.token)
	req.Header.Set(HeaderVersion, APIVersion)
	req.Header.Set("Content-Type", "application/json")

	res, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("gọi agent %s: %w", h.Name(), err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var failure ErrorResponse
		// Agent có thể trả về thứ không phải JSON; đã có mã trạng thái làm nội
		// dung thay thế bên dưới.
		_ = json.NewDecoder(io.LimitReader(res.Body, 8000)).Decode(&failure)
		if failure.Error == "" {
			failure.Error = res.Status
		}
		return fmt.Errorf("agent %s: %s", h.Name(), failure.Error)
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("đọc phản hồi agent: %w", err)
	}
	return nil
}

// Ping kiểm tra agent còn sống và đúng phiên bản.
func (h *RemoteHost) Ping(ctx context.Context) (PingResponse, error) {
	var out PingResponse
	if err := h.call(ctx, PathPing, nil, &out); err != nil {
		return PingResponse{}, err
	}
	if out.Version != APIVersion {
		return out, fmt.Errorf(
			"phiên bản giao thức không khớp: panel %s, agent %s", APIVersion, out.Version)
	}
	return out, nil
}

// Exec chạy một lệnh trên máy chủ ở xa.
func (h *RemoteHost) Exec(ctx context.Context, cmd host.Command) (host.Result, error) {
	req := ExecRequest{
		Path: cmd.Path, Args: cmd.Args, Dir: cmd.Dir,
		Env: cmd.Env, Timeout: cmd.Timeout, Stdin: string(cmd.Stdin),
	}

	var out ExecResponse
	if err := h.call(ctx, PathExec, req, &out); err != nil {
		return host.Result{}, err
	}
	return out.Result(), nil
}

// Info đọc thông tin hệ thống của máy chủ ở xa.
func (h *RemoteHost) Info(ctx context.Context) (host.SystemInfo, error) {
	var out host.SystemInfo
	if err := h.call(ctx, PathInfo, nil, &out); err != nil {
		return host.SystemInfo{}, err
	}
	return out, nil
}

// FS trả về lớp truy cập hệ thống tệp của máy chủ ở xa.
func (h *RemoteHost) FS() host.FileSystem { return h.fs }

// remoteFS thực hiện các thao tác tệp qua agent.
type remoteFS struct {
	remote *RemoteHost
}

var _ host.FileSystem = (*remoteFS)(nil)

// Root trả về thư mục gốc mà agent cho phép.
//
// Phạm vi thật do agent quyết định; panel không tự nới rộng được, nên giá trị ở
// đây chỉ mang tính hiển thị.
func (f *remoteFS) Root() string { return "/" }

func (f *remoteFS) List(ctx context.Context, path string) ([]host.FileInfo, error) {
	var out FSResponse
	if err := f.remote.call(ctx, PathFS, FSRequest{Action: FSList, Path: path}, &out); err != nil {
		return nil, err
	}
	return out.Entries, nil
}

func (f *remoteFS) Stat(ctx context.Context, path string) (host.FileInfo, error) {
	var out FSResponse
	if err := f.remote.call(ctx, PathFS, FSRequest{Action: FSStat, Path: path}, &out); err != nil {
		return host.FileInfo{}, err
	}
	if out.Info == nil {
		return host.FileInfo{}, fmt.Errorf("agent không trả về thông tin tệp")
	}
	return *out.Info, nil
}

func (f *remoteFS) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	var out FSResponse
	if err := f.remote.call(ctx, PathFS, FSRequest{Action: FSRead, Path: path}, &out); err != nil {
		return nil, err
	}

	content, err := base64.StdEncoding.DecodeString(out.Content)
	if err != nil {
		return nil, fmt.Errorf("giải mã nội dung tệp: %w", err)
	}
	return io.NopCloser(bytes.NewReader(content)), nil
}

func (f *remoteFS) Write(ctx context.Context, path string, r io.Reader, mode os.FileMode) error {
	content, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("đọc dữ liệu ghi: %w", err)
	}

	return f.remote.call(ctx, PathFS, FSRequest{
		Action: FSWrite, Path: path, Mode: uint32(mode),
		Content: base64.StdEncoding.EncodeToString(content),
	}, nil)
}

func (f *remoteFS) Mkdir(ctx context.Context, path string, mode os.FileMode) error {
	return f.remote.call(ctx, PathFS, FSRequest{
		Action: FSMkdir, Path: path, Mode: uint32(mode),
	}, nil)
}

func (f *remoteFS) Remove(ctx context.Context, path string, recursive bool) error {
	return f.remote.call(ctx, PathFS, FSRequest{
		Action: FSRemove, Path: path, Recursive: recursive,
	}, nil)
}

func (f *remoteFS) Move(ctx context.Context, oldPath, newPath string) error {
	return f.remote.call(ctx, PathFS, FSRequest{
		Action: FSMove, Path: oldPath, NewPath: newPath,
	}, nil)
}

func (f *remoteFS) Chmod(ctx context.Context, path string, mode os.FileMode) error {
	return f.remote.call(ctx, PathFS, FSRequest{
		Action: FSChmod, Path: path, Mode: uint32(mode),
	}, nil)
}
