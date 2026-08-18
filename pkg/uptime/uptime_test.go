package uptime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheckHealthyTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<h1>Xin chào</h1>"))
	}))
	defer server.Close()

	result := NewChecker().Check(context.Background(), Target{URL: server.URL})
	if !result.Up {
		t.Fatalf("trang bình thường mà báo hỏng: %+v", result)
	}
	if result.StatusCode != 200 {
		t.Errorf("mã trạng thái = %d", result.StatusCode)
	}
}

// Mã 4xx và 5xx là hỏng theo mặc định: một trang trả 500 vẫn "trả lời" nhưng
// người dùng cuối thì không xem được gì.
func TestCheckFailsOnErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	result := NewChecker().Check(context.Background(), Target{URL: server.URL})
	if result.Up || !strings.Contains(result.Error, "500") {
		t.Fatalf("kết quả = %+v", result)
	}
}

// Mã mong đợi cho phép theo dõi cả những đường dẫn cố tình trả 401 hay 302.
func TestCheckAcceptsExpectedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	result := NewChecker().Check(context.Background(), Target{URL: server.URL, ExpectedStatus: 401})
	if !result.Up {
		t.Fatalf("kết quả = %+v", result)
	}
}

// Từ khóa bắt được trường hợp khó chịu nhất: máy chủ trả 200 kèm trang lỗi của
// chính ứng dụng, nên theo mã trạng thái thì mọi thứ vẫn "bình thường".
func TestCheckKeyword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("Database connection failed"))
	}))
	defer server.Close()

	checker := NewChecker()
	if result := checker.Check(context.Background(), Target{URL: server.URL, Keyword: "Xin chào"}); result.Up {
		t.Errorf("thiếu từ khóa mà vẫn báo bình thường: %+v", result)
	}
	if result := checker.Check(context.Background(), Target{URL: server.URL, Keyword: "Database"}); !result.Up {
		t.Errorf("có từ khóa mà báo hỏng: %+v", result)
	}
}

func TestCheckTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer server.Close()

	result := NewChecker().Check(context.Background(), Target{URL: server.URL, Timeout: 50 * time.Millisecond})
	if result.Up || result.Error != "quá thời gian chờ" {
		t.Fatalf("kết quả = %+v", result)
	}
}

func TestCheckUnreachableHost(t *testing.T) {
	result := NewChecker().Check(context.Background(), Target{
		URL:     "http://khong-ton-tai.sunpanel-test.invalid",
		Timeout: 2 * time.Second,
	})
	if result.Up {
		t.Fatal("tên miền không tồn tại mà báo bình thường")
	}
	if result.Error == "" {
		t.Error("thiếu lý do thất bại")
	}
}

// Chứng chỉ tự ký là chuyện thường ở dịch vụ nội bộ: mặc định phải báo hỏng,
// còn khi người dùng chọn bỏ qua thì phải kiểm được.
func TestCheckSelfSignedCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	checker := NewChecker()
	if result := checker.Check(context.Background(), Target{URL: server.URL}); result.Up {
		t.Errorf("chứng chỉ tự ký mà vẫn báo bình thường: %+v", result)
	}

	result := checker.Check(context.Background(), Target{URL: server.URL, SkipTLSVerify: true})
	if !result.Up {
		t.Fatalf("bỏ qua kiểm chứng chỉ mà vẫn hỏng: %+v", result)
	}
	if result.CertExpiresIn < 0 {
		t.Error("không đọc được hạn chứng chỉ")
	}
}
