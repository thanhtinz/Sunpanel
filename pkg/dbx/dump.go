package dbx

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// dumpTimeout giới hạn thời gian một lần kết xuất.
//
// Cơ sở dữ liệu vài chục gigabyte mất khá lâu, nhưng vẫn phải có trần: một lần
// kết xuất treo sẽ giữ tiến trình và tệp tạm mãi mãi.
const dumpTimeout = 4 * time.Hour

// DumpTool là tên chương trình kết xuất của từng loại máy chủ.
func (k Kind) DumpTool() string {
	if k == PostgreSQL {
		return "pg_dump"
	}
	return "mysqldump"
}

// RestoreTool là tên chương trình khôi phục của từng loại máy chủ.
func (k Kind) RestoreTool() string {
	if k == PostgreSQL {
		return "psql"
	}
	return "mysql"
}

// DumpAvailable cho biết công cụ kết xuất có trên máy hay không.
//
// Panel dùng driver thuần Go cho mọi thao tác quản lý, nhưng việc kết xuất thì
// vẫn phải nhờ công cụ chính hãng: một bản kết xuất tự viết rất dễ thiếu ràng
// buộc, trigger hay thứ tự khóa ngoại, và chỉ lộ ra đúng lúc cần khôi phục.
func (k Kind) DumpAvailable() bool {
	_, err := exec.LookPath(k.DumpTool())
	return err == nil
}

// Dump kết xuất một cơ sở dữ liệu ra writer.
func (s Server) Dump(ctx context.Context, database string, w io.Writer) error {
	if err := ValidateIdentifier(database); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, dumpTimeout)
	defer cancel()

	cmd, err := s.dumpCommand(ctx, database)
	if err != nil {
		return err
	}
	cmd.Stdout = w

	// Đầu ra lỗi được giữ lại để báo cho người dùng; kết xuất thất bại mà chỉ nói
	// "mã thoát 1" thì không ai sửa được.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("mở luồng lỗi: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("chạy %s: %w", s.Kind.DumpTool(), err)
	}
	message, _ := io.ReadAll(io.LimitReader(stderr, 4000))

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%s thất bại: %w: %s", s.Kind.DumpTool(), err, message)
	}
	return nil
}

// dumpCommand dựng lệnh kết xuất cho từng loại máy chủ.
func (s Server) dumpCommand(ctx context.Context, database string) (*exec.Cmd, error) {
	switch s.Kind {
	case MySQL:
		cmd := exec.CommandContext(ctx, "mysqldump",
			"--host="+s.Host, "--port="+strconv.Itoa(s.Port), "--user="+s.User,
			// Khóa bảng khi kết xuất sẽ chặn ghi trên cả cơ sở dữ liệu đang chạy;
			// single-transaction cho bản kết xuất nhất quán mà không khóa gì.
			"--single-transaction", "--quick", "--routines", "--triggers", "--events",
			"--default-character-set=utf8mb4",
			database,
		)
		// Mật khẩu truyền qua biến môi trường chứ không qua tham số dòng lệnh:
		// tham số dòng lệnh hiện trong ps và mọi người dùng trên máy đều đọc được.
		cmd.Env = append(os.Environ(), "MYSQL_PWD="+s.Password)
		return cmd, nil

	case PostgreSQL:
		cmd := exec.CommandContext(ctx, "pg_dump",
			"--host="+s.Host, "--port="+strconv.Itoa(s.Port), "--username="+s.User,
			"--no-password", "--clean", "--if-exists", database,
		)
		cmd.Env = append(os.Environ(), "PGPASSWORD="+s.Password)
		return cmd, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrUnsupportedKind, s.Kind)
}

// Restore nạp một bản kết xuất vào cơ sở dữ liệu.
func (s Server) Restore(ctx context.Context, database string, r io.Reader) error {
	if err := ValidateIdentifier(database); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, dumpTimeout)
	defer cancel()

	cmd, err := s.restoreCommand(ctx, database)
	if err != nil {
		return err
	}
	cmd.Stdin = r

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("mở luồng lỗi: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("chạy %s: %w", s.Kind.RestoreTool(), err)
	}
	message, _ := io.ReadAll(io.LimitReader(stderr, 4000))

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%s thất bại: %w: %s", s.Kind.RestoreTool(), err, message)
	}
	return nil
}

func (s Server) restoreCommand(ctx context.Context, database string) (*exec.Cmd, error) {
	switch s.Kind {
	case MySQL:
		cmd := exec.CommandContext(ctx, "mysql",
			"--host="+s.Host, "--port="+strconv.Itoa(s.Port), "--user="+s.User,
			"--default-character-set=utf8mb4", database,
		)
		cmd.Env = append(os.Environ(), "MYSQL_PWD="+s.Password)
		return cmd, nil

	case PostgreSQL:
		cmd := exec.CommandContext(ctx, "psql",
			"--host="+s.Host, "--port="+strconv.Itoa(s.Port), "--username="+s.User,
			"--no-password",
			// Dừng ngay ở lỗi đầu tiên: khôi phục "một nửa" nguy hiểm hơn là không
			// khôi phục, vì người dùng tưởng dữ liệu đã về đủ.
			"--set", "ON_ERROR_STOP=on",
			"--dbname="+database,
		)
		cmd.Env = append(os.Environ(), "PGPASSWORD="+s.Password)
		return cmd, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrUnsupportedKind, s.Kind)
}
