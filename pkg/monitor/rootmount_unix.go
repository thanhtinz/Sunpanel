//go:build !windows

package monitor

// isRootMount cho biết điểm gắn kết có phải phân vùng gốc hay không.
func isRootMount(mountpoint string) bool { return mountpoint == "/" }
