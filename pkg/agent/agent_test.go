package agent

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// startAgent dựng một agent thật trên HTTPS và trả về RemoteHost trỏ vào nó.
//
// Chạy qua TLS chứ không phải HTTP thường: token đi qua kênh không mã hóa thì
// coi như không có token, nên đường đi thật phải được kiểm.
func startAgent(t *testing.T, root string, allowed []string) (*RemoteHost, *httptest.Server) {
	t.Helper()

	local := host.NewLocalHost(root, allowed)
	server := httptest.NewTLSServer(NewServer(local, "token-bi-mat", "thu-nghiem").Handler())
	t.Cleanup(server.Close)

	remote := NewRemoteHost(Options{
		Name: "node-thu", BaseURL: server.URL, Token: "token-bi-mat", SkipVerify: true,
	})
	t.Cleanup(func() { remote.Close() })
	return remote, server
}

func TestRemoteHostSatisfiesHostInterface(t *testing.T) {
	// Đây là điều kiện để mọi tính năng của panel chạy được trên máy chủ ở xa mà
	// không phải sửa business logic; kiểm ngay ở mức biên dịch.
	var _ host.Host = (*RemoteHost)(nil)
}

func TestPingAndInfo(t *testing.T) {
	remote, _ := startAgent(t, t.TempDir(), nil)
	ctx := context.Background()

	ping, err := remote.Ping(ctx)
	if err != nil {
		t.Fatalf("ping agent: %v", err)
	}
	if ping.Version != APIVersion {
		t.Errorf("phiên bản giao thức sai: %s", ping.Version)
	}
	if ping.Hostname == "" {
		t.Error("ping không trả về tên máy")
	}
	t.Logf("agent %s trên máy %s", ping.AgentVersion, ping.Hostname)

	info, err := remote.Info(ctx)
	if err != nil {
		t.Fatalf("đọc thông tin hệ thống: %v", err)
	}
	if info.OS == "" || info.Arch == "" {
		t.Errorf("thông tin hệ thống thiếu: %+v", info)
	}
}

// Token sai phải bị từ chối; agent chạy quyền root nên đây là hàng rào duy nhất
// giữa mạng nội bộ và toàn bộ máy chủ.
func TestWrongTokenIsRejected(t *testing.T) {
	_, server := startAgent(t, t.TempDir(), nil)

	remote := NewRemoteHost(Options{
		BaseURL: server.URL, Token: "token-sai", SkipVerify: true,
	})
	defer remote.Close()

	if _, err := remote.Ping(context.Background()); err == nil {
		t.Fatal("token sai phải bị từ chối")
	} else if !strings.Contains(err.Error(), "token") {
		t.Errorf("thông báo lỗi chưa nói rõ lý do: %v", err)
	}
}

func TestRemoteExec(t *testing.T) {
	remote, _ := startAgent(t, "/", []string{"echo", "sh"})
	ctx := context.Background()

	result, err := remote.Exec(ctx, host.Command{Path: "echo", Args: []string{"xin chào từ node"}})
	if err != nil {
		t.Fatalf("chạy lệnh: %v", err)
	}
	if !result.OK() {
		t.Fatalf("lệnh thất bại: %s", result.Stderr)
	}
	// Dấu tiếng Việt phải đi qua JSON nguyên vẹn.
	if strings.TrimSpace(result.Stdout) != "xin chào từ node" {
		t.Errorf("đầu ra sai: %q", result.Stdout)
	}

	// Lệnh ngoài allowlist phải bị agent từ chối, không phải panel.
	if _, err := remote.Exec(ctx, host.Command{Path: "rm", Args: []string{"-rf", "/"}}); err == nil {
		t.Error("lệnh ngoài allowlist phải bị từ chối")
	}
}

func TestRemoteFileOperations(t *testing.T) {
	root := t.TempDir()
	remote, _ := startAgent(t, root, nil)
	ctx := context.Background()
	fs := remote.FS()

	if err := fs.Mkdir(ctx, "/thu-muc", 0o755); err != nil {
		t.Fatalf("tạo thư mục: %v", err)
	}

	const content = "nội dung có dấu tiếng Việt\nvà xuống dòng\x00cùng byte nhị phân"
	if err := fs.Write(ctx, "/thu-muc/tep.txt", strings.NewReader(content), 0o644); err != nil {
		t.Fatalf("ghi tệp: %v", err)
	}

	reader, err := fs.Open(ctx, "/thu-muc/tep.txt")
	if err != nil {
		t.Fatalf("đọc tệp: %v", err)
	}
	got, _ := io.ReadAll(reader)
	reader.Close()
	// base64 giữ nguyên cả dấu tiếng Việt lẫn byte nhị phân qua JSON.
	if string(got) != content {
		t.Errorf("nội dung đọc lại khác nội dung đã ghi: %q", got)
	}

	entries, err := fs.List(ctx, "/thu-muc")
	if err != nil {
		t.Fatalf("liệt kê: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "tep.txt" {
		t.Errorf("danh sách sai: %+v", entries)
	}

	info, err := fs.Stat(ctx, "/thu-muc/tep.txt")
	if err != nil {
		t.Fatalf("xem thông tin: %v", err)
	}
	if info.Size != int64(len(content)) {
		t.Errorf("kích thước sai: %d, kỳ vọng %d", info.Size, len(content))
	}

	if err := fs.Move(ctx, "/thu-muc/tep.txt", "/thu-muc/moi.txt"); err != nil {
		t.Fatalf("di chuyển: %v", err)
	}
	if _, err := fs.Stat(ctx, "/thu-muc/moi.txt"); err != nil {
		t.Errorf("tệp sau khi đổi tên không tồn tại: %v", err)
	}

	if err := fs.Chmod(ctx, "/thu-muc/moi.txt", 0o600); err != nil {
		t.Fatalf("đổi quyền: %v", err)
	}
	stat, _ := os.Stat(filepath.Join(root, "thu-muc", "moi.txt"))
	if stat.Mode().Perm() != 0o600 {
		t.Errorf("quyền trên đĩa thật là %v", stat.Mode().Perm())
	}

	if err := fs.Remove(ctx, "/thu-muc", true); err != nil {
		t.Fatalf("xóa: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "thu-muc")); !os.IsNotExist(err) {
		t.Error("thư mục vẫn còn sau khi xóa")
	}
}

// Phạm vi thư mục của agent phải được giữ qua giao thức: panel không được vượt
// qua nó bằng cách gửi một đường dẫn thoát ra ngoài.
func TestRemotePathTraversalIsBlocked(t *testing.T) {
	root := t.TempDir()
	remote, _ := startAgent(t, root, nil)
	ctx := context.Background()

	secret := filepath.Join(filepath.Dir(root), "bi-mat.txt")
	if err := os.WriteFile(secret, []byte("không được đọc"), 0o600); err != nil {
		t.Fatalf("tạo tệp ngoài phạm vi: %v", err)
	}
	defer os.Remove(secret)

	for _, path := range []string{"../bi-mat.txt", "/../bi-mat.txt", "../../etc/passwd"} {
		if _, err := remote.FS().Open(ctx, path); err == nil {
			t.Errorf("đường dẫn %q phải bị chặn", path)
		}
	}
}

// Hai bên lệch phiên bản mà vẫn cố nói chuyện sẽ hỏng theo cách khó lần ra hơn
// nhiều so với một lỗi rõ ràng.
func TestVersionMismatchIsRejected(t *testing.T) {
	_, server := startAgent(t, t.TempDir(), nil)

	client := server.Client()
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	req, _ := http.NewRequest(http.MethodGet, server.URL+PathPing, nil)
	req.Header.Set(HeaderToken, "token-bi-mat")
	req.Header.Set(HeaderVersion, "999")

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("gọi agent: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("phiên bản lệch phải bị từ chối, nhận %s", res.Status)
	}
}
