// Package appstore mô tả và cài đặt các ứng dụng cài sẵn một chạm.
//
// Mỗi ứng dụng là một tệp YAML gồm khuôn Docker Compose và danh sách biến mà
// người dùng phải điền. Giá trị người dùng nhập KHÔNG bao giờ được ghép thẳng
// vào khuôn compose: chúng đi qua tệp .env riêng, còn khuôn compose tham chiếu
// tới chúng bằng ${TÊN_BIẾN}. Ghép chuỗi trực tiếp sẽ cho phép một giá trị chứa
// xuống dòng chèn thêm chỉ thị vào compose — mà compose thì cấu hình được cả
// container đặc quyền lẫn gắn thư mục gốc của máy chủ.
package appstore

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Các lỗi dùng chung.
var (
	// ErrNotFound là không tìm thấy ứng dụng trong danh mục.
	ErrNotFound = errors.New("appstore: không tìm thấy ứng dụng")
	// ErrInvalidApp là định nghĩa ứng dụng không hợp lệ.
	ErrInvalidApp = errors.New("appstore: định nghĩa ứng dụng không hợp lệ")
	// ErrInvalidValue là giá trị người dùng nhập không hợp lệ.
	ErrInvalidValue = errors.New("appstore: giá trị không hợp lệ")
	// ErrMissingValue là thiếu giá trị bắt buộc.
	ErrMissingValue = errors.New("appstore: thiếu giá trị bắt buộc")
)

// Text là một chuỗi có bản dịch.
//
// Danh mục ứng dụng nằm ở phía máy chủ nên không dùng được bảng dịch của giao
// diện; mỗi chuỗi mang sẵn các bản dịch của nó.
type Text struct {
	VI string `yaml:"vi" json:"vi"`
	EN string `yaml:"en" json:"en"`
}

// FieldType là kiểu của một biến cấu hình.
type FieldType string

// Các kiểu biến được hỗ trợ.
const (
	// FieldText là chuỗi tự do.
	FieldText FieldType = "text"
	// FieldPassword là mật khẩu; giao diện che đi và có thể sinh ngẫu nhiên.
	FieldPassword FieldType = "password"
	// FieldPort là số hiệu cổng.
	FieldPort FieldType = "port"
	// FieldNumber là số nguyên.
	FieldNumber FieldType = "number"
	// FieldSelect là lựa chọn trong danh sách cho trước.
	FieldSelect FieldType = "select"
)

// Option là một lựa chọn của biến kiểu select.
type Option struct {
	Value string `yaml:"value" json:"value"`
	Label Text   `yaml:"label" json:"label"`
}

// Field là một biến cấu hình mà người dùng điền khi cài ứng dụng.
type Field struct {
	// Key là tên biến trong tệp .env, viết HOA_GACH_DUOI.
	Key   string    `yaml:"key" json:"key"`
	Label Text      `yaml:"label" json:"label"`
	Help  Text      `yaml:"help" json:"help"`
	Type  FieldType `yaml:"type" json:"type"`
	// Default là giá trị gợi ý sẵn.
	Default string `yaml:"default" json:"default"`
	// Required bắt buộc phải có giá trị.
	Required bool `yaml:"required" json:"required"`
	// Generate sinh ngẫu nhiên khi người dùng để trống; chỉ dùng với mật khẩu.
	Generate bool `yaml:"generate" json:"generate"`
	// Min và Max giới hạn giá trị số.
	Min int `yaml:"min" json:"min"`
	Max int `yaml:"max" json:"max"`
	// Options là danh sách lựa chọn của kiểu select.
	Options []Option `yaml:"options" json:"options"`
}

// App là một ứng dụng trong danh mục.
type App struct {
	// Key là định danh, dùng làm tên thư mục cài đặt.
	Key         string `yaml:"key" json:"key"`
	Name        Text   `yaml:"name" json:"name"`
	Description Text   `yaml:"description" json:"description"`
	// Category dùng để lọc trong giao diện.
	Category string `yaml:"category" json:"category"`
	// Version là phiên bản ứng dụng mà định nghĩa này cài.
	Version string `yaml:"version" json:"version"`
	// Website là trang chủ của ứng dụng.
	Website string `yaml:"website" json:"website"`
	// Icon là biểu trưng dạng SVG, giao diện hiển thị qua data URI.
	//
	// Để trống thì panel tự lấy tệp icons/<định danh>.svg trong cùng danh mục.
	// Giao diện nhúng nó vào thẻ <img> chứ không chèn thẳng vào trang: danh mục
	// tự thêm của quản trị viên cũng là dữ liệu ngoài, và một tệp SVG chèn thẳng
	// vào trang thì chạy được mã kịch bản.
	Icon string `yaml:"icon" json:"icon"`
	// Images liệt kê các image cần tải, để giao diện báo trước dung lượng sẽ tải.
	Images []string `yaml:"images" json:"images"`
	// PortField là tên biến chứa cổng chính, dùng để dựng liên kết mở ứng dụng.
	PortField string  `yaml:"portField" json:"portField"`
	Fields    []Field `yaml:"fields" json:"fields"`
	// Compose là khuôn tệp docker-compose.yml, chỉ tham chiếu biến qua ${TÊN}.
	Compose string `yaml:"compose" json:"-"`
}

// ReservedVariables là các biến do chính panel điền, không do người dùng nhập.
//
// Khuôn compose dùng được chúng như biến thường, nhưng chúng không xuất hiện
// trong biểu mẫu cài đặt — người dùng không nên đặt tên container hay thư mục
// dữ liệu, vì panel cần biết chắc chúng để quản lý được ứng dụng về sau.
var ReservedVariables = map[string]string{
	"CONTAINER_NAME": "tên container chính, panel đặt theo định danh ứng dụng",
	"APP_KEY":        "định danh ứng dụng",
	"DATA_DIR":       "thư mục dữ liệu của ứng dụng trên máy chủ",
}

// validKey khớp định danh ứng dụng, vốn được dùng làm tên thư mục.
var validKey = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// validFieldKey khớp tên biến môi trường.
var validFieldKey = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

// Validate kiểm tra một định nghĩa ứng dụng.
//
// Chạy cho mọi ứng dụng lúc nạp danh mục: một định nghĩa hỏng phải lộ ra ngay
// khi khởi động chứ không phải lúc người dùng bấm cài.
func (a App) Validate() error {
	if !validKey.MatchString(a.Key) {
		return fmt.Errorf("%w: định danh %q", ErrInvalidApp, a.Key)
	}
	if strings.TrimSpace(a.Compose) == "" {
		return fmt.Errorf("%w: %s thiếu khuôn compose", ErrInvalidApp, a.Key)
	}
	if a.Name.VI == "" || a.Name.EN == "" {
		return fmt.Errorf("%w: %s thiếu tên ở một ngôn ngữ", ErrInvalidApp, a.Key)
	}

	seen := make(map[string]struct{}, len(a.Fields))
	for _, field := range a.Fields {
		if !validFieldKey.MatchString(field.Key) {
			return fmt.Errorf("%w: %s có biến %q không hợp lệ", ErrInvalidApp, a.Key, field.Key)
		}
		if _, duplicate := seen[field.Key]; duplicate {
			return fmt.Errorf("%w: %s khai báo trùng biến %q", ErrInvalidApp, a.Key, field.Key)
		}
		seen[field.Key] = struct{}{}

		switch field.Type {
		case FieldText, FieldPassword, FieldPort, FieldNumber:
		case FieldSelect:
			if len(field.Options) == 0 {
				return fmt.Errorf("%w: %s biến %q kiểu select nhưng không có lựa chọn",
					ErrInvalidApp, a.Key, field.Key)
			}
		default:
			return fmt.Errorf("%w: %s biến %q có kiểu %q không hỗ trợ",
				ErrInvalidApp, a.Key, field.Key, field.Type)
		}
	}

	if a.PortField != "" {
		if _, ok := seen[a.PortField]; !ok {
			return fmt.Errorf("%w: %s trỏ portField tới biến %q không tồn tại",
				ErrInvalidApp, a.Key, a.PortField)
		}
	}

	// Mọi biến khuôn compose dùng đều phải được khai báo, nếu không compose sẽ
	// thay bằng chuỗi rỗng và container chạy với cấu hình trống rỗng.
	for _, name := range composeVariables(a.Compose) {
		if _, reserved := ReservedVariables[name]; reserved {
			continue
		}
		if _, ok := seen[name]; !ok {
			return fmt.Errorf("%w: %s dùng biến %q chưa khai báo", ErrInvalidApp, a.Key, name)
		}
	}
	return nil
}

// composeVarPattern khớp tham chiếu biến trong khuôn compose.
var composeVarPattern = regexp.MustCompile(`\$\{([A-Z][A-Z0-9_]*)\}`)

// composeVariables liệt kê các biến mà khuôn compose tham chiếu.
func composeVariables(template string) []string {
	matches := composeVarPattern.FindAllStringSubmatch(template, -1)
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if _, ok := seen[match[1]]; ok {
			continue
		}
		seen[match[1]] = struct{}{}
		out = append(out, match[1])
	}
	return out
}

// Resolve kiểm tra giá trị người dùng nhập và bổ sung mặc định.
//
// Trả về bộ giá trị đầy đủ cho mọi biến của ứng dụng.
func (a App) Resolve(input map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(a.Fields))

	for _, field := range a.Fields {
		if _, reserved := ReservedVariables[field.Key]; reserved {
			return nil, fmt.Errorf("%w: %s dùng tên biến dành riêng %q",
				ErrInvalidApp, a.Key, field.Key)
		}
		value := strings.TrimSpace(input[field.Key])

		if value == "" && field.Generate {
			generated, err := generateSecret()
			if err != nil {
				return nil, err
			}
			value = generated
		}
		if value == "" {
			value = field.Default
		}
		if value == "" && field.Required {
			return nil, fmt.Errorf("%w: %s", ErrMissingValue, field.Key)
		}

		if err := validateValue(field, value); err != nil {
			return nil, err
		}
		out[field.Key] = value
	}
	return out, nil
}

// validateValue kiểm tra một giá trị theo kiểu của biến.
func validateValue(field Field, value string) error {
	// Giá trị đi vào tệp .env, nơi mỗi dòng là một cặp khóa=giá trị. Một giá trị
	// chứa xuống dòng sẽ tạo thêm biến, và biến đó có thể ghi đè cấu hình của
	// container — kể cả các cấu hình cho phép thoát khỏi container.
	if strings.ContainsAny(value, "\n\r\x00") {
		return fmt.Errorf("%w: %s chứa ký tự xuống dòng", ErrInvalidValue, field.Key)
	}

	switch field.Type {
	case FieldPort:
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("%w: %s không phải số hiệu cổng hợp lệ", ErrInvalidValue, field.Key)
		}
	case FieldNumber:
		number, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%w: %s không phải số", ErrInvalidValue, field.Key)
		}
		if field.Min != 0 || field.Max != 0 {
			if number < field.Min || number > field.Max {
				return fmt.Errorf("%w: %s phải nằm trong khoảng %d-%d",
					ErrInvalidValue, field.Key, field.Min, field.Max)
			}
		}
	case FieldSelect:
		for _, option := range field.Options {
			if option.Value == value {
				return nil
			}
		}
		return fmt.Errorf("%w: %s không nằm trong danh sách lựa chọn", ErrInvalidValue, field.Key)
	}
	return nil
}

// generateSecret sinh một chuỗi bí mật ngẫu nhiên.
func generateSecret() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("sinh chuỗi bí mật: %w", err)
	}
	// base64 không dấu để giá trị dùng được trong URL và trong tệp .env mà không
	// cần trích dẫn.
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// RenderEnv dựng nội dung tệp .env từ bộ giá trị.
func RenderEnv(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sortStrings(keys)

	var builder strings.Builder
	builder.WriteString("# Tệp này do SunPanel sinh ra khi cài ứng dụng.\n")
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(values[key])
		builder.WriteString("\n")
	}
	return builder.String()
}

// sortStrings sắp xếp tại chỗ, giữ thứ tự ổn định giữa các lần cài để so sánh
// hai tệp .env còn có nghĩa.
func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
