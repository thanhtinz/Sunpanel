//go:build windows

package main

import (
	"os"
	"path/filepath"
)

// defaultAgentDataDir là nơi agent lưu token và chứng chỉ trên Windows.
func defaultAgentDataDir() string {
	if dir := os.Getenv("ProgramData"); dir != "" {
		return filepath.Join(dir, "SunPanelAgent")
	}
	return filepath.Join("C:\\", "ProgramData", "SunPanelAgent")
}
