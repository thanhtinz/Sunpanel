// Package dbx quản lý máy chủ cơ sở dữ liệu MySQL/MariaDB và PostgreSQL.
//
// Panel nói chuyện với cơ sở dữ liệu bằng driver thuần Go thay vì gọi mysql hay
// psql trên dòng lệnh. Nhờ vậy binary vẫn cross-compile được mà không cần cgo,
// và máy chủ không phải cài sẵn công cụ dòng lệnh nào.
package dbx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Các lỗi dùng chung.
var (
	// ErrUnsupportedKind là loại cơ sở dữ liệu không được hỗ trợ.
	ErrUnsupportedKind = errors.New("dbx: loại cơ sở dữ liệu không được hỗ trợ")
	// ErrInvalidIdentifier là tên cơ sở dữ liệu, bảng hoặc người dùng không hợp lệ.
	ErrInvalidIdentifier = errors.New("dbx: tên không hợp lệ")
	// ErrProtected là đối tượng hệ thống, không cho thao tác.
	ErrProtected = errors.New("dbx: đây là đối tượng hệ thống")
)

// Kind là loại máy chủ cơ sở dữ liệu.
type Kind string

// Các loại được hỗ trợ.
const (
	// MySQL dùng chung cho cả MySQL và MariaDB — hai bên tương thích ở mọi câu
	// lệnh mà panel sử dụng.
	MySQL Kind = "mysql"
	// PostgreSQL là máy chủ PostgreSQL.
	PostgreSQL Kind = "postgres"
)

// Valid cho biết loại có được hỗ trợ hay không.
func (k Kind) Valid() bool { return k == MySQL || k == PostgreSQL }

// connectTimeout giới hạn thời gian chờ khi mở kết nối.
const connectTimeout = 10 * time.Second

// Server là thông tin kết nối tới một máy chủ cơ sở dữ liệu.
type Server struct {
	Kind     Kind
	Host     string
	Port     int
	User     string
	Password string
	// Database là cơ sở dữ liệu mặc định để kết nối vào.
	//
	// PostgreSQL bắt buộc phải kết nối vào một cơ sở dữ liệu cụ thể; để trống thì
	// dùng "postgres", vốn luôn tồn tại.
	Database string
}

// validIdentifier khớp tên cơ sở dữ liệu, bảng và người dùng.
//
// Những tên này KHÔNG truyền được qua tham số câu lệnh — "CREATE DATABASE ?"
// không tồn tại — nên chúng phải được ghép vào chuỗi SQL. Đây là bề mặt tấn công
// duy nhất còn lại, vì vậy chỉ chấp nhận đúng những ký tự an toàn thay vì cố
// thoát ký tự đặc biệt.
var validIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$-]{0,62}$`)

// ValidateIdentifier kiểm tra một tên do người dùng nhập.
func ValidateIdentifier(name string) error {
	if !validIdentifier.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidIdentifier, name)
	}
	return nil
}

// systemDatabases là các cơ sở dữ liệu của chính máy chủ, không được xóa.
//
// Xóa nhầm một trong số này làm hỏng cả máy chủ cơ sở dữ liệu chứ không chỉ mất
// một cơ sở dữ liệu.
var systemDatabases = map[string]struct{}{
	"mysql": {}, "information_schema": {}, "performance_schema": {}, "sys": {},
	"postgres": {}, "template0": {}, "template1": {},
}

// IsSystemDatabase cho biết một cơ sở dữ liệu có phải của hệ thống hay không.
func IsSystemDatabase(name string) bool {
	_, ok := systemDatabases[strings.ToLower(name)]
	return ok
}

// DSN dựng chuỗi kết nối cho driver tương ứng.
func (s Server) DSN() (driver, dsn string, err error) {
	switch s.Kind {
	case MySQL:
		database := s.Database
		return "mysql", fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?parseTime=true&timeout=%s&charset=utf8mb4",
			s.User, s.Password, s.Host, s.Port, database, connectTimeout,
		), nil
	case PostgreSQL:
		database := s.Database
		if database == "" {
			database = "postgres"
		}
		// Dùng dạng khóa=giá trị thay vì URL: mật khẩu chứa ký tự đặc biệt không
		// phải mã hóa lại, nên không có chỗ nào để mã hóa sai.
		return "pgx", fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=prefer connect_timeout=%d",
			quotePG(s.Host), s.Port, quotePG(s.User), quotePG(s.Password),
			quotePG(database), int(connectTimeout.Seconds()),
		), nil
	}
	return "", "", fmt.Errorf("%w: %q", ErrUnsupportedKind, s.Kind)
}

// quotePG bọc một giá trị trong chuỗi kết nối PostgreSQL.
func quotePG(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " '\\") {
		return value
	}
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	return "'" + escaped + "'"
}

// Open mở kết nối tới máy chủ.
func (s Server) Open(ctx context.Context) (*sql.DB, error) {
	driver, dsn, err := s.DSN()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("mở kết nối: %w", err)
	}

	// Panel mở kết nối cho từng thao tác rồi đóng ngay, nên giữ một pool lớn chỉ
	// tổ chiếm slot kết nối của máy chủ cơ sở dữ liệu.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("kết nối tới máy chủ: %w", err)
	}
	return db, nil
}

// quoteIdentifier bọc một tên đã qua kiểm tra theo cú pháp của từng máy chủ.
func (s Server) quoteIdentifier(name string) string {
	if s.Kind == MySQL {
		return "`" + name + "`"
	}
	return `"` + name + `"`
}
