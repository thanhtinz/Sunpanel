package firewall

import "net"

// isIPOrCIDR cho biết chuỗi có phải địa chỉ IP hoặc dải CIDR hợp lệ hay không.
//
// Giá trị này đi thẳng vào tham số dòng lệnh của công cụ tường lửa, nên phải
// được kiểm chặt: chấp nhận chuỗi tùy ý ở đây là mở đường cho việc chèn thêm cờ
// dòng lệnh vào công cụ.
func isIPOrCIDR(value string) bool {
	if net.ParseIP(value) != nil {
		return true
	}
	_, _, err := net.ParseCIDR(value)
	return err == nil
}
