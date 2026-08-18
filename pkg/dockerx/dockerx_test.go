package dockerx

import (
	"context"
	"strings"
	"testing"
)

// newTestClient trả về client thật, bỏ qua bài test nếu Docker không chạy.
func newTestClient(t *testing.T) *Client {
	t.Helper()

	client := NewClient()
	if !client.Available(context.Background()) {
		t.Skipf("Docker không khả dụng tại %s", client.SocketPath())
	}
	return client
}

func TestAvailableAndInfo(t *testing.T) {
	client := newTestClient(t)

	info, err := client.Info(context.Background())
	if err != nil {
		t.Fatalf("đọc thông tin Docker: %v", err)
	}
	if info.Version == "" {
		t.Error("thiếu phiên bản Docker")
	}
	if info.StorageDriver == "" {
		t.Error("thiếu tên storage driver")
	}
	t.Logf("Docker %s, driver %s, %d container (%d đang chạy), %d image",
		info.Version, info.StorageDriver, info.Containers, info.ContainersRunning, info.Images)
}

func TestListResources(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	if _, err := client.ListContainers(ctx, true); err != nil {
		t.Errorf("liệt kê container: %v", err)
	}
	if _, err := client.ListImages(ctx); err != nil {
		t.Errorf("liệt kê image: %v", err)
	}
	if _, err := client.ListVolumes(ctx); err != nil {
		t.Errorf("liệt kê volume: %v", err)
	}

	// Docker luôn có sẵn mạng bridge, host và none.
	networks, err := client.ListNetworks(ctx)
	if err != nil {
		t.Fatalf("liệt kê mạng: %v", err)
	}
	if len(networks) == 0 {
		t.Error("Docker phải có ít nhất các mạng mặc định")
	}
}

// Định danh đi thẳng vào đường dẫn URL gửi tới Docker, mà Docker có quyền tương
// đương root — chấp nhận chuỗi tùy ý là cho phép gọi sang endpoint khác.
func TestValidateIDRejectsPathInjection(t *testing.T) {
	invalid := []string{
		"", "../containers/other/kill", "abc/../../info", "abc?force=true",
		"abc&all=1", "abc/json", "-flag", "abc def", "abc\x00",
	}

	for _, id := range invalid {
		t.Run(id, func(t *testing.T) {
			if err := validateID(id); err == nil {
				t.Fatalf("định danh %q phải bị từ chối", id)
			}
		})
	}
}

func TestValidateIDAcceptsRealIDs(t *testing.T) {
	valid := []string{
		"a1b2c3d4e5f6",
		"sunpanel-test-container",
		"my_app.1",
		"3f2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c7d6e5f4a3b2c1d0e9f8a7b6c5d4e3f2b",
	}
	for _, id := range valid {
		if err := validateID(id); err != nil {
			t.Errorf("định danh %q phải hợp lệ: %v", id, err)
		}
	}
}

func TestValidateImageRef(t *testing.T) {
	valid := []string{
		"nginx", "nginx:1.25-alpine", "ghcr.io/owner/app:v1",
		"registry.local:5000/team/app:latest",
		"alpine@sha256:abc123",
	}
	for _, ref := range valid {
		if err := validateImageRef(ref); err != nil {
			t.Errorf("tham chiếu %q phải hợp lệ: %v", ref, err)
		}
	}

	invalid := []string{"", "nginx; rm -rf /", "nginx&x=1", "../etc/passwd", "nginx image"}
	for _, ref := range invalid {
		if err := validateImageRef(ref); err == nil {
			t.Errorf("tham chiếu %q phải bị từ chối", ref)
		}
	}
}

// Docker chèn header 8 byte trước mỗi đoạn nhật ký của container không có TTY;
// không bóc lớp này thì nhật ký hiện ra kèm ký tự rác ở đầu mỗi dòng.
func TestDemultiplexLogs(t *testing.T) {
	// Hai khung: stdout "xin chào\n" (9 byte) và stderr "loi\n" (4 byte).
	frame := func(stream byte, payload string) string {
		size := len(payload)
		header := []byte{stream, 0, 0, 0,
			byte(size >> 24), byte(size >> 16), byte(size >> 8), byte(size)}
		return string(header) + payload
	}

	input := frame(1, "xin chào\n") + frame(2, "loi\n")
	got, err := demultiplexLogs(strings.NewReader(input))
	if err != nil {
		t.Fatalf("bóc khung nhật ký: %v", err)
	}
	if want := "xin chào\nloi\n"; got != want {
		t.Errorf("nhật ký = %q, mong đợi %q", got, want)
	}
}

// Container có TTY không đóng khung dữ liệu; nội dung phải được trả nguyên văn.
func TestDemultiplexLogsWithTTY(t *testing.T) {
	input := "day la nhat ky khong dong khung\n"
	got, err := demultiplexLogs(strings.NewReader(input))
	if err != nil {
		t.Fatalf("bóc khung nhật ký: %v", err)
	}
	if got != input {
		t.Errorf("nhật ký = %q, mong đợi %q", got, input)
	}
}

func TestConvertStats(t *testing.T) {
	var raw statsJSON
	raw.CPUStats.CPUUsage.TotalUsage = 2_000_000_000
	raw.PreCPUStats.CPUUsage.TotalUsage = 1_000_000_000
	raw.CPUStats.SystemCPUUsage = 20_000_000_000
	raw.PreCPUStats.SystemCPUUsage = 10_000_000_000
	raw.CPUStats.OnlineCPUs = 4
	raw.MemoryStats.Usage = 300 << 20
	raw.MemoryStats.Limit = 1 << 30
	raw.MemoryStats.Stats = map[string]uint64{"inactive_file": 100 << 20}

	stats := convertStats(raw)

	// (1e9 / 1e10) * 4 nhân 100 = 40%.
	if stats.CPUPercent != 40 {
		t.Errorf("CPU = %v%%, mong đợi 40%%", stats.CPUPercent)
	}
	// Bộ nhớ đệm trang phải bị trừ ra để khớp với "docker stats".
	if want := uint64(200 << 20); stats.MemoryUsage != want {
		t.Errorf("bộ nhớ = %d, mong đợi %d", stats.MemoryUsage, want)
	}
}

// Lần lấy mẫu đầu tiên không có mốc so sánh; không được chia cho 0.
func TestConvertStatsWithoutPreviousSample(t *testing.T) {
	var raw statsJSON
	raw.MemoryStats.Usage = 1 << 20
	raw.MemoryStats.Limit = 1 << 30

	if stats := convertStats(raw); stats.CPUPercent != 0 {
		t.Errorf("CPU = %v, mong đợi 0 khi chưa có mốc so sánh", stats.CPUPercent)
	}
}

func TestShortID(t *testing.T) {
	long := "3f2b1a0c9d8e7f6a5b4c3d2e1f0a9b8c"
	if got := shortID(long); got != "3f2b1a0c9d8e" {
		t.Errorf("shortID = %q", got)
	}
	if got := shortID("abc"); got != "abc" {
		t.Errorf("shortID ngắn = %q", got)
	}
}

func TestContainerName(t *testing.T) {
	if got := containerName([]string{"/sunpanel-web"}); got != "sunpanel-web" {
		t.Errorf("tên = %q", got)
	}
	if got := containerName(nil); got != "" {
		t.Errorf("tên rỗng = %q", got)
	}
}

// Docker báo lỗi tải image NGAY TRONG luồng tiến trình chứ không qua mã trạng
// thái HTTP: một lần tải thất bại vẫn trả về 200 OK. Bỏ qua luồng này nghĩa là
// người dùng thấy "cài đặt xong" trong khi không có gì được cài.
func TestReadProgressStreamDetectsError(t *testing.T) {
	stream := `{"status":"Pulling from library/busybox","id":"latest"}
{"status":"Downloading","progressDetail":{"current":100,"total":1000}}
{"error":"failed to copy: Forbidden","errorDetail":{"message":"failed to copy: Forbidden"}}`

	err := readProgressStream(strings.NewReader(stream))
	if err == nil {
		t.Fatal("lỗi trong luồng tiến trình phải được phát hiện")
	}
	if !strings.Contains(err.Error(), "Forbidden") {
		t.Errorf("lỗi = %v, phải nêu được nguyên nhân thật", err)
	}
}

func TestReadProgressStreamSucceeds(t *testing.T) {
	stream := `{"status":"Pulling from library/busybox","id":"latest"}
{"status":"Download complete"}
{"status":"Status: Downloaded newer image for busybox:latest"}`

	if err := readProgressStream(strings.NewReader(stream)); err != nil {
		t.Fatalf("luồng thành công không được báo lỗi: %v", err)
	}
}

// Luồng rỗng (Docker không gửi gì) không phải là lỗi.
func TestReadProgressStreamEmpty(t *testing.T) {
	if err := readProgressStream(strings.NewReader("")); err != nil {
		t.Fatalf("luồng rỗng: %v", err)
	}
}
