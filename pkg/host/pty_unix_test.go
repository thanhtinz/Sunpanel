//go:build !windows

package host

import "testing"

func TestIsUTF8Locale(t *testing.T) {
	cases := map[string]bool{
		"en_US.UTF-8": true,
		"en_US.utf8":  true,
		"C.UTF-8":     true,
		"C.utf8":      true,
		"vi_VN.UTF-8": true,
		"C":           false,
		"POSIX":       false,
		"en_US":       false,
		"":            false,
	}
	for value, want := range cases {
		if got := isUTF8Locale(value); got != want {
			t.Errorf("isUTF8Locale(%q) = %v, mong đợi %v", value, got, want)
		}
	}
}

// Không có biến môi trường nào dùng được thì phải rơi về C.UTF-8, vốn có sẵn
// trên hầu hết hệ thống mà không cần sinh locale.
func TestDetectUTF8LocaleFallback(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LANG", "POSIX")

	if got := detectUTF8Locale(); got != "C.UTF-8" {
		t.Errorf("detectUTF8Locale() = %q, mong đợi \"C.UTF-8\"", got)
	}
}

// Locale UTF-8 mà người quản trị đã đặt sẵn phải được tôn trọng.
func TestDetectUTF8LocaleUsesEnvironment(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LANG", "vi_VN.UTF-8")

	if got := detectUTF8Locale(); got != "vi_VN.UTF-8" {
		t.Errorf("detectUTF8Locale() = %q, mong đợi \"vi_VN.UTF-8\"", got)
	}
}
