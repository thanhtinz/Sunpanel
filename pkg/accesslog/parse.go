// Package accesslog đọc nhật ký truy cập của nginx và tóm tắt thành số liệu.
//
// Câu hỏi thường gặp nhất sau khi dựng xong một website là "có ai vào không, và
// vào cái gì". Trả lời nó bằng tay nghĩa là mở SSH rồi ghép một chuỗi awk với
// sort | uniq -c mỗi lần muốn xem; gói này làm đúng chuỗi đó nhưng trả về dữ
// liệu để giao diện vẽ.
package accesslog

import (
	"strconv"
	"strings"
	"time"
)

// Entry là một dòng nhật ký đã tách.
type Entry struct {
	IP        string
	Time      time.Time
	Method    string
	Path      string
	Status    int
	Bytes     int64
	Referrer  string
	UserAgent string
}

// timeLayout là định dạng thời gian của nginx: 10/Oct/2000:13:55:36 -0700.
const timeLayout = "02/Jan/2006:15:04:05 -0700"

// Parse tách một dòng theo định dạng combined của nginx.
//
//	$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent
//	"$http_referer" "$http_user_agent"
//
// Trả về false khi dòng không đúng định dạng thay vì báo lỗi: một tệp nhật ký
// thật luôn có vài dòng rác — bản ghi bị cắt giữa chừng lúc logrotate chạy, hay
// dòng của một log_format khác còn sót lại — và chúng không được phép làm hỏng
// cả bản tóm tắt.
func Parse(line string) (Entry, bool) {
	var entry Entry

	ip, rest, ok := strings.Cut(line, " ")
	if !ok || ip == "" {
		return entry, false
	}
	entry.IP = ip

	open := strings.IndexByte(rest, '[')
	closing := strings.IndexByte(rest, ']')
	if open < 0 || closing < open {
		return entry, false
	}
	stamp, err := time.Parse(timeLayout, rest[open+1:closing])
	if err != nil {
		return entry, false
	}
	entry.Time = stamp

	// Phần còn lại gồm ba chuỗi trong ngoặc kép, xen giữa là mã trạng thái và
	// dung lượng. Tách theo dấu ngoặc chứ không theo khoảng trắng: đường dẫn và
	// chuỗi trình duyệt đều có khoảng trắng bên trong.
	quoted, between := splitQuoted(rest[closing+1:])
	if len(quoted) < 1 || len(between) < 1 {
		return entry, false
	}

	request := strings.Fields(quoted[0])
	if len(request) >= 2 {
		entry.Method, entry.Path = request[0], request[1]
	} else {
		entry.Path = quoted[0]
	}

	numbers := strings.Fields(between[0])
	if len(numbers) >= 1 {
		entry.Status, _ = strconv.Atoi(numbers[0])
	}
	if len(numbers) >= 2 {
		entry.Bytes, _ = strconv.ParseInt(numbers[1], 10, 64)
	}
	if entry.Status == 0 {
		return entry, false
	}

	if len(quoted) >= 2 {
		entry.Referrer = quoted[1]
	}
	if len(quoted) >= 3 {
		entry.UserAgent = quoted[2]
	}
	return entry, true
}

// splitQuoted tách một chuỗi thành các đoạn trong ngoặc kép và các đoạn ngoài.
//
// Nginx thoát dấu ngoặc kép trong dữ liệu người dùng gửi lên thành \" nên dấu
// ngoặc đi sau dấu gạch chéo ngược không được coi là dấu đóng — nếu không, một
// yêu cầu chứa dấu ngoặc sẽ làm lệch toàn bộ các trường phía sau.
func splitQuoted(s string) (quoted, between []string) {
	var current strings.Builder
	inside, escaped := false, false

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case escaped:
			current.WriteByte(c)
			escaped = false
		case c == '\\' && inside:
			escaped = true
		case c == '"':
			if inside {
				quoted = append(quoted, current.String())
			} else if value := strings.TrimSpace(current.String()); value != "" {
				// Khoảng trắng ngăn cách giữa hai chuỗi không phải là dữ liệu; giữ
				// lại thì trường đầu tiên của phần ngoài ngoặc luôn là chuỗi rỗng.
				between = append(between, value)
			}
			current.Reset()
			inside = !inside
		default:
			current.WriteByte(c)
		}
	}
	if inside {
		// Dòng bị cắt giữa chừng: phần dở dang không đáng tin, bỏ đi.
		return quoted, between
	}
	if value := strings.TrimSpace(current.String()); value != "" {
		between = append(between, value)
	}
	return quoted, between
}
