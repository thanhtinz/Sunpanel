package webserver

import "sort"

// Mẫu quy tắc viết lại đường dẫn ("pseudo-static").
//
// Gần như mọi mã nguồn PHP phổ biến đều cần một khối rewrite riêng thì đường
// dẫn đẹp mới chạy: thiếu nó, WordPress vào trang chủ được nhưng mọi bài viết
// trả về 404. Đây là chỗ người mới dựng website mất nhiều thời gian nhất, và
// cũng là chỗ dễ chép nhầm một đoạn cấu hình lạ trên mạng vào máy chủ của mình.
//
// Mỗi mẫu chỉ thay đúng khối location / — phần PHP, chặn IP hay hỏi mật khẩu
// vẫn do khuôn chung sinh ra, nên bật một mẫu không bao giờ vô hiệu hóa các lớp
// bảo vệ đã đặt.
const (
	// RewriteNone là không dùng mẫu nào.
	RewriteNone = ""
)

// Rewrite là một mẫu quy tắc viết lại.
type Rewrite struct {
	// Key là định danh lưu vào cơ sở dữ liệu và gửi cho giao diện.
	Key string `json:"key"`
	// Body là khối cấu hình thay cho location / mặc định.
	Body string `json:"body"`
	// Note là điều kiện cần biết trước khi bật, ví dụ thư mục gốc phải trỏ đâu.
	Note string `json:"note,omitempty"`
}

// rewrites là toàn bộ mẫu panel biết.
var rewrites = map[string]Rewrite{
	"wordpress": {
		Key: "wordpress",
		Body: `    location / {
        try_files $uri $uri/ /index.php?$args;
    }`,
	},
	"laravel": {
		Key:  "laravel",
		Note: "public",
		Body: `    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }`,
	},
	"thinkphp": {
		Key: "thinkphp",
		Body: `    location / {
        if (!-e $request_filename) {
            rewrite ^(.*)$ /index.php?s=$1 last;
        }
    }`,
	},
	"codeigniter": {
		Key: "codeigniter",
		Body: `    location / {
        try_files $uri $uri/ /index.php?/$request_uri;
    }`,
	},
	"drupal": {
		Key: "drupal",
		Body: `    location / {
        try_files $uri /index.php?$query_string;
    }`,
	},
	"typecho": {
		Key: "typecho",
		Body: `    location / {
        try_files $uri $uri/ /index.php$is_args$args;
    }`,
	},
	"spa": {
		Key: "spa",
		Body: `    location / {
        try_files $uri $uri/ /index.html;
    }`,
	},
}

// Rewrites liệt kê các mẫu, sắp theo định danh.
func Rewrites() []Rewrite {
	out := make([]Rewrite, 0, len(rewrites))
	for _, item := range rewrites {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// RewriteBody trả về khối cấu hình của một mẫu.
//
// Mẫu không tồn tại trả về false thay vì khối rỗng: bỏ qua trong im lặng nghĩa
// là website chạy với quy tắc mặc định trong khi giao diện vẫn hiện tên mẫu mà
// người dùng đã chọn.
func RewriteBody(key string) (string, bool) {
	if key == RewriteNone {
		return "", true
	}
	item, ok := rewrites[key]
	if !ok {
		return "", false
	}
	return item.Body, true
}
