package service

import (
	"context"
	"errors"
	"testing"

	"github.com/thanhtinz/sunpanel/internal/apperr"
)

// Khóa tài khoản chỉ chặn người dò đúng một tài khoản. Đây là bài kiểm tra cho
// lớp còn lại: thử mỗi tên đăng nhập một lần thì không tài khoản nào chạm ngưỡng
// khóa, nên chỉ bộ đếm theo địa chỉ mới nhận ra đang có người quét.
func TestLoginBlocksSourceAddressAcrossUsernames(t *testing.T) {
	auth, db := newTestAuth(t)
	seedUser(t, db, "admin")

	for _, name := range []string{"admin", "root", "administrator", "sunpanel"} {
		_, err := auth.Login(context.Background(), LoginRequest{
			Username: name, Password: "mat-khau-sai", IP: "203.0.113.30",
		})
		// Lần sai làm tràn ngưỡng vẫn phải nhận đúng lỗi như mọi lần khác, nếu
		// không thì chính phản hồi nói cho kẻ dò biết họ vừa chạm ngưỡng nào.
		if !errors.Is(err, apperr.InvalidCredentials) {
			t.Fatalf("đăng nhập %q: lỗi = %v, mong InvalidCredentials", name, err)
		}
	}

	block, blocked := auth.Guard().Blocked("203.0.113.30")
	if !blocked {
		t.Fatal("địa chỉ dò rải tên đăng nhập không bị chặn")
	}
	if block.Failures != 4 {
		t.Errorf("số lần sai ghi nhận = %d, mong 4", block.Failures)
	}

	// Một địa chỉ khác vẫn đăng nhập được bình thường.
	if _, err := auth.Login(context.Background(), LoginRequest{
		Username: "admin", Password: testPassword, IP: "203.0.113.31",
	}); err != nil {
		t.Errorf("địa chỉ khác bị vạ lây: %v", err)
	}
}

// Đăng nhập đúng phải xóa bộ đếm, nếu không người gõ nhầm vài lần rồi vào được
// vẫn bị chặn ở lần đăng nhập sau đó.
func TestLoginSuccessClearsFailureCounter(t *testing.T) {
	auth, db := newTestAuth(t)
	seedUser(t, db, "admin")

	for i := 0; i < 2; i++ {
		auth.Login(context.Background(), LoginRequest{
			Username: "admin", Password: "mat-khau-sai", IP: "203.0.113.32",
		})
	}
	if _, err := auth.Login(context.Background(), LoginRequest{
		Username: "admin", Password: testPassword, IP: "203.0.113.32",
	}); err != nil {
		t.Fatalf("đăng nhập đúng thất bại: %v", err)
	}

	for i := 0; i < 3; i++ {
		auth.Login(context.Background(), LoginRequest{
			Username: "admin", Password: "mat-khau-sai", IP: "203.0.113.32",
		})
	}
	if _, blocked := auth.Guard().Blocked("203.0.113.32"); blocked {
		t.Error("bộ đếm không được xóa sau lần đăng nhập đúng")
	}
}

func TestSecurityOverviewAndUnblock(t *testing.T) {
	auth, db := newTestAuth(t)
	seedUser(t, db, "admin")

	security := NewSecurityService(db, auth, 4, 60, 60, NewAuditService(db))

	for i := 0; i < 4; i++ {
		auth.Login(context.Background(), LoginRequest{
			Username: "admin", Password: "mat-khau-sai", IP: "203.0.113.33",
		})
	}

	view, err := security.Overview(context.Background())
	if err != nil {
		t.Fatalf("đọc tổng quan bảo mật: %v", err)
	}
	if !view.Enabled || len(view.Blocks) != 1 || view.Blocks[0].IP != "203.0.113.33" {
		t.Fatalf("danh sách chặn = %+v", view.Blocks)
	}
	if view.FailedLastDay < 4 {
		t.Errorf("số lần hỏng trong ngày = %d, mong ít nhất 4", view.FailedLastDay)
	}
	if len(view.Offenders) != 1 || view.Offenders[0].IP != "203.0.113.33" {
		t.Fatalf("danh sách địa chỉ thử sai = %+v", view.Offenders)
	}
	if view.Offenders[0].Failures < 4 || !view.Offenders[0].Blocked {
		t.Errorf("chi tiết địa chỉ thử sai = %+v", view.Offenders[0])
	}
	if view.Offenders[0].LastUser != "admin" {
		t.Errorf("tên đăng nhập bị thử gần nhất = %q", view.Offenders[0].LastUser)
	}

	actor := AuditEntry{Username: "admin", IP: "127.0.0.1"}
	if err := security.Unblock(context.Background(), "203.0.113.33", actor); err != nil {
		t.Fatalf("gỡ chặn: %v", err)
	}
	if _, blocked := auth.Guard().Blocked("203.0.113.33"); blocked {
		t.Error("địa chỉ vẫn bị chặn sau khi gỡ")
	}

	// Chuỗi không phải địa chỉ IP không được lọt vào danh sách kiểm toán như
	// một lệnh hợp lệ.
	if err := security.Unblock(context.Background(), "không-phải-ip", actor); !errors.Is(err, apperr.BadRequest) {
		t.Errorf("gỡ chặn giá trị rác: lỗi = %v, mong BadRequest", err)
	}
}
