package dbx

// Đăng ký driver cho database/sql.
//
// Hai driver này đều thuần Go, nên binary vẫn dựng được với CGO_ENABLED=0 và
// cross-compile sang mọi nền tảng mà không cần trình biên dịch C.
import (
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)
