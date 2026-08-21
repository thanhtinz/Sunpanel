package service

import (
	"os"
	"testing"
)

func TestParseMode(t *testing.T) {
	cases := map[string]os.FileMode{
		"0644": 0o644,
		"755":  0o755,
		"600":  0o600,
		"0777": 0o777,
	}
	for input, want := range cases {
		got, err := parseMode(input)
		if err != nil || got != want {
			t.Errorf("parseMode(%q) = %v, %v; mong %v", input, got, err, want)
		}
	}

	// Giá trị lạ phải bị chặn ngay: chuỗi này đi thẳng vào lệnh đổi quyền trên
	// một máy chủ khác, và "0999" hay "rwx" ở đó nghĩa là một quyền bất kỳ.
	for _, bad := range []string{"", "rwx", "0999", "12345678", "-644"} {
		if _, err := parseMode(bad); err == nil {
			t.Errorf("parseMode(%q) lại được chấp nhận", bad)
		}
	}
}
