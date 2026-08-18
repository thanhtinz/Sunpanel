package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/host"
)

func newCronService(t *testing.T) *CronService {
	t.Helper()

	db := newMemoryDB(t)
	// Host không giới hạn allowlist, đúng như cách cron được nối trong ứng dụng thật.
	h := host.NewLocalHost(t.TempDir(), nil)
	return NewCronService(db, h, t.TempDir(), NewAuditService(db))
}

func TestCronCreateAndList(t *testing.T) {
	svc := newCronService(t)
	ctx := context.Background()

	job, err := svc.Create(ctx, CronJobRequest{
		Name:     "Sao lưu hằng ngày",
		Schedule: "0 3 * * *",
		Command:  "echo sao-luu",
	})
	if err != nil {
		t.Fatalf("tạo tác vụ: %v", err)
	}
	if !job.Enabled {
		t.Error("tác vụ mới phải được bật mặc định")
	}

	jobs, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("liệt kê: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Name != "Sao lưu hằng ngày" {
		t.Fatalf("danh sách = %+v", jobs)
	}
}

func TestCronRejectsInvalidSchedule(t *testing.T) {
	svc := newCronService(t)
	ctx := context.Background()

	invalid := []string{
		"không phải cron",
		"* * * *",
		"60 * * * *",
		"* * * * * * *",
		"",
	}
	for _, schedule := range invalid {
		t.Run(schedule, func(t *testing.T) {
			_, err := svc.Create(ctx, CronJobRequest{
				Name: "thử", Schedule: schedule, Command: "echo x",
			})
			if !errors.Is(err, apperr.CronInvalidSchedule) && !errors.Is(err, apperr.BadRequest) {
				t.Fatalf("lỗi = %v, mong đợi CronInvalidSchedule", err)
			}
		})
	}
}

func TestCronAcceptsDescriptors(t *testing.T) {
	svc := newCronService(t)
	ctx := context.Background()

	for _, schedule := range []string{"@daily", "@hourly", "@every 1h30m", "*/5 * * * *"} {
		t.Run(schedule, func(t *testing.T) {
			if _, err := svc.Create(ctx, CronJobRequest{
				Name: "thử " + schedule, Schedule: schedule, Command: "echo x",
			}); err != nil {
				t.Fatalf("biểu thức %q phải được chấp nhận, lỗi: %v", schedule, err)
			}
		})
	}
}

// Chạy ngay phải thực thi lệnh thật và lưu đầu ra.
func TestCronRunNowCapturesOutput(t *testing.T) {
	svc := newCronService(t)
	ctx := context.Background()

	job, err := svc.Create(ctx, CronJobRequest{
		Name: "In chữ", Schedule: "@daily", Command: "echo 'kết quả có dấu: ăâđ'",
	})
	if err != nil {
		t.Fatalf("tạo tác vụ: %v", err)
	}

	run, err := svc.RunNow(ctx, job.ID)
	if err != nil {
		t.Fatalf("chạy ngay: %v", err)
	}
	if !run.Success || run.ExitCode != 0 {
		t.Fatalf("lần chạy: success=%v exitCode=%d error=%q", run.Success, run.ExitCode, run.Error)
	}
	if !strings.Contains(run.Output, "kết quả có dấu: ăâđ") {
		t.Errorf("đầu ra = %q", run.Output)
	}
	if run.TriggeredBy != "manual" {
		t.Errorf("nguồn kích hoạt = %q, mong đợi manual", run.TriggeredBy)
	}
	if run.DurationMS < 0 {
		t.Errorf("thời gian chạy âm: %d", run.DurationMS)
	}
}

// Lệnh thoát khác 0 phải được ghi là thất bại kèm mã thoát, chứ không phải lỗi hệ thống.
func TestCronRunRecordsFailure(t *testing.T) {
	svc := newCronService(t)
	ctx := context.Background()

	job, _ := svc.Create(ctx, CronJobRequest{
		Name: "Thất bại", Schedule: "@daily", Command: "echo loi-ra-stderr >&2; exit 3",
	})

	run, err := svc.RunNow(ctx, job.ID)
	if err != nil {
		t.Fatalf("chạy ngay: %v", err)
	}
	if run.Success {
		t.Error("lệnh thoát mã 3 phải được ghi là thất bại")
	}
	if run.ExitCode != 3 {
		t.Errorf("mã thoát = %d, mong đợi 3", run.ExitCode)
	}
	if !strings.Contains(run.Output, "loi-ra-stderr") {
		t.Errorf("đầu ra phải gồm cả stderr, nhận: %q", run.Output)
	}

	// Tóm tắt trên tác vụ phải được cập nhật để danh sách hiện đúng ngay.
	updated, err := svc.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("đọc lại tác vụ: %v", err)
	}
	if updated.LastSuccess == nil || *updated.LastSuccess {
		t.Error("tóm tắt phải ghi lần chạy gần nhất là thất bại")
	}
	if updated.LastExitCode != 3 {
		t.Errorf("mã thoát trong tóm tắt = %d, mong đợi 3", updated.LastExitCode)
	}
	if updated.LastRunAt == nil {
		t.Error("thiếu thời điểm chạy gần nhất")
	}
}

// Tác vụ chạy quá lâu phải bị cắt chứ không giữ tài nguyên vô hạn.
func TestCronRunHonoursTimeout(t *testing.T) {
	svc := newCronService(t)
	ctx := context.Background()

	job, _ := svc.Create(ctx, CronJobRequest{
		Name: "Treo", Schedule: "@daily", Command: "sleep 30", TimeoutSeconds: 1,
	})

	started := time.Now()
	run, err := svc.RunNow(ctx, job.ID)
	if err != nil {
		t.Fatalf("chạy ngay: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 10*time.Second {
		t.Errorf("tác vụ chạy %v, phải bị cắt sau khoảng 1 giây", elapsed)
	}
	if run.Success {
		t.Error("tác vụ bị cắt vì quá giờ phải được ghi là thất bại")
	}
}

func TestCronRunsHistory(t *testing.T) {
	svc := newCronService(t)
	ctx := context.Background()

	job, _ := svc.Create(ctx, CronJobRequest{
		Name: "Lặp", Schedule: "@daily", Command: "echo ok",
	})
	for range 3 {
		if _, err := svc.RunNow(ctx, job.ID); err != nil {
			t.Fatalf("chạy: %v", err)
		}
	}

	runs, err := svc.Runs(ctx, job.ID, 10)
	if err != nil {
		t.Fatalf("đọc lịch sử: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("số lần chạy = %d, mong đợi 3", len(runs))
	}
	// Mới nhất phải đứng trước.
	if runs[0].StartedAt.Before(runs[len(runs)-1].StartedAt) {
		t.Error("lịch sử phải sắp xếp mới nhất trước")
	}
}

// Xóa tác vụ phải dọn cả lịch sử, không để lại bản ghi mồ côi.
func TestCronDeleteRemovesHistory(t *testing.T) {
	svc := newCronService(t)
	ctx := context.Background()

	job, _ := svc.Create(ctx, CronJobRequest{
		Name: "Xóa", Schedule: "@daily", Command: "echo ok",
	})
	if _, err := svc.RunNow(ctx, job.ID); err != nil {
		t.Fatalf("chạy: %v", err)
	}

	if err := svc.Delete(ctx, job.ID); err != nil {
		t.Fatalf("xóa: %v", err)
	}
	if _, err := svc.Get(ctx, job.ID); !errors.Is(err, apperr.CronJobNotFound) {
		t.Errorf("lỗi = %v, mong đợi CronJobNotFound", err)
	}

	runs, err := svc.Runs(ctx, job.ID, 10)
	if err != nil {
		t.Fatalf("đọc lịch sử: %v", err)
	}
	if len(runs) != 0 {
		t.Errorf("còn %d bản ghi lịch sử sau khi xóa tác vụ", len(runs))
	}
}

func TestValidateSchedule(t *testing.T) {
	svc := newCronService(t)

	next, err := svc.ValidateSchedule("0 3 * * *", 3)
	if err != nil {
		t.Fatalf("kiểm tra biểu thức: %v", err)
	}
	if len(next) != 3 {
		t.Fatalf("số lần chạy kế tiếp = %d, mong đợi 3", len(next))
	}
	for i, at := range next {
		if at.Hour() != 3 || at.Minute() != 0 {
			t.Errorf("lần %d = %v, mong đợi 03:00", i, at)
		}
		if i > 0 && !at.After(next[i-1]) {
			t.Error("các lần chạy kế tiếp phải tăng dần")
		}
	}

	if _, err := svc.ValidateSchedule("sai bét", 3); !errors.Is(err, apperr.CronInvalidSchedule) {
		t.Errorf("lỗi = %v, mong đợi CronInvalidSchedule", err)
	}
}

// Sửa tác vụ phải gỡ lịch cũ, nếu không nó sẽ chạy theo cả hai biểu thức.
func TestCronUpdateReschedules(t *testing.T) {
	svc := newCronService(t)
	ctx := context.Background()

	job, _ := svc.Create(ctx, CronJobRequest{
		Name: "Ban đầu", Schedule: "0 3 * * *", Command: "echo a",
	})

	svc.mu.Lock()
	firstEntry := svc.entries[job.ID]
	svc.mu.Unlock()

	updated, err := svc.Update(ctx, job.ID, CronJobRequest{
		Name: "Đã sửa", Schedule: "0 4 * * *", Command: "echo b",
	})
	if err != nil {
		t.Fatalf("sửa tác vụ: %v", err)
	}
	if updated.Schedule != "0 4 * * *" || updated.Command != "echo b" {
		t.Errorf("tác vụ sau khi sửa = %+v", updated)
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()
	if len(svc.entries) != 1 {
		t.Fatalf("số mục trong bộ lập lịch = %d, mong đợi 1", len(svc.entries))
	}
	if svc.entries[job.ID] == firstEntry {
		t.Error("phải đăng ký mục mới sau khi đổi biểu thức")
	}
}

func TestCronSetEnabledUnschedules(t *testing.T) {
	svc := newCronService(t)
	ctx := context.Background()

	job, _ := svc.Create(ctx, CronJobRequest{
		Name: "Bật tắt", Schedule: "@daily", Command: "echo ok",
	})

	if _, err := svc.SetEnabled(ctx, job.ID, false); err != nil {
		t.Fatalf("tắt tác vụ: %v", err)
	}
	svc.mu.Lock()
	count := len(svc.entries)
	svc.mu.Unlock()
	if count != 0 {
		t.Errorf("tác vụ đã tắt vẫn còn trong bộ lập lịch (%d mục)", count)
	}

	if _, err := svc.SetEnabled(ctx, job.ID, true); err != nil {
		t.Fatalf("bật lại: %v", err)
	}
	svc.mu.Lock()
	count = len(svc.entries)
	svc.mu.Unlock()
	if count != 1 {
		t.Errorf("số mục = %d, mong đợi 1 sau khi bật lại", count)
	}
}

// Thời điểm chạy kế tiếp phải có ngay khi tạo tác vụ; nếu chỉ điền lúc liệt kê
// thì giao diện sẽ hiện ô trống ngay sau khi người dùng vừa bấm tạo.
func TestCronCreateReturnsNextRun(t *testing.T) {
	svc := newCronService(t)
	ctx := context.Background()

	job, err := svc.Create(ctx, CronJobRequest{
		Name: "Hằng ngày", Schedule: "0 3 * * *", Command: "echo ok",
	})
	if err != nil {
		t.Fatalf("tạo tác vụ: %v", err)
	}
	if job.NextRunAt == nil {
		t.Fatal("tác vụ vừa tạo phải có thời điểm chạy kế tiếp")
	}
	if !job.NextRunAt.After(time.Now()) {
		t.Errorf("thời điểm chạy kế tiếp = %v, phải nằm ở tương lai", job.NextRunAt)
	}

	// Tắt tác vụ thì không còn lần chạy kế tiếp nào.
	disabled, err := svc.SetEnabled(ctx, job.ID, false)
	if err != nil {
		t.Fatalf("tắt tác vụ: %v", err)
	}
	if disabled.NextRunAt != nil {
		t.Errorf("tác vụ đã tắt vẫn có thời điểm chạy kế tiếp: %v", disabled.NextRunAt)
	}

	enabled, err := svc.SetEnabled(ctx, job.ID, true)
	if err != nil {
		t.Fatalf("bật lại: %v", err)
	}
	if enabled.NextRunAt == nil {
		t.Error("bật lại phải có thời điểm chạy kế tiếp")
	}
}
