//go:build windows

package service

// shellCommand dựng lệnh chạy một chuỗi shell của tác vụ định kỳ trên Windows.
//
// cmd.exe dùng cờ /c chứ không phải -c như shell Unix.
func shellCommand(command string) (string, []string) {
	return "cmd.exe", []string{"/c", command}
}
