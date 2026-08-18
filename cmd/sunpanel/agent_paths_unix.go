//go:build !windows

package main

// defaultAgentDataDir là nơi agent lưu token và chứng chỉ.
//
// Tách khỏi thư mục dữ liệu của panel: một máy chủ chỉ chạy agent thì không có
// cơ sở dữ liệu hay khóa chủ nào để lưu.
func defaultAgentDataDir() string { return "/opt/sunpanel-agent" }
