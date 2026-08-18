//go:build !windows

package service

import "os"

// shellCommand dựng lệnh chạy một chuỗi shell của tác vụ định kỳ.
//
// Ưu tiên bash vì quản trị viên thường viết cú pháp bash (mảng, [[ ]]), lùi về
// sh trên các hệ tối giản không có bash.
func shellCommand(command string) (string, []string) {
	if _, err := os.Stat("/bin/bash"); err == nil {
		return "/bin/bash", []string{"-c", command}
	}
	return "/bin/sh", []string{"-c", command}
}
