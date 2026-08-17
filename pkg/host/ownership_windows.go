//go:build windows

package host

import "os"

// fillOwnership không áp dụng trên Windows — mô hình phân quyền ở đó dựa trên ACL
// chứ không phải UID/GID, sẽ được xử lý riêng khi hỗ trợ đầy đủ Windows.
func fillOwnership(_ *FileInfo, _ os.FileInfo) {}
