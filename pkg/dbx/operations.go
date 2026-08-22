package dbx

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// queryTimeout giới hạn thời gian một câu truy vấn được chạy.
//
// Một câu SELECT quét toàn bảng trên cơ sở dữ liệu lớn có thể chạy rất lâu và
// giữ kết nối; giới hạn để giao diện không treo vô hạn.
const queryTimeout = 60 * time.Second

// maxRows giới hạn số dòng trả về cho giao diện.
//
// Trả về cả triệu dòng sẽ làm trình duyệt đứng hình và ngốn hết bộ nhớ panel.
const maxRows = 500

// DatabaseInfo là thông tin một cơ sở dữ liệu.
type DatabaseInfo struct {
	Name string `json:"name"`
	// Charset và Collation chỉ có ở MySQL.
	Charset   string `json:"charset,omitempty"`
	Collation string `json:"collation,omitempty"`
	// SizeBytes là dung lượng ước tính; -1 nghĩa là không đọc được.
	SizeBytes int64 `json:"sizeBytes"`
	// System đánh dấu cơ sở dữ liệu của chính máy chủ.
	System bool `json:"system"`
}

// UserInfo là thông tin một tài khoản cơ sở dữ liệu.
type UserInfo struct {
	Name string `json:"name"`
	// Host chỉ có ở MySQL, cho biết tài khoản được phép kết nối từ đâu.
	Host string `json:"host,omitempty"`
	// SuperUser đánh dấu tài khoản có toàn quyền.
	SuperUser bool `json:"superUser"`
}

// TableInfo là thông tin một bảng.
type TableInfo struct {
	Name      string `json:"name"`
	Rows      int64  `json:"rows"`
	SizeBytes int64  `json:"sizeBytes"`
}

// QueryResult là kết quả một câu truy vấn.
type QueryResult struct {
	// Columns là tên các cột; rỗng nếu câu lệnh không trả về dòng nào.
	Columns []string `json:"columns"`
	// Rows là dữ liệu, mỗi phần tử là một dòng.
	Rows [][]any `json:"rows"`
	// RowsAffected là số dòng bị ảnh hưởng với câu lệnh ghi.
	RowsAffected int64 `json:"rowsAffected"`
	// Truncated cho biết kết quả đã bị cắt bớt vì quá dài.
	Truncated bool `json:"truncated"`
	// DurationMS là thời gian chạy tính bằng mili giây.
	DurationMS int64 `json:"durationMs"`
}

// Version đọc phiên bản máy chủ.
func (s Server) Version(ctx context.Context, db *sql.DB) (string, error) {
	var version string
	query := "SELECT VERSION()"
	if s.Kind == PostgreSQL {
		query = "SELECT version()"
	}
	if err := db.QueryRowContext(ctx, query).Scan(&version); err != nil {
		return "", fmt.Errorf("đọc phiên bản: %w", err)
	}
	return version, nil
}

// ListDatabases liệt kê các cơ sở dữ liệu trên máy chủ.
func (s Server) ListDatabases(ctx context.Context, db *sql.DB) ([]DatabaseInfo, error) {
	if s.Kind == MySQL {
		return s.listMySQLDatabases(ctx, db)
	}
	return s.listPostgresDatabases(ctx, db)
}

func (s Server) listMySQLDatabases(ctx context.Context, db *sql.DB) ([]DatabaseInfo, error) {
	// Dung lượng lấy từ information_schema.tables; cơ sở dữ liệu rỗng không có
	// dòng nào ở đó nên phải nối trái để chúng vẫn xuất hiện.
	const query = `
		SELECT s.SCHEMA_NAME, s.DEFAULT_CHARACTER_SET_NAME, s.DEFAULT_COLLATION_NAME,
		       COALESCE(SUM(t.DATA_LENGTH + t.INDEX_LENGTH), 0)
		FROM information_schema.SCHEMATA s
		LEFT JOIN information_schema.TABLES t ON t.TABLE_SCHEMA = s.SCHEMA_NAME
		GROUP BY s.SCHEMA_NAME, s.DEFAULT_CHARACTER_SET_NAME, s.DEFAULT_COLLATION_NAME
		ORDER BY s.SCHEMA_NAME`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("liệt kê cơ sở dữ liệu: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []DatabaseInfo
	for rows.Next() {
		var info DatabaseInfo
		if err := rows.Scan(&info.Name, &info.Charset, &info.Collation, &info.SizeBytes); err != nil {
			return nil, fmt.Errorf("đọc dòng: %w", err)
		}
		info.System = IsSystemDatabase(info.Name)
		out = append(out, info)
	}
	return out, rows.Err()
}

func (s Server) listPostgresDatabases(ctx context.Context, db *sql.DB) ([]DatabaseInfo, error) {
	const query = `
		SELECT datname,
		       pg_encoding_to_char(encoding),
		       datcollate,
		       CASE WHEN has_database_privilege(datname, 'CONNECT')
		            THEN pg_database_size(datname) ELSE 0 END
		FROM pg_database
		WHERE datistemplate = false OR datname IN ('template0', 'template1')
		ORDER BY datname`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("liệt kê cơ sở dữ liệu: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []DatabaseInfo
	for rows.Next() {
		var info DatabaseInfo
		if err := rows.Scan(&info.Name, &info.Charset, &info.Collation, &info.SizeBytes); err != nil {
			return nil, fmt.Errorf("đọc dòng: %w", err)
		}
		info.System = IsSystemDatabase(info.Name)
		out = append(out, info)
	}
	return out, rows.Err()
}

// CreateDatabase tạo một cơ sở dữ liệu mới.
func (s Server) CreateDatabase(ctx context.Context, db *sql.DB, name string) error {
	if err := ValidateIdentifier(name); err != nil {
		return err
	}

	// Tên đã qua ValidateIdentifier rồi mới được trích dẫn theo đúng cú pháp của
	// từng loại máy chủ; CREATE DATABASE không nhận tham số nên nối chuỗi là cách
	// duy nhất.
	quoted := s.quoteIdentifier(name)
	query := "CREATE DATABASE " + quoted //nolint:gosec
	if s.Kind == MySQL {
		// utf8mb4 là bộ mã duy nhất của MySQL chứa đủ tiếng Việt lẫn biểu tượng
		// cảm xúc; utf8 của MySQL chỉ có 3 byte và cắt mất chúng.
		query += " CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"
	}

	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("tạo cơ sở dữ liệu: %w", err)
	}
	return nil
}

// DropDatabase xóa một cơ sở dữ liệu.
func (s Server) DropDatabase(ctx context.Context, db *sql.DB, name string) error {
	if err := ValidateIdentifier(name); err != nil {
		return err
	}
	if IsSystemDatabase(name) {
		return fmt.Errorf("%w: %s", ErrProtected, name)
	}

	if _, err := db.ExecContext(ctx, "DROP DATABASE "+s.quoteIdentifier(name)); err != nil {
		return fmt.Errorf("xóa cơ sở dữ liệu: %w", err)
	}
	return nil
}

// ListUsers liệt kê tài khoản trên máy chủ.
func (s Server) ListUsers(ctx context.Context, db *sql.DB) ([]UserInfo, error) {
	query := `SELECT user, host, IF(Super_priv = 'Y', 1, 0) FROM mysql.user ORDER BY user, host`
	if s.Kind == PostgreSQL {
		query = `SELECT rolname, '', rolsuper::int FROM pg_roles WHERE rolcanlogin ORDER BY rolname`
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("liệt kê tài khoản: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []UserInfo
	for rows.Next() {
		var info UserInfo
		var super int
		if err := rows.Scan(&info.Name, &info.Host, &super); err != nil {
			return nil, fmt.Errorf("đọc dòng: %w", err)
		}
		info.SuperUser = super == 1
		out = append(out, info)
	}
	return out, rows.Err()
}

// CreateUser tạo tài khoản mới và cấp toàn quyền trên một cơ sở dữ liệu.
//
// database để trống thì chỉ tạo tài khoản mà không cấp quyền gì.
func (s Server) CreateUser(ctx context.Context, db *sql.DB, name, password, database, hostPattern string) error {
	if err := ValidateIdentifier(name); err != nil {
		return err
	}
	if database != "" {
		if err := ValidateIdentifier(database); err != nil {
			return err
		}
	}

	if s.Kind == MySQL {
		return s.createMySQLUser(ctx, db, name, password, database, hostPattern)
	}
	return s.createPostgresUser(ctx, db, name, password, database)
}

// validHostPattern khớp phần host của tài khoản MySQL.
//
// Ký tự % là ký tự đại diện hợp lệ của MySQL nên phải cho phép, nhưng chỉ thế và
// các ký tự của tên máy chủ hoặc địa chỉ IP.
var validHostPattern = regexp.MustCompile(`^[A-Za-z0-9_.%:-]{1,60}$`)

func (s Server) createMySQLUser(ctx context.Context, db *sql.DB, name, password, database, hostPattern string) error {
	if hostPattern == "" {
		hostPattern = "%"
	}
	if !validHostPattern.MatchString(hostPattern) {
		return fmt.Errorf("%w: máy chủ nguồn %q", ErrInvalidIdentifier, hostPattern)
	}

	// MySQL không nhận tham số ở câu lệnh DDL, kể cả cho mật khẩu, nên mật khẩu
	// phải được bọc thành hằng SQL — bằng chính hàm QUOTE của máy chủ.
	quotedPassword, err := quoteMySQL(ctx, db, password)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("CREATE USER '%s'@'%s' IDENTIFIED BY %s", name, hostPattern, quotedPassword)
	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("tạo tài khoản: %w", err)
	}

	if database == "" {
		return nil
	}
	grant := fmt.Sprintf("GRANT ALL PRIVILEGES ON %s.* TO '%s'@'%s'",
		s.quoteIdentifier(database), name, hostPattern)
	if _, err := db.ExecContext(ctx, grant); err != nil {
		return fmt.Errorf("cấp quyền: %w", err)
	}
	return nil
}

func (s Server) createPostgresUser(ctx context.Context, db *sql.DB, name, password, database string) error {
	// PostgreSQL không nhận tham số ở câu lệnh DDL, kể cả cho mật khẩu, nên mật
	// khẩu phải được bọc bằng hàm trích dẫn chuẩn của chính PostgreSQL.
	quotedPassword, err := quoteLiteral(ctx, db, password)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD %s", s.quoteIdentifier(name), quotedPassword)
	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("tạo tài khoản: %w", err)
	}

	if database == "" {
		return nil
	}
	grant := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s",
		s.quoteIdentifier(database), s.quoteIdentifier(name))
	if _, err := db.ExecContext(ctx, grant); err != nil {
		return fmt.Errorf("cấp quyền: %w", err)
	}

	return s.grantPostgresSchema(ctx, name, database)
}

// grantPostgresSchema cấp quyền trên schema public của một cơ sở dữ liệu.
//
// Từ PostgreSQL 15, quyền trên cơ sở dữ liệu KHÔNG còn kéo theo quyền tạo bảng
// trong schema public. Thiếu bước này thì tài khoản mới đăng nhập được nhưng mọi
// lệnh CREATE TABLE đều báo "permission denied for schema public" — và người
// dùng không hiểu vì sao ứng dụng vừa cài lại không chạy.
//
// Quyền schema phải cấp từ bên trong chính cơ sở dữ liệu đó, nên phải mở một
// kết nối riêng.
func (s Server) grantPostgresSchema(ctx context.Context, name, database string) error {
	target := s
	target.Database = database

	db, err := target.Open(ctx)
	if err != nil {
		return fmt.Errorf("kết nối vào %s để cấp quyền schema: %w", database, err)
	}
	defer func() { _ = db.Close() }()

	statements := []string{
		fmt.Sprintf("GRANT ALL ON SCHEMA public TO %s", s.quoteIdentifier(name)),
		// Bảng và chuỗi tạo sau này cũng phải thuộc quyền của tài khoản đó, nếu
		// không mỗi lần ứng dụng tạo bảng mới lại phải cấp quyền lại bằng tay.
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO %s",
			s.quoteIdentifier(name)),
		fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO %s",
			s.quoteIdentifier(name)),
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("cấp quyền schema: %w", err)
		}
	}
	return nil
}

// quoteMySQL nhờ chính MySQL bọc một chuỗi thành hằng SQL an toàn.
//
// Cùng lý do với quoteLiteral của PostgreSQL: tự viết hàm thoát dấu nháy là chỗ
// dễ sai nhất, còn hàm QUOTE của máy chủ xử lý đúng theo cấu hình của nó.
func quoteMySQL(ctx context.Context, db *sql.DB, value string) (string, error) {
	var quoted string
	if err := db.QueryRowContext(ctx, "SELECT QUOTE(?)", value).Scan(&quoted); err != nil {
		return "", fmt.Errorf("bọc giá trị: %w", err)
	}
	return quoted, nil
}

// quoteLiteral nhờ chính PostgreSQL bọc một chuỗi thành hằng SQL an toàn.
//
// Tự viết hàm thoát dấu nháy là chỗ dễ sai nhất; quote_literal của PostgreSQL
// xử lý đúng cả dấu nháy lẫn dấu chéo ngược theo đúng cấu hình của máy chủ.
func quoteLiteral(ctx context.Context, db *sql.DB, value string) (string, error) {
	var quoted string
	if err := db.QueryRowContext(ctx, "SELECT quote_literal($1)", value).Scan(&quoted); err != nil {
		return "", fmt.Errorf("bọc giá trị: %w", err)
	}
	return quoted, nil
}

// DropUser xóa một tài khoản.
func (s Server) DropUser(ctx context.Context, db *sql.DB, name, hostPattern string) error {
	if err := ValidateIdentifier(name); err != nil {
		return err
	}
	// Xóa tài khoản mà panel đang dùng để kết nối sẽ khiến panel mất luôn đường
	// vào máy chủ cơ sở dữ liệu.
	if strings.EqualFold(name, s.User) {
		return fmt.Errorf("%w: %s là tài khoản panel đang dùng", ErrProtected, name)
	}
	if strings.EqualFold(name, "root") || strings.EqualFold(name, "postgres") {
		return fmt.Errorf("%w: %s", ErrProtected, name)
	}

	var query string
	if s.Kind == MySQL {
		if hostPattern == "" {
			hostPattern = "%"
		}
		if !validHostPattern.MatchString(hostPattern) {
			return fmt.Errorf("%w: máy chủ nguồn %q", ErrInvalidIdentifier, hostPattern)
		}
		query = fmt.Sprintf("DROP USER '%s'@'%s'", name, hostPattern)
	} else {
		query = "DROP ROLE " + s.quoteIdentifier(name)
	}

	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("xóa tài khoản: %w", err)
	}
	return nil
}

// ChangePassword đổi mật khẩu một tài khoản.
func (s Server) ChangePassword(ctx context.Context, db *sql.DB, name, password, hostPattern string) error {
	if err := ValidateIdentifier(name); err != nil {
		return err
	}

	if s.Kind == MySQL {
		if hostPattern == "" {
			hostPattern = "%"
		}
		if !validHostPattern.MatchString(hostPattern) {
			return fmt.Errorf("%w: máy chủ nguồn %q", ErrInvalidIdentifier, hostPattern)
		}
		quotedPassword, err := quoteMySQL(ctx, db, password)
		if err != nil {
			return err
		}
		query := fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY %s", name, hostPattern, quotedPassword)
		if _, err := db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("đổi mật khẩu: %w", err)
		}
		return nil
	}

	quotedPassword, err := quoteLiteral(ctx, db, password)
	if err != nil {
		return err
	}
	query := fmt.Sprintf("ALTER ROLE %s PASSWORD %s", s.quoteIdentifier(name), quotedPassword)
	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("đổi mật khẩu: %w", err)
	}
	return nil
}

// ListTables liệt kê bảng trong một cơ sở dữ liệu.
func (s Server) ListTables(ctx context.Context, db *sql.DB, database string) ([]TableInfo, error) {
	if err := ValidateIdentifier(database); err != nil {
		return nil, err
	}

	var query string
	if s.Kind == MySQL {
		query = `SELECT TABLE_NAME, COALESCE(TABLE_ROWS, 0),
		         COALESCE(DATA_LENGTH + INDEX_LENGTH, 0)
		         FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? ORDER BY TABLE_NAME`
	} else {
		// Ở PostgreSQL, kết nối đã nằm sẵn trong cơ sở dữ liệu cần xem, nên tham
		// số chỉ dùng để lọc schema.
		query = `SELECT relname, COALESCE(n_live_tup, 0), pg_total_relation_size(relid)
		         FROM pg_stat_user_tables WHERE schemaname = $1 ORDER BY relname`
	}

	argument := any(database)
	if s.Kind == PostgreSQL {
		argument = "public"
	}

	rows, err := db.QueryContext(ctx, query, argument)
	if err != nil {
		return nil, fmt.Errorf("liệt kê bảng: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []TableInfo
	for rows.Next() {
		var info TableInfo
		if err := rows.Scan(&info.Name, &info.Rows, &info.SizeBytes); err != nil {
			return nil, fmt.Errorf("đọc dòng: %w", err)
		}
		out = append(out, info)
	}
	return out, rows.Err()
}

// Query chạy một câu lệnh SQL tùy ý và trả về kết quả.
//
// Đây là công cụ ở mức của quản trị viên cơ sở dữ liệu: câu lệnh chạy nguyên văn,
// kể cả DROP hay DELETE. Panel không cố đoán ý người dùng, chỉ giới hạn thời gian
// chạy và số dòng trả về.
func (s Server) Query(ctx context.Context, db *sql.DB, statement string) (QueryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	started := time.Now()
	// Trả thẳng lỗi của máy chủ cơ sở dữ liệu, không bọc thêm chữ nào: với cửa sổ
	// SQL thì thông báo lỗi chính là kết quả người dùng cần đọc, và một tiền tố
	// tiếng Việt đặt trước "syntax error at or near ..." chỉ làm khó đọc hơn.
	rows, err := db.QueryContext(ctx, statement)
	if err != nil {
		return QueryResult{}, err
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return QueryResult{}, fmt.Errorf("đọc tên cột: %w", err)
	}

	result := QueryResult{Columns: columns, Rows: [][]any{}}
	for rows.Next() {
		if len(result.Rows) >= maxRows {
			result.Truncated = true
			break
		}

		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return QueryResult{}, fmt.Errorf("đọc dòng: %w", err)
		}
		result.Rows = append(result.Rows, normalizeRow(values))
	}
	if err := rows.Err(); err != nil {
		return QueryResult{}, fmt.Errorf("đọc kết quả: %w", err)
	}

	result.DurationMS = time.Since(started).Milliseconds()
	return result, nil
}

// Exec chạy một câu lệnh không trả về dòng dữ liệu.
func (s Server) Exec(ctx context.Context, db *sql.DB, statement string) (QueryResult, error) {
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	started := time.Now()
	res, err := db.ExecContext(ctx, statement)
	if err != nil {
		return QueryResult{}, err
	}

	affected, _ := res.RowsAffected()
	return QueryResult{
		Columns: []string{}, Rows: [][]any{},
		RowsAffected: affected, DurationMS: time.Since(started).Milliseconds(),
	}, nil
}

// normalizeRow đổi giá trị driver trả về sang kiểu mà JSON biểu diễn được.
//
// Driver trả []byte cho phần lớn kiểu chuỗi; để nguyên thì JSON mã hóa chúng
// thành base64 và giao diện hiện ra một mớ ký tự vô nghĩa.
func normalizeRow(values []any) []any {
	out := make([]any, len(values))
	for i, value := range values {
		switch typed := value.(type) {
		case []byte:
			out[i] = string(typed)
		case time.Time:
			out[i] = typed.Format(time.RFC3339)
		default:
			out[i] = value
		}
	}
	return out
}
