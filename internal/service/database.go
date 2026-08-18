package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/crypto"
	"github.com/thanhtinz/sunpanel/pkg/dbx"
)

// ServerRequest là dữ liệu tạo hoặc sửa một máy chủ cơ sở dữ liệu.
type ServerRequest struct {
	Name string `json:"name" binding:"required"`
	Kind string `json:"kind" binding:"required"`
	Host string `json:"host" binding:"required"`
	Port int    `json:"port"`
	User string `json:"user" binding:"required"`
	// Password để trống khi sửa nghĩa là giữ nguyên mật khẩu cũ.
	Password string `json:"password"`
	Remark   string `json:"remark"`
}

// ServerInfo là máy chủ kèm trạng thái kết nối.
type ServerInfo struct {
	model.DatabaseServer
	// Online cho biết panel có kết nối được tới máy chủ hay không.
	Online bool `json:"online"`
	// Version là chuỗi phiên bản máy chủ báo về.
	Version string `json:"version,omitempty"`
	// Error là lý do không kết nối được.
	Error string `json:"error,omitempty"`
}

// DatabaseService quản lý các máy chủ cơ sở dữ liệu.
type DatabaseService struct {
	db     *gorm.DB
	sealer *crypto.Sealer
	audit  *AuditService
}

// NewDatabaseService tạo dịch vụ cơ sở dữ liệu.
func NewDatabaseService(db *gorm.DB, sealer *crypto.Sealer, audit *AuditService) *DatabaseService {
	return &DatabaseService{db: db, sealer: sealer, audit: audit}
}

// defaultPortFor trả về cổng mặc định của từng loại máy chủ.
func defaultPortFor(kind dbx.Kind) int {
	if kind == dbx.PostgreSQL {
		return 5432
	}
	return 3306
}

// ListServers liệt kê máy chủ kèm trạng thái kết nối.
func (s *DatabaseService) ListServers(ctx context.Context) ([]ServerInfo, error) {
	var records []model.DatabaseServer
	if err := s.db.WithContext(ctx).Order("name").Find(&records).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	out := make([]ServerInfo, 0, len(records))
	for _, record := range records {
		info := ServerInfo{DatabaseServer: record}

		// Máy chủ không kết nối được không phải lỗi của cả danh sách: nó là một
		// trạng thái cần hiển thị, vì đó chính là điều người dùng đang muốn biết.
		server, err := s.connection(record)
		if err == nil {
			var conn *sql.DB
			if conn, err = server.Open(ctx); err == nil {
				info.Online = true
				info.Version, _ = server.Version(ctx, conn)
				conn.Close()
			}
		}
		if err != nil {
			info.Error = err.Error()
		}
		out = append(out, info)
	}
	return out, nil
}

// connection dựng thông tin kết nối từ một bản ghi.
func (s *DatabaseService) connection(record model.DatabaseServer) (dbx.Server, error) {
	password, err := s.sealer.Open(record.Password)
	if err != nil {
		return dbx.Server{}, apperr.Internal.Wrap(err)
	}
	return dbx.Server{
		Kind: dbx.Kind(record.Kind), Host: record.Host, Port: record.Port,
		User: record.User, Password: password,
	}, nil
}

// open mở kết nối tới một máy chủ đã lưu.
//
// database để trống thì kết nối vào cơ sở dữ liệu mặc định của máy chủ.
func (s *DatabaseService) open(ctx context.Context, id uint, database string) (dbx.Server, *sql.DB, error) {
	record, err := s.findServer(ctx, id)
	if err != nil {
		return dbx.Server{}, nil, err
	}

	server, err := s.connection(record)
	if err != nil {
		return dbx.Server{}, nil, err
	}
	if database != "" {
		if err := dbx.ValidateIdentifier(database); err != nil {
			return dbx.Server{}, nil, apperr.DBInvalidName.WithParam("name", database)
		}
		server.Database = database
	}

	conn, err := server.Open(ctx)
	if err != nil {
		return dbx.Server{}, nil, apperr.DBConnectFailed.Wrap(err)
	}
	return server, conn, nil
}

func (s *DatabaseService) findServer(ctx context.Context, id uint) (model.DatabaseServer, error) {
	var record model.DatabaseServer
	err := s.db.WithContext(ctx).First(&record, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.DatabaseServer{}, apperr.DBServerNotFound
	}
	if err != nil {
		return model.DatabaseServer{}, apperr.Internal.Wrap(err)
	}
	return record, nil
}

// CreateServer thêm một máy chủ cơ sở dữ liệu.
func (s *DatabaseService) CreateServer(ctx context.Context, req ServerRequest) (model.DatabaseServer, error) {
	record, err := s.buildServer(ctx, model.DatabaseServer{}, req, 0)
	if err != nil {
		return model.DatabaseServer{}, err
	}

	// Thử kết nối trước khi lưu: lưu một thông tin sai rồi mới báo lỗi ở lần dùng
	// sau khiến người dùng không biết mình gõ nhầm ở đâu.
	if err := s.probe(ctx, record); err != nil {
		return model.DatabaseServer{}, err
	}

	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return model.DatabaseServer{}, apperr.Internal.Wrap(err)
	}
	return record, nil
}

// UpdateServer sửa thông tin một máy chủ.
func (s *DatabaseService) UpdateServer(ctx context.Context, id uint, req ServerRequest) (model.DatabaseServer, error) {
	existing, err := s.findServer(ctx, id)
	if err != nil {
		return model.DatabaseServer{}, err
	}

	record, err := s.buildServer(ctx, existing, req, id)
	if err != nil {
		return model.DatabaseServer{}, err
	}
	if err := s.probe(ctx, record); err != nil {
		return model.DatabaseServer{}, err
	}

	if err := s.db.WithContext(ctx).Save(&record).Error; err != nil {
		return model.DatabaseServer{}, apperr.Internal.Wrap(err)
	}
	return record, nil
}

// buildServer kiểm tra yêu cầu và dựng bản ghi máy chủ.
func (s *DatabaseService) buildServer(
	ctx context.Context, base model.DatabaseServer, req ServerRequest, selfID uint,
) (model.DatabaseServer, error) {
	kind := dbx.Kind(strings.TrimSpace(req.Kind))
	if !kind.Valid() {
		return model.DatabaseServer{}, apperr.DBInvalidKind.WithParam("kind", req.Kind)
	}

	name := strings.TrimSpace(req.Name)
	var count int64
	err := s.db.WithContext(ctx).Model(&model.DatabaseServer{}).
		Where("name = ? AND id <> ?", name, selfID).Count(&count).Error
	if err != nil {
		return model.DatabaseServer{}, apperr.Internal.Wrap(err)
	}
	if count > 0 {
		return model.DatabaseServer{}, apperr.DBServerNameExists.WithParam("name", name)
	}

	record := base
	record.Name = name
	record.Kind = string(kind)
	record.Host = strings.TrimSpace(req.Host)
	record.Port = req.Port
	if record.Port <= 0 || record.Port > 65535 {
		record.Port = defaultPortFor(kind)
	}
	record.User = strings.TrimSpace(req.User)
	record.Remark = strings.TrimSpace(req.Remark)

	// Mật khẩu để trống khi sửa nghĩa là giữ nguyên; đây là cách duy nhất để sửa
	// thông tin khác mà không phải gõ lại mật khẩu.
	if req.Password != "" {
		sealed, err := s.sealer.Seal(req.Password)
		if err != nil {
			return model.DatabaseServer{}, apperr.Internal.Wrap(err)
		}
		record.Password = sealed
	}
	if record.Password == "" {
		return model.DatabaseServer{}, apperr.BadRequest.WithParam("field", "password")
	}

	return record, nil
}

// probe thử kết nối tới máy chủ.
func (s *DatabaseService) probe(ctx context.Context, record model.DatabaseServer) error {
	server, err := s.connection(record)
	if err != nil {
		return err
	}
	conn, err := server.Open(ctx)
	if err != nil {
		return apperr.DBConnectFailed.Wrap(err)
	}
	conn.Close()
	return nil
}

// DeleteServer gỡ một máy chủ khỏi panel.
//
// Chỉ gỡ khỏi danh sách quản lý; dữ liệu trên máy chủ đó không bị đụng tới.
func (s *DatabaseService) DeleteServer(ctx context.Context, id uint) error {
	record, err := s.findServer(ctx, id)
	if err != nil {
		return err
	}
	if err := s.db.WithContext(ctx).Delete(&record).Error; err != nil {
		return apperr.Internal.Wrap(err)
	}
	return nil
}

// ListDatabases liệt kê cơ sở dữ liệu trên một máy chủ.
func (s *DatabaseService) ListDatabases(ctx context.Context, id uint) ([]dbx.DatabaseInfo, error) {
	server, conn, err := s.open(ctx, id, "")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	list, err := server.ListDatabases(ctx, conn)
	if err != nil {
		return nil, apperr.DBActionFailed.Wrap(err)
	}
	return list, nil
}

// CreateDatabase tạo một cơ sở dữ liệu mới.
func (s *DatabaseService) CreateDatabase(ctx context.Context, id uint, name string) error {
	server, conn, err := s.open(ctx, id, "")
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := server.CreateDatabase(ctx, conn, name); err != nil {
		return translateDBError(err)
	}
	return nil
}

// DropDatabase xóa một cơ sở dữ liệu.
func (s *DatabaseService) DropDatabase(ctx context.Context, id uint, name string) error {
	server, conn, err := s.open(ctx, id, "")
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := server.DropDatabase(ctx, conn, name); err != nil {
		return translateDBError(err)
	}
	return nil
}

// ListTables liệt kê bảng trong một cơ sở dữ liệu.
func (s *DatabaseService) ListTables(ctx context.Context, id uint, database string) ([]dbx.TableInfo, error) {
	server, conn, err := s.open(ctx, id, database)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	tables, err := server.ListTables(ctx, conn, database)
	if err != nil {
		return nil, apperr.DBActionFailed.Wrap(err)
	}
	return tables, nil
}

// ListUsers liệt kê tài khoản trên một máy chủ.
func (s *DatabaseService) ListUsers(ctx context.Context, id uint) ([]dbx.UserInfo, error) {
	server, conn, err := s.open(ctx, id, "")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	users, err := server.ListUsers(ctx, conn)
	if err != nil {
		return nil, apperr.DBActionFailed.Wrap(err)
	}
	return users, nil
}

// DBUserRequest là dữ liệu tạo hoặc sửa một tài khoản cơ sở dữ liệu.
type DBUserRequest struct {
	Name     string `json:"name" binding:"required"`
	Password string `json:"password"`
	// Database là cơ sở dữ liệu được cấp toàn quyền; để trống thì không cấp gì.
	Database string `json:"database"`
	// Host là phạm vi kết nối, chỉ dùng với MySQL.
	Host string `json:"host"`
}

// CreateUser tạo tài khoản cơ sở dữ liệu.
func (s *DatabaseService) CreateUser(ctx context.Context, id uint, req DBUserRequest) error {
	server, conn, err := s.open(ctx, id, "")
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := server.CreateUser(ctx, conn, req.Name, req.Password, req.Database, req.Host); err != nil {
		return translateDBError(err)
	}
	return nil
}

// DropUser xóa một tài khoản cơ sở dữ liệu.
func (s *DatabaseService) DropUser(ctx context.Context, id uint, name, host string) error {
	server, conn, err := s.open(ctx, id, "")
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := server.DropUser(ctx, conn, name, host); err != nil {
		return translateDBError(err)
	}
	return nil
}

// ChangePassword đổi mật khẩu một tài khoản cơ sở dữ liệu.
func (s *DatabaseService) ChangePassword(ctx context.Context, id uint, req DBUserRequest) error {
	server, conn, err := s.open(ctx, id, "")
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := server.ChangePassword(ctx, conn, req.Name, req.Password, req.Host); err != nil {
		return translateDBError(err)
	}
	return nil
}

// Query chạy một câu lệnh SQL trên một cơ sở dữ liệu.
//
// Câu lệnh chạy nguyên văn với quyền của tài khoản quản trị đã cấu hình — đây là
// công cụ ở mức quản trị viên cơ sở dữ liệu, tương đương một cửa sổ psql hay mysql.
func (s *DatabaseService) Query(ctx context.Context, id uint, database, statement string) (dbx.QueryResult, error) {
	server, conn, err := s.open(ctx, id, database)
	if err != nil {
		return dbx.QueryResult{}, err
	}
	defer conn.Close()

	// Câu lệnh trả về dòng dữ liệu và câu lệnh chỉ ghi cần hai đường xử lý khác
	// nhau: Query trên một lệnh INSERT không cho biết số dòng bị ảnh hưởng.
	if returnsRows(statement) {
		result, err := server.Query(ctx, conn, statement)
		if err != nil {
			return dbx.QueryResult{}, queryError(err)
		}
		return result, nil
	}

	result, err := server.Exec(ctx, conn, statement)
	if err != nil {
		return dbx.QueryResult{}, queryError(err)
	}
	return result, nil
}

// queryError bọc lỗi SQL kèm nguyên văn thông báo của máy chủ cơ sở dữ liệu.
//
// Với cửa sổ SQL, thông báo lỗi CHÍNH LÀ kết quả cần xem: "syntax error at or
// near ..." cho biết phải sửa ở đâu, còn một mã lỗi chung chung thì không giúp
// được gì. Người dùng đã có toàn quyền chạy SQL nên nội dung này không lộ thêm
// điều gì mà họ chưa truy cập được.
func queryError(err error) error {
	message := err.Error()
	if len(message) > maxErrorLength {
		message = message[:maxErrorLength] + "…"
	}
	return apperr.DBQueryFailed.WithParam("message", message)
}

// maxErrorLength giới hạn độ dài thông báo lỗi trả về cho giao diện.
const maxErrorLength = 2000

// rowReturningPrefixes là các câu lệnh trả về dòng dữ liệu.
var rowReturningPrefixes = []string{
	"select", "show", "describe", "desc", "explain", "with", "values", "table", "pragma",
}

// returnsRows đoán một câu lệnh có trả về dòng dữ liệu hay không.
//
// Chỉ để chọn đúng đường xử lý; đoán sai cũng chỉ dẫn tới một thông báo lỗi của
// chính máy chủ cơ sở dữ liệu, không làm hỏng dữ liệu.
func returnsRows(statement string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(statement))
	// Bỏ qua chú thích đầu câu để "-- ghi chú\nSELECT ..." vẫn nhận đúng.
	for strings.HasPrefix(trimmed, "--") || strings.HasPrefix(trimmed, "/*") {
		if strings.HasPrefix(trimmed, "--") {
			_, rest, found := strings.Cut(trimmed, "\n")
			if !found {
				return false
			}
			trimmed = strings.TrimSpace(rest)
			continue
		}
		_, rest, found := strings.Cut(trimmed, "*/")
		if !found {
			return false
		}
		trimmed = strings.TrimSpace(rest)
	}

	for _, prefix := range rowReturningPrefixes {
		if strings.HasPrefix(trimmed, prefix+" ") || trimmed == prefix {
			return true
		}
	}
	return false
}

// translateDBError đổi lỗi của gói dbx sang mã lỗi của API.
func translateDBError(err error) error {
	switch {
	case errors.Is(err, dbx.ErrInvalidIdentifier):
		return apperr.DBInvalidName.Wrap(err)
	case errors.Is(err, dbx.ErrProtected):
		return apperr.DBProtected.Wrap(err)
	case errors.Is(err, dbx.ErrUnsupportedKind):
		return apperr.DBInvalidKind.Wrap(err)
	default:
		return apperr.DBActionFailed.Wrap(err)
	}
}
