package dockerx

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// Vòng đời đầy đủ của một container: tải image, tạo, chạy, đọc nhật ký, đọc số
// liệu, dừng, rồi xóa. Đây là bài test chạm vào Docker thật, không phải giả lập.
func TestContainerLifecycle(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	const name = "sunpanel-test-vongdoi"

	image := testImage(t, client)

	// Dọn trước phòng khi lần chạy trước để lại.
	_ = client.RemoveContainer(ctx, name, true)

	id, err := client.CreateContainer(ctx, CreateOptions{
		Name:  name,
		Image: image,
	})
	if err != nil {
		t.Fatalf("tạo container: %v", err)
	}
	t.Cleanup(func() { _ = client.RemoveContainer(context.Background(), name, true) })

	if id == "" {
		t.Fatal("thiếu ID container")
	}

	if err := client.StartContainer(ctx, name); err != nil {
		t.Fatalf("khởi động container: %v", err)
	}

	// Chờ container in xong dòng đầu tiên.
	time.Sleep(2 * time.Second)

	containers, err := client.ListContainers(ctx, false)
	if err != nil {
		t.Fatalf("liệt kê container: %v", err)
	}
	var target *Container
	for i := range containers {
		if containers[i].Name == name {
			target = &containers[i]
		}
	}
	if target == nil {
		t.Fatal("không thấy container vừa khởi động trong danh sách container đang chạy")
	}
	if !target.Running() {
		t.Errorf("trạng thái = %q, mong đợi running", target.State)
	}

	logs, err := client.ContainerLogs(ctx, name, 100)
	if err != nil {
		t.Fatalf("đọc nhật ký: %v", err)
	}
	if !strings.Contains(logs, "xin-chao-tu-sunpanel") {
		t.Errorf("nhật ký = %q, không chứa dòng mong đợi", logs)
	}

	stats, err := client.ContainerStats(ctx, name)
	if err != nil {
		t.Fatalf("đọc số liệu: %v", err)
	}
	if stats.MemoryLimit == 0 {
		t.Error("thiếu giới hạn bộ nhớ trong số liệu")
	}
	t.Logf("số liệu: cpu=%.2f%% mem=%d/%d", stats.CPUPercent, stats.MemoryUsage, stats.MemoryLimit)

	if err := client.RestartContainer(ctx, name, 5); err != nil {
		t.Fatalf("khởi động lại: %v", err)
	}
	if err := client.StopContainer(ctx, name, 5); err != nil {
		t.Fatalf("dừng container: %v", err)
	}

	// Sau khi dừng, container không được nằm trong danh sách đang chạy nhưng vẫn
	// phải thấy khi liệt kê tất cả.
	running, _ := client.ListContainers(ctx, false)
	for _, item := range running {
		if item.Name == name {
			t.Error("container đã dừng vẫn nằm trong danh sách đang chạy")
		}
	}
	all, _ := client.ListContainers(ctx, true)
	stillThere := false
	for _, item := range all {
		if item.Name == name {
			stillThere = true
		}
	}
	if !stillThere {
		t.Error("container đã dừng phải còn khi liệt kê tất cả")
	}

	if err := client.RemoveContainer(ctx, name, false); err != nil {
		t.Fatalf("xóa container: %v", err)
	}

	final, _ := client.ListContainers(ctx, true)
	for _, item := range final {
		if item.Name == name {
			t.Error("container vẫn còn sau khi xóa")
		}
	}
}

// testImage chọn một image có sẵn trên máy để chạy thử.
//
// Bài test không tự tải image: môi trường chạy kiểm thử thường không ra được
// Internet, và một bài test phụ thuộc mạng là bài test hay đỏ vì lý do không
// liên quan tới thứ nó kiểm tra.
func testImage(t *testing.T, client *Client) string {
	t.Helper()

	images, err := client.ListImages(context.Background())
	if err != nil {
		t.Fatalf("liệt kê image: %v", err)
	}
	for _, image := range images {
		for _, tag := range image.Tags {
			if strings.HasPrefix(tag, "sunpanel-test:") {
				return tag
			}
		}
	}

	t.Skip("không có image thử nghiệm trên máy (cần image gắn thẻ sunpanel-test)")
	return ""
}

// Thao tác lên container không tồn tại phải trả lỗi rõ ràng.
func TestOperationsOnMissingContainer(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	const name = "sunpanel-chac-chan-khong-ton-tai"

	if err := client.StartContainer(ctx, name); !errors.Is(err, ErrNotFound) {
		t.Errorf("khởi động: lỗi = %v, mong đợi ErrNotFound", err)
	}
	if _, err := client.ContainerLogs(ctx, name, 10); !errors.Is(err, ErrNotFound) {
		t.Errorf("nhật ký: lỗi = %v, mong đợi ErrNotFound", err)
	}
	if err := client.RemoveContainer(ctx, name, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("xóa: lỗi = %v, mong đợi ErrNotFound", err)
	}
}
