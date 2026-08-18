package service

import (
	"errors"
	"testing"
	"time"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
)

func TestDownloadTicketRoundTrip(t *testing.T) {
	issuer := NewTokenIssuer("khoa-ky-test", time.Minute)

	ticket, err := issuer.IssueDownloadTicket(7, "/www/tep.zip")
	if err != nil {
		t.Fatalf("cấp vé: %v", err)
	}

	claims, err := issuer.ParseDownloadTicket(ticket)
	if err != nil {
		t.Fatalf("đọc vé: %v", err)
	}
	if claims.Path != "/www/tep.zip" {
		t.Errorf("đường dẫn = %q", claims.Path)
	}
	if claims.UserID != 7 {
		t.Errorf("người dùng = %d, mong đợi 7", claims.UserID)
	}
}

// Access token thường không được dùng thay cho vé tải xuống, dù hai loại dùng
// chung khóa ký — nếu không, một access token bị lộ sẽ đọc được mọi tệp qua URL.
func TestAccessTokenIsNotAValidTicket(t *testing.T) {
	issuer := NewTokenIssuer("khoa-ky-test", time.Minute)

	access, _, err := issuer.Issue(&model.User{ID: 1, Username: "admin", Role: model.RoleAdmin}, 1)
	if err != nil {
		t.Fatalf("cấp access token: %v", err)
	}

	if _, err := issuer.ParseDownloadTicket(access); !errors.Is(err, apperr.Unauthorized) {
		t.Fatalf("lỗi = %v, mong đợi Unauthorized", err)
	}
}

// Vé ký bằng khóa khác phải bị từ chối.
func TestTicketFromOtherKeyRejected(t *testing.T) {
	issuer := NewTokenIssuer("khoa-that", time.Minute)
	attacker := NewTokenIssuer("khoa-gia", time.Minute)

	forged, err := attacker.IssueDownloadTicket(1, "/etc/shadow")
	if err != nil {
		t.Fatalf("cấp vé giả: %v", err)
	}

	if _, err := issuer.ParseDownloadTicket(forged); !errors.Is(err, apperr.Unauthorized) {
		t.Fatalf("lỗi = %v, mong đợi Unauthorized", err)
	}
}
