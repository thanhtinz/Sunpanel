package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStripEntryPath(t *testing.T) {
	// Handler đích ghi lại đường dẫn nó thực sự nhận được.
	var seen string
	target := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	handler := StripEntryPath("secret123", target)

	cases := []struct {
		name       string
		path       string
		wantStatus int
		wantSeen   string
	}{
		{"đường dẫn đúng", "/secret123/api/v1/health", http.StatusOK, "/api/v1/health"},
		{"chỉ tiền tố kèm gạch chéo", "/secret123/", http.StatusOK, "/"},
		{"gốc không có tiền tố", "/", http.StatusNotFound, ""},
		{"đoán đường dẫn API", "/api/v1/health", http.StatusNotFound, ""},
		{"tiền tố sai", "/secret124/api/v1/health", http.StatusNotFound, ""},
		{"tiền tố là tiền tố con", "/secret1234/api", http.StatusNotFound, ""},
		{"đường dẫn quản trị quen thuộc", "/admin", http.StatusNotFound, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seen = ""
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))

			if rec.Code != tc.wantStatus {
				t.Errorf("mã trạng thái = %d, mong đợi %d", rec.Code, tc.wantStatus)
			}
			if seen != tc.wantSeen {
				t.Errorf("handler nhận đường dẫn %q, mong đợi %q", seen, tc.wantSeen)
			}
		})
	}
}

// Truy cập đúng tiền tố nhưng thiếu gạch chéo cuối phải được chuyển hướng, để
// các đường dẫn tương đối của giao diện phân giải đúng.
func TestStripEntryPathRedirectsBarePrefix(t *testing.T) {
	handler := StripEntryPath("secret123", http.NotFoundHandler())

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/secret123", nil))

	if rec.Code != http.StatusFound {
		t.Fatalf("mã trạng thái = %d, mong đợi 302", rec.Code)
	}
	if location := rec.Header().Get("Location"); location != "/secret123/" {
		t.Errorf("Location = %q, mong đợi \"/secret123/\"", location)
	}
}

// Không cấu hình đường dẫn bí mật thì handler phải đi thẳng, không đổi gì.
func TestStripEntryPathDisabled(t *testing.T) {
	called := false
	handler := StripEntryPath("", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

	if !called {
		t.Fatal("handler đích phải được gọi khi không bật đường dẫn bí mật")
	}
}

func TestNormalizeEntryPath(t *testing.T) {
	cases := map[string]string{
		"secret":   "/secret",
		"/secret":  "/secret",
		"/secret/": "/secret",
		"  abc  ":  "/abc",
		"":         "",
		"/":        "",
	}
	for input, want := range cases {
		if got := normalizeEntryPath(input); got != want {
			t.Errorf("normalizeEntryPath(%q) = %q, mong đợi %q", input, got, want)
		}
	}
}

func TestParseAcceptLanguage(t *testing.T) {
	cases := map[string]string{
		"vi-VN,vi;q=0.9,en;q=0.8": "vi",
		"en-US,en;q=0.9":          "en",
		"fr-FR,fr;q=0.9,en;q=0.5": "en",
		"ja,ko":                   "",
		"":                        "",
	}
	for header, want := range cases {
		if got := parseAcceptLanguage(header); got != want {
			t.Errorf("parseAcceptLanguage(%q) = %q, mong đợi %q", header, got, want)
		}
	}
}
