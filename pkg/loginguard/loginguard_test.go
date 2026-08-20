package loginguard

import (
	"testing"
	"time"
)

func newGuard(t *testing.T, opts Options) *Guard {
	t.Helper()
	if opts.Threshold == 0 {
		opts.Threshold = 3
	}
	if opts.Window == 0 {
		opts.Window = time.Minute
	}
	if opts.Duration == 0 {
		opts.Duration = time.Minute
	}
	return New(opts)
}

func TestGuardBlocksAfterThreshold(t *testing.T) {
	g := newGuard(t, Options{Threshold: 3})

	for i := 1; i < 3; i++ {
		if _, blocked := g.Fail("203.0.113.7", "admin"); blocked {
			t.Fatalf("bị chặn quá sớm ở lần thứ %d", i)
		}
	}

	block, blocked := g.Fail("203.0.113.7", "root")
	if !blocked {
		t.Fatal("không bị chặn khi đã đủ số lần sai")
	}
	if block.Failures != 3 || block.LastUser != "root" {
		t.Errorf("chi tiết lần chặn = %+v", block)
	}

	if _, ok := g.Blocked("203.0.113.7"); !ok {
		t.Error("địa chỉ vừa bị chặn lại không nằm trong danh sách")
	}
	// Một địa chỉ khác không bị vạ lây.
	if _, ok := g.Blocked("203.0.113.8"); ok {
		t.Error("địa chỉ khác cũng bị chặn theo")
	}
}

// Đếm theo nguồn gửi chứ không theo tài khoản: một máy quét thử mỗi tên đăng
// nhập đúng một lần vẫn phải bị chặn, dù không tài khoản nào chạm ngưỡng khóa.
func TestGuardCountsAcrossUsernames(t *testing.T) {
	g := newGuard(t, Options{Threshold: 3})

	g.Fail("198.51.100.4", "admin")
	g.Fail("198.51.100.4", "root")
	if _, blocked := g.Fail("198.51.100.4", "administrator"); !blocked {
		t.Fatal("dò rải tên đăng nhập không bị chặn")
	}
}

// Cửa sổ đếm phải trôi, nếu không thì người gõ nhầm mỗi tháng một lần cũng có
// ngày tự khóa mình ra ngoài.
func TestGuardForgetsOldFailures(t *testing.T) {
	g := newGuard(t, Options{Threshold: 3, Window: 30 * time.Millisecond})

	g.Fail("192.0.2.5", "admin")
	g.Fail("192.0.2.5", "admin")
	time.Sleep(50 * time.Millisecond)

	if _, blocked := g.Fail("192.0.2.5", "admin"); blocked {
		t.Fatal("các lần sai đã quá cũ vẫn bị cộng dồn")
	}
}

func TestGuardSucceedClearsCounter(t *testing.T) {
	g := newGuard(t, Options{Threshold: 3})

	g.Fail("192.0.2.9", "admin")
	g.Fail("192.0.2.9", "admin")
	g.Succeed("192.0.2.9")

	if _, blocked := g.Fail("192.0.2.9", "admin"); blocked {
		t.Fatal("bộ đếm không được xóa sau lần đăng nhập đúng")
	}
}

func TestGuardBlockExpires(t *testing.T) {
	g := newGuard(t, Options{Threshold: 1, Duration: 20 * time.Millisecond})

	if _, blocked := g.Fail("192.0.2.11", "admin"); !blocked {
		t.Fatal("không chặn ở ngưỡng bằng một")
	}
	time.Sleep(40 * time.Millisecond)

	if _, ok := g.Blocked("192.0.2.11"); ok {
		t.Error("lệnh chặn không tự hết hạn")
	}
	if len(g.List()) != 0 {
		t.Error("danh sách vẫn còn mục đã hết hạn")
	}
}

// Danh sách miễn trừ tồn tại để quản trị viên không tự khóa mình ra ngoài khi
// đang thử nghiệm từ chính máy của mình.
func TestGuardSkipsTrusted(t *testing.T) {
	g := newGuard(t, Options{Threshold: 1, Trusted: []string{"10.0.0.0/8"}})

	// 127.0.0.1 không có trong danh sách cấu hình: địa chỉ nội bộ luôn được miễn
	// trừ, vì đó là đường quay lại cuối cùng khi tự khóa mình ra ngoài.
	for _, ip := range []string{"10.1.2.3", "127.0.0.1", "::1"} {
		if _, blocked := g.Fail(ip, "admin"); blocked {
			t.Errorf("địa chỉ tin cậy %s bị chặn", ip)
		}
	}
	if _, blocked := g.Fail("203.0.113.1", "admin"); !blocked {
		t.Error("địa chỉ ngoài danh sách tin cậy không bị chặn")
	}
}

func TestGuardUnblockAndPrune(t *testing.T) {
	g := newGuard(t, Options{Threshold: 1, Window: 20 * time.Millisecond, Duration: 20 * time.Millisecond})

	g.Fail("203.0.113.2", "admin")
	if !g.Unblock("203.0.113.2") {
		t.Fatal("không gỡ được lệnh chặn")
	}
	if g.Unblock("203.0.113.2") {
		t.Error("gỡ chặn một địa chỉ không bị chặn lại báo thành công")
	}

	g.Fail("203.0.113.3", "admin")
	time.Sleep(50 * time.Millisecond)
	if removed := g.Prune(); removed != 1 {
		t.Errorf("số mục được dọn = %d, mong 1", removed)
	}
}

// Ngưỡng bằng 0 là cách tắt tính năng, và phải tắt thật.
func TestGuardDisabled(t *testing.T) {
	g := New(Options{Threshold: 0, Window: time.Minute, Duration: time.Minute})

	for i := 0; i < 50; i++ {
		if _, blocked := g.Fail("203.0.113.4", "admin"); blocked {
			t.Fatal("bộ chặn đã tắt nhưng vẫn chặn")
		}
	}
	if len(g.List()) != 0 {
		t.Error("bộ chặn đã tắt nhưng vẫn có danh sách")
	}
}
