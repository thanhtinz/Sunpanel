package dbx

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
)

// Máy chủ dùng cho test lấy từ biến môi trường: CI không phải lúc nào cũng có
// sẵn MySQL và PostgreSQL, và test phải bỏ qua chứ không được báo hỏng.
func testServer(t *testing.T, kind Kind) Server {
	t.Helper()

	password := os.Getenv("SUNPANEL_TEST_DB_PASSWORD")
	if password == "" {
		t.Skip("chưa đặt SUNPANEL_TEST_DB_PASSWORD")
	}

	switch kind {
	case MySQL:
		return Server{Kind: MySQL, Host: "127.0.0.1", Port: 3306, User: "panel", Password: password}
	default:
		return Server{Kind: PostgreSQL, Host: "127.0.0.1", Port: 5432, User: "postgres", Password: password}
	}
}

func openTest(t *testing.T, kind Kind) (Server, *sql.DB) {
	t.Helper()

	server := testServer(t, kind)
	db, err := server.Open(context.Background())
	if err != nil {
		t.Skipf("không kết nối được tới %s: %v", kind, err)
	}
	t.Cleanup(func() { db.Close() })
	return server, db
}

// Tên cơ sở dữ liệu và tài khoản không truyền được qua tham số câu lệnh nên
// phải ghép vào chuỗi SQL. Đây là bề mặt tấn công duy nhất còn lại.
func TestValidateIdentifierRejectsInjection(t *testing.T) {
	bad := []string{
		"a; DROP DATABASE khac", "a`b", `a"b`, "a'b", "a b", "", "-bat-dau-bang-gach",
		"a\nb", "../thoat", strings.Repeat("a", 64), "a\\b", "a%b",
	}
	for _, name := range bad {
		if err := ValidateIdentifier(name); !errors.Is(err, ErrInvalidIdentifier) {
			t.Errorf("tên %q phải bị từ chối", name)
		}
	}

	good := []string{"wordpress", "my_db", "Db1", "a-b", "_x", "app$data"}
	for _, name := range good {
		if err := ValidateIdentifier(name); err != nil {
			t.Errorf("tên %q phải được chấp nhận: %v", name, err)
		}
	}
}

// Xóa nhầm cơ sở dữ liệu hệ thống làm hỏng cả máy chủ chứ không chỉ mất dữ liệu.
func TestSystemDatabasesAreProtected(t *testing.T) {
	for _, name := range []string{"mysql", "MySQL", "information_schema", "postgres", "template1"} {
		if !IsSystemDatabase(name) {
			t.Errorf("%q phải được coi là cơ sở dữ liệu hệ thống", name)
		}
	}
	if IsSystemDatabase("wordpress") {
		t.Error("cơ sở dữ liệu thường bị nhận nhầm là của hệ thống")
	}
}

func TestMySQLLifecycle(t *testing.T)    { runLifecycle(t, MySQL) }
func TestPostgresLifecycle(t *testing.T) { runLifecycle(t, PostgreSQL) }

// runLifecycle chạy trọn vòng đời: tạo cơ sở dữ liệu, tạo tài khoản, ghi dữ
// liệu, đọc lại, rồi dọn sạch.
func runLifecycle(t *testing.T, kind Kind) {
	server, db := openTest(t, kind)
	ctx := context.Background()

	version, err := server.Version(ctx, db)
	if err != nil {
		t.Fatalf("đọc phiên bản: %v", err)
	}
	t.Logf("%s: %s", kind, strings.SplitN(version, " ", 3)[0])

	const dbName = "sunpanel_kiem_thu"
	const userName = "sunpanel_thu"

	// Dọn tàn dư của lần chạy trước để test lặp lại được.
	_ = server.DropDatabase(ctx, db, dbName)
	_ = server.DropUser(ctx, db, userName, "%")

	if err := server.CreateDatabase(ctx, db, dbName); err != nil {
		t.Fatalf("tạo cơ sở dữ liệu: %v", err)
	}
	t.Cleanup(func() {
		_ = server.DropUser(context.Background(), db, userName, "%")
		_ = server.DropDatabase(context.Background(), db, dbName)
	})

	databases, err := server.ListDatabases(ctx, db)
	if err != nil {
		t.Fatalf("liệt kê cơ sở dữ liệu: %v", err)
	}
	if !containsDatabase(databases, dbName) {
		t.Fatalf("cơ sở dữ liệu vừa tạo không có trong danh sách: %+v", databases)
	}

	// Mật khẩu chứa dấu nháy và dấu chéo ngược: đây đúng là chỗ mà việc tự thoát
	// ký tự hay sai và tạo ra lỗ hổng.
	const password = `mat'khau"kho\\`
	if err := server.CreateUser(ctx, db, userName, password, dbName, "%"); err != nil {
		t.Fatalf("tạo tài khoản: %v", err)
	}

	users, err := server.ListUsers(ctx, db)
	if err != nil {
		t.Fatalf("liệt kê tài khoản: %v", err)
	}
	if !containsUser(users, userName) {
		t.Errorf("tài khoản vừa tạo không có trong danh sách")
	}

	// Tài khoản mới phải đăng nhập được bằng đúng mật khẩu vừa đặt.
	userServer := server
	userServer.User = userName
	userServer.Password = password
	userServer.Database = dbName
	userDB, err := userServer.Open(ctx)
	if err != nil {
		t.Fatalf("tài khoản mới không đăng nhập được: %v", err)
	}
	defer userDB.Close()

	if _, err := userServer.Exec(ctx, userDB, "CREATE TABLE ghi_chu (id int, noi_dung varchar(100))"); err != nil {
		t.Fatalf("tạo bảng: %v", err)
	}
	if _, err := userServer.Exec(ctx, userDB, "INSERT INTO ghi_chu VALUES (1, 'xin chào tiếng Việt')"); err != nil {
		t.Fatalf("chèn dữ liệu: %v", err)
	}

	result, err := userServer.Query(ctx, userDB, "SELECT id, noi_dung FROM ghi_chu")
	if err != nil {
		t.Fatalf("truy vấn: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("kỳ vọng 1 dòng, nhận %d", len(result.Rows))
	}
	// Driver trả []byte cho kiểu chuỗi; nếu không đổi sang string thì JSON mã hóa
	// thành base64 và giao diện hiện một mớ ký tự vô nghĩa.
	if text, ok := result.Rows[0][1].(string); !ok || text != "xin chào tiếng Việt" {
		t.Errorf("giá trị chuỗi sai kiểu hoặc sai nội dung: %#v", result.Rows[0][1])
	}

	tables, err := userServer.ListTables(ctx, userDB, dbName)
	if err != nil {
		t.Fatalf("liệt kê bảng: %v", err)
	}
	if len(tables) == 0 || tables[0].Name != "ghi_chu" {
		t.Errorf("bảng vừa tạo không có trong danh sách: %+v", tables)
	}
}

// Đổi mật khẩu phải thực sự có hiệu lực, nếu không người dùng tưởng đã đổi mà
// mật khẩu cũ vẫn dùng được.
func TestChangePasswordTakesEffect(t *testing.T) {
	for _, kind := range []Kind{MySQL, PostgreSQL} {
		t.Run(string(kind), func(t *testing.T) {
			server, db := openTest(t, kind)
			ctx := context.Background()

			const userName = "sunpanel_doi_mk"
			_ = server.DropUser(ctx, db, userName, "%")

			if err := server.CreateUser(ctx, db, userName, "mat-khau-cu", "", "%"); err != nil {
				t.Fatalf("tạo tài khoản: %v", err)
			}
			t.Cleanup(func() { _ = server.DropUser(context.Background(), db, userName, "%") })

			if err := server.ChangePassword(ctx, db, userName, "mat-khau-moi", "%"); err != nil {
				t.Fatalf("đổi mật khẩu: %v", err)
			}

			withNew := server
			withNew.User, withNew.Password = userName, "mat-khau-moi"
			newDB, err := withNew.Open(ctx)
			if err != nil {
				t.Fatalf("mật khẩu mới không dùng được: %v", err)
			}
			newDB.Close()

			withOld := server
			withOld.User, withOld.Password = userName, "mat-khau-cu"
			if oldDB, err := withOld.Open(ctx); err == nil {
				oldDB.Close()
				t.Error("mật khẩu cũ vẫn đăng nhập được sau khi đổi")
			}
		})
	}
}

// Từ chối xóa cơ sở dữ liệu hệ thống phải là hàng rào thật, không chỉ là kiểm
// tra ở giao diện.
func TestDropSystemDatabaseIsRefused(t *testing.T) {
	server, db := openTest(t, MySQL)
	if err := server.DropDatabase(context.Background(), db, "mysql"); !errors.Is(err, ErrProtected) {
		t.Errorf("xóa cơ sở dữ liệu hệ thống phải bị từ chối, nhận: %v", err)
	}
}

func containsDatabase(list []DatabaseInfo, name string) bool {
	for _, item := range list {
		if item.Name == name {
			return true
		}
	}
	return false
}

func containsUser(list []UserInfo, name string) bool {
	for _, item := range list {
		if item.Name == name {
			return true
		}
	}
	return false
}

// Kết xuất rồi khôi phục là đường duy nhất để cứu dữ liệu; nó phải được thử
// trọn vòng chứ không chỉ kiểm là lệnh chạy được.
func TestDumpAndRestoreRoundTrip(t *testing.T) {
	for _, kind := range []Kind{MySQL, PostgreSQL} {
		t.Run(string(kind), func(t *testing.T) {
			server, db := openTest(t, kind)
			ctx := context.Background()

			if !kind.DumpAvailable() {
				t.Skipf("máy không có %s", kind.DumpTool())
			}

			const source = "sunpanel_ket_xuat"
			const target = "sunpanel_khoi_phuc"
			for _, name := range []string{source, target} {
				_ = server.DropDatabase(ctx, db, name)
				if err := server.CreateDatabase(ctx, db, name); err != nil {
					t.Fatalf("tạo %s: %v", name, err)
				}
			}
			t.Cleanup(func() {
				_ = server.DropDatabase(context.Background(), db, source)
				_ = server.DropDatabase(context.Background(), db, target)
			})

			// Ghi dữ liệu có dấu tiếng Việt: bộ mã sai chỉ lộ ra khi khôi phục.
			write := server
			write.Database = source
			writeDB, err := write.Open(ctx)
			if err != nil {
				t.Fatalf("kết nối vào %s: %v", source, err)
			}
			defer writeDB.Close()

			if _, err := write.Exec(ctx, writeDB, "CREATE TABLE khach (id int, ten varchar(100))"); err != nil {
				t.Fatalf("tạo bảng: %v", err)
			}
			if _, err := write.Exec(ctx, writeDB, "INSERT INTO khach VALUES (1, 'Nguyễn Văn Đức')"); err != nil {
				t.Fatalf("chèn dữ liệu: %v", err)
			}

			var dump bytes.Buffer
			if err := server.Dump(ctx, source, &dump); err != nil {
				t.Fatalf("kết xuất: %v", err)
			}
			if dump.Len() == 0 {
				t.Fatal("bản kết xuất rỗng")
			}

			if err := server.Restore(ctx, target, bytes.NewReader(dump.Bytes())); err != nil {
				t.Fatalf("khôi phục: %v", err)
			}

			read := server
			read.Database = target
			readDB, err := read.Open(ctx)
			if err != nil {
				t.Fatalf("kết nối vào %s: %v", target, err)
			}
			defer readDB.Close()

			result, err := read.Query(ctx, readDB, "SELECT ten FROM khach WHERE id = 1")
			if err != nil {
				t.Fatalf("đọc lại dữ liệu: %v", err)
			}
			if len(result.Rows) != 1 {
				t.Fatalf("dữ liệu không được khôi phục: %d dòng", len(result.Rows))
			}
			if name, _ := result.Rows[0][0].(string); name != "Nguyễn Văn Đức" {
				t.Errorf("dấu tiếng Việt hỏng sau khi khôi phục: %q", name)
			}
		})
	}
}
