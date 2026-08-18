// Package sysuser quản lý tài khoản đăng nhập của chính hệ điều hành và khóa
// SSH của chúng.
//
// Đây là phần việc mà người quản trị vẫn phải mở SSH để làm: tạo tài khoản cho
// đồng nghiệp, khóa tài khoản của người đã nghỉ, gắn khóa công khai để họ đăng
// nhập không cần mật khẩu.
package sysuser

import (
	"strconv"
	"strings"
)

// User là một tài khoản đăng nhập của hệ điều hành.
type User struct {
	Name    string `json:"name"`
	UID     int    `json:"uid"`
	GID     int    `json:"gid"`
	Comment string `json:"comment"`
	Home    string `json:"home"`
	Shell   string `json:"shell"`
	// Locked là tài khoản đã bị khóa đăng nhập bằng mật khẩu.
	Locked bool `json:"locked"`
	// NoPassword là tài khoản chưa từng đặt mật khẩu.
	NoPassword bool `json:"noPassword"`
	// System đánh dấu tài khoản của hệ thống chứ không phải của người.
	System bool `json:"system"`
	// Groups là các nhóm phụ tài khoản thuộc về.
	Groups []string `json:"groups"`
	// Sudo là tài khoản nằm trong nhóm cấp quyền quản trị.
	Sudo bool `json:"sudo"`
	// Keys là số khóa SSH đang gắn cho tài khoản.
	Keys int `json:"keys"`
}

// systemUIDLimit là mốc phân biệt tài khoản hệ thống với tài khoản của người.
//
// Mọi bản Linux thông dụng đều cấp UID từ 1000 trở lên cho người dùng thật;
// dưới mốc đó là tài khoản do gói phần mềm tạo ra để chạy dịch vụ.
const systemUIDLimit = 1000

// sudoGroups là các nhóm cho phép dùng sudo, tùy dòng phân phối.
var sudoGroups = map[string]bool{"sudo": true, "wheel": true, "admin": true}

// ParsePasswd đọc nội dung /etc/passwd.
//
// Dòng hỏng bị bỏ qua thay vì làm hỏng cả danh sách: một dòng lạ do phần mềm
// khác ghi vào không nên khiến panel không hiện được tài khoản nào.
func ParsePasswd(content string) []User {
	users := make([]User, 0, 32)

	for _, line := range strings.Split(content, "\n") {
		fields := strings.Split(strings.TrimSpace(line), ":")
		if len(fields) < 7 || fields[0] == "" {
			continue
		}

		uid, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		gid, _ := strconv.Atoi(fields[3])

		users = append(users, User{
			Name:    fields[0],
			UID:     uid,
			GID:     gid,
			Comment: strings.TrimRight(fields[4], ","),
			Home:    fields[5],
			Shell:   fields[6],
			// root có UID 0 nhưng là tài khoản người ta thật sự đăng nhập, nên nó
			// không bị xếp vào nhóm tài khoản hệ thống.
			System: uid != 0 && uid < systemUIDLimit,
		})
	}
	return users
}

// ParseShadow đọc /etc/shadow và cho biết tài khoản nào bị khóa.
//
// Trường mật khẩu bắt đầu bằng "!" hoặc "*" nghĩa là không đăng nhập bằng mật
// khẩu được: "!" là bị khóa, "*" là chưa bao giờ đặt mật khẩu. Đọc thẳng tệp
// thay vì gọi `passwd -S` vì đầu ra của lệnh đó đổi theo ngôn ngữ hệ thống.
func ParseShadow(content string) map[string]struct{ Locked, NoPassword bool } {
	out := map[string]struct{ Locked, NoPassword bool }{}

	for _, line := range strings.Split(content, "\n") {
		fields := strings.Split(strings.TrimSpace(line), ":")
		if len(fields) < 2 || fields[0] == "" {
			continue
		}

		hash := fields[1]
		out[fields[0]] = struct{ Locked, NoPassword bool }{
			Locked:     strings.HasPrefix(hash, "!"),
			NoPassword: hash == "" || hash == "*" || hash == "!" || hash == "!!",
		}
	}
	return out
}

// ParseGroups đọc /etc/group và trả về danh sách nhóm phụ theo từng tài khoản.
func ParseGroups(content string) map[string][]string {
	out := map[string][]string{}

	for _, line := range strings.Split(content, "\n") {
		fields := strings.Split(strings.TrimSpace(line), ":")
		if len(fields) < 4 || fields[0] == "" {
			continue
		}
		for _, member := range strings.Split(fields[3], ",") {
			if member = strings.TrimSpace(member); member != "" {
				out[member] = append(out[member], fields[0])
			}
		}
	}
	return out
}

// ValidName kiểm tra tên tài khoản theo đúng giới hạn của useradd.
//
// Tên sai bị lệnh useradd từ chối bằng một thông báo tiếng Anh trên stderr;
// bắt sớm ở đây cho ra lỗi dịch được và không phải chạy lệnh nào cả.
func ValidName(name string) bool {
	if len(name) == 0 || len(name) > 32 {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r == '_':
		case r >= '0' && r <= '9', r == '-':
			// Chữ số và gạch ngang không được đứng đầu.
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
