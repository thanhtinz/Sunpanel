//go:build windows

package monitor

import (
	"os"
	"strings"
)

// isRootMount trên Windows là ổ đĩa hệ thống, thường là C:.
func isRootMount(mountpoint string) bool {
	sys := os.Getenv("SystemDrive")
	if sys == "" {
		sys = "C:"
	}
	return strings.EqualFold(strings.TrimSuffix(mountpoint, "\\"), strings.TrimSuffix(sys, "\\"))
}
