package service

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/thanhtinz/sunpanel/internal/model"
)

// newMemoryDB tạo một cơ sở dữ liệu trong bộ nhớ đã chạy migration, dùng chung
// cho các bài test cần AuditService thật.
//
// Mỗi lần gọi tạo một cơ sở dữ liệu riêng biệt (không dùng cache=shared) để các
// bài test chạy song song không giẫm lên dữ liệu của nhau.
func newMemoryDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("mở cơ sở dữ liệu trong bộ nhớ: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	err = db.AutoMigrate(
		&model.User{}, &model.Session{}, &model.AuditLog{},
		&model.LoginLog{}, &model.MonitorSample{}, &model.Setting{},
		&model.CronJob{}, &model.CronRun{},
	)
	if err != nil {
		t.Fatalf("chạy migration: %v", err)
	}

	return db
}
