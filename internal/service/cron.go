package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/host"
)

// Giới hạn của bộ lập lịch.
const (
	// defaultJobTimeout là thời gian chạy tối đa mặc định của một tác vụ.
	defaultJobTimeout = 30 * time.Minute
	// maxJobTimeout là trần thời gian chạy; tác vụ treo mãi sẽ giữ tài nguyên vô hạn.
	maxJobTimeout = 6 * time.Hour
	// maxOutputLength là độ dài đầu ra được lưu cho mỗi lần chạy.
	maxOutputLength = 64 * 1024
	// runRetention là thời gian giữ lịch sử chạy.
	runRetention = 30 * 24 * time.Hour
	// maxRunsPerJob là số bản ghi lịch sử tối đa giữ lại cho mỗi tác vụ.
	maxRunsPerJob = 200
)

// cronParser chấp nhận biểu thức cron 5 trường quen thuộc, cộng thêm các bí danh
// mô tả như "@daily" và "@every 1h" cho tiện.
var cronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// CronService quản lý và chạy các tác vụ định kỳ.
type CronService struct {
	db    *gorm.DB
	host  host.Host
	audit *AuditService
	// workDir là thư mục làm việc mặc định của tác vụ.
	workDir string

	scheduler *cron.Cron
	// entries ánh xạ ID tác vụ sang ID mục trong bộ lập lịch, để gỡ lịch khi
	// tác vụ bị sửa hoặc xóa.
	mu      sync.Mutex
	entries map[uint]cron.EntryID
}

// NewCronService tạo service quản lý tác vụ định kỳ.
//
// cronHost cố ý KHÔNG bị giới hạn bởi danh sách lệnh cho phép: tác vụ định kỳ là
// lệnh do chính quản trị viên soạn, cùng mức tin cậy với terminal. Tách thành
// host riêng để hàng rào allowlist của phần còn lại không bị nới ra.
func NewCronService(db *gorm.DB, cronHost host.Host, workDir string, audit *AuditService) *CronService {
	return &CronService{
		db:      db,
		host:    cronHost,
		audit:   audit,
		workDir: workDir,
		// Bộ lập lịch chạy theo giờ địa phương của máy chủ, giống crontab hệ thống,
		// để "0 3 * * *" nghĩa là 3 giờ sáng theo giờ quản trị viên đang nghĩ.
		scheduler: cron.New(cron.WithParser(cronParser), cron.WithLocation(time.Local)),
		entries:   make(map[uint]cron.EntryID),
	}
}

// Start nạp các tác vụ đang bật và khởi động bộ lập lịch.
func (s *CronService) Start(ctx context.Context) error {
	var jobs []model.CronJob
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Find(&jobs).Error; err != nil {
		return fmt.Errorf("nạp tác vụ định kỳ: %w", err)
	}

	for i := range jobs {
		if err := s.schedule(&jobs[i]); err != nil {
			// Một biểu thức hỏng không được ngăn các tác vụ còn lại chạy.
			slog.Error("không lên lịch được tác vụ",
				"job", jobs[i].Name, "schedule", jobs[i].Schedule, "error", err)
		}
	}

	s.scheduler.Start()
	slog.Info("bộ lập lịch đã khởi động", "jobs", len(s.entries))

	go s.pruneLoop(ctx)
	return nil
}

// Stop dừng bộ lập lịch và chờ các tác vụ đang chạy kết thúc.
func (s *CronService) Stop() {
	<-s.scheduler.Stop().Done()
}

// CronJobRequest là dữ liệu tạo hoặc sửa một tác vụ.
type CronJobRequest struct {
	Name           string `json:"name" binding:"required"`
	Schedule       string `json:"schedule" binding:"required"`
	Command        string `json:"command" binding:"required"`
	WorkDir        string `json:"workDir"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	Enabled        *bool  `json:"enabled"`
}

// List liệt kê toàn bộ tác vụ kèm thời điểm chạy kế tiếp.
func (s *CronService) List(ctx context.Context) ([]model.CronJob, error) {
	var jobs []model.CronJob
	if err := s.db.WithContext(ctx).Order("id ASC").Find(&jobs).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	for i := range jobs {
		s.fillNextRun(&jobs[i])
	}
	return jobs, nil
}

// Get lấy một tác vụ theo ID.
func (s *CronService) Get(ctx context.Context, id uint) (*model.CronJob, error) {
	var job model.CronJob
	if err := s.db.WithContext(ctx).First(&job, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.CronJobNotFound
		}
		return nil, apperr.Internal.Wrap(err)
	}
	s.fillNextRun(&job)
	return &job, nil
}

// fillNextRun điền thời điểm chạy kế tiếp của một tác vụ.
//
// Giá trị này không nằm trong cơ sở dữ liệu nên mọi đường trả tác vụ ra ngoài
// đều phải đi qua đây. Ưu tiên con số của bộ lập lịch vì nó phản ánh lịch thật
// (quan trọng với "@every", vốn tính từ lần chạy trước), nhưng phải có đường
// lùi: khi vừa đăng ký, mục mới còn nằm trong hàng đợi và vòng lặp của bộ lập
// lịch chưa kịp tính Next, nên đọc ngay sẽ ra giá trị rỗng.
func (s *CronService) fillNextRun(job *model.CronJob) {
	if !job.Enabled {
		job.NextRunAt = nil
		return
	}

	if at, ok := s.nextRuns()[job.ID]; ok && !at.IsZero() {
		job.NextRunAt = &at
		return
	}

	sched, err := cronParser.Parse(strings.TrimSpace(job.Schedule))
	if err != nil {
		return
	}
	at := sched.Next(time.Now())
	job.NextRunAt = &at
}

// Create tạo tác vụ mới.
func (s *CronService) Create(ctx context.Context, req CronJobRequest) (*model.CronJob, error) {
	job, err := buildJob(&model.CronJob{}, req)
	if err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).Create(job).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	if job.Enabled {
		if err := s.schedule(job); err != nil {
			return nil, err
		}
	}
	s.fillNextRun(job)
	return job, nil
}

// Update sửa một tác vụ và lên lịch lại.
func (s *CronService) Update(ctx context.Context, id uint, req CronJobRequest) (*model.CronJob, error) {
	job, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := buildJob(job, req); err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).Save(job).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	// Gỡ lịch cũ trước rồi mới lên lịch mới, nếu không tác vụ sẽ chạy hai lần
	// theo cả biểu thức cũ lẫn mới.
	s.unschedule(job.ID)
	if job.Enabled {
		if err := s.schedule(job); err != nil {
			return nil, err
		}
	}
	s.fillNextRun(job)
	return job, nil
}

// Delete xóa một tác vụ cùng lịch sử chạy của nó.
func (s *CronService) Delete(ctx context.Context, id uint) error {
	s.unschedule(id)

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&model.CronJob{}, id).Error; err != nil {
			return err
		}
		return tx.Where("job_id = ?", id).Delete(&model.CronRun{}).Error
	})
	if err != nil {
		return apperr.Internal.Wrap(err)
	}
	return nil
}

// SetEnabled bật hoặc tắt một tác vụ.
func (s *CronService) SetEnabled(ctx context.Context, id uint, enabled bool) (*model.CronJob, error) {
	job, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	job.Enabled = enabled
	if err := s.db.WithContext(ctx).Model(job).Update("enabled", enabled).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	s.unschedule(id)
	if enabled {
		if err := s.schedule(job); err != nil {
			return nil, err
		}
	}
	s.fillNextRun(job)
	return job, nil
}

// RunNow chạy một tác vụ ngay lập tức, không chờ tới lịch.
func (s *CronService) RunNow(ctx context.Context, id uint) (*model.CronRun, error) {
	job, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.run(ctx, job, "manual"), nil
}

// Runs trả về lịch sử chạy của một tác vụ, mới nhất trước.
func (s *CronService) Runs(ctx context.Context, jobID uint, limit int) ([]model.CronRun, error) {
	switch {
	case limit <= 0:
		limit = 50
	case limit > 200:
		limit = 200
	}

	var runs []model.CronRun
	err := s.db.WithContext(ctx).
		Where("job_id = ?", jobID).
		Order("started_at DESC").Limit(limit).
		Find(&runs).Error
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	return runs, nil
}

// ValidateSchedule kiểm tra một biểu thức cron và trả về vài lần chạy kế tiếp,
// để người dùng thấy ngay biểu thức mình gõ có đúng ý không.
func (s *CronService) ValidateSchedule(expression string, count int) ([]time.Time, error) {
	sched, err := cronParser.Parse(strings.TrimSpace(expression))
	if err != nil {
		return nil, apperr.CronInvalidSchedule.Wrap(err)
	}

	if count <= 0 || count > 10 {
		count = 5
	}

	next := make([]time.Time, 0, count)
	at := time.Now()
	for range count {
		at = sched.Next(at)
		next = append(next, at)
	}
	return next, nil
}

// schedule đăng ký một tác vụ với bộ lập lịch.
func (s *CronService) schedule(job *model.CronJob) error {
	sched, err := cronParser.Parse(strings.TrimSpace(job.Schedule))
	if err != nil {
		return apperr.CronInvalidSchedule.Wrap(err)
	}

	id := job.ID
	entryID := s.scheduler.Schedule(sched, cron.FuncJob(func() {
		// Đọc lại tác vụ từ cơ sở dữ liệu ở mỗi lần chạy: bản ghi đã đóng gói
		// trong closure sẽ cũ đi sau khi người dùng sửa lệnh.
		ctx := context.Background()
		current, err := s.Get(ctx, id)
		if err != nil {
			slog.Error("không đọc được tác vụ khi tới lịch", "job", id, "error", err)
			return
		}
		if !current.Enabled {
			return
		}
		s.run(ctx, current, "schedule")
	}))

	s.mu.Lock()
	s.entries[job.ID] = entryID
	s.mu.Unlock()

	return nil
}

// unschedule gỡ một tác vụ khỏi bộ lập lịch.
func (s *CronService) unschedule(jobID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.entries[jobID]; ok {
		s.scheduler.Remove(entryID)
		delete(s.entries, jobID)
	}
}

// nextRuns đọc thời điểm chạy kế tiếp của mọi tác vụ đang được lên lịch.
func (s *CronService) nextRuns() map[uint]time.Time {
	s.mu.Lock()
	entries := make(map[uint]cron.EntryID, len(s.entries))
	for jobID, entryID := range s.entries {
		entries[jobID] = entryID
	}
	s.mu.Unlock()

	next := make(map[uint]time.Time, len(entries))
	for jobID, entryID := range entries {
		if entry := s.scheduler.Entry(entryID); !entry.Next.IsZero() {
			next[jobID] = entry.Next
		}
	}
	return next
}

// run thực thi một tác vụ và ghi lại kết quả.
func (s *CronService) run(ctx context.Context, job *model.CronJob, triggeredBy string) *model.CronRun {
	timeout := time.Duration(job.TimeoutSeconds) * time.Second
	switch {
	case timeout <= 0:
		timeout = defaultJobTimeout
	case timeout > maxJobTimeout:
		timeout = maxJobTimeout
	}

	workDir := job.WorkDir
	if workDir == "" {
		workDir = s.workDir
	}

	started := time.Now()
	run := &model.CronRun{
		JobID:       job.ID,
		JobName:     job.Name,
		StartedAt:   started,
		TriggeredBy: triggeredBy,
	}

	// Lệnh chạy qua shell vì quản trị viên mong đợi cú pháp shell đầy đủ: ống
	// dẫn, chuyển hướng, biến môi trường. Đây là lệnh do chính họ soạn, cùng mức
	// tin cậy với terminal — không phải dữ liệu từ bên ngoài.
	shell, args := shellCommand(job.Command)
	res, err := s.host.Exec(ctx, host.Command{
		Path:    shell,
		Args:    args,
		Dir:     workDir,
		Timeout: timeout,
	})

	finished := time.Now()
	run.FinishedAt = &finished
	run.DurationMS = finished.Sub(started).Milliseconds()
	run.Output = truncate(res.Stdout+res.Stderr, maxOutputLength)

	if err != nil {
		run.Success = false
		run.ExitCode = -1
		run.Error = err.Error()
	} else {
		run.ExitCode = res.ExitCode
		run.Success = res.OK()
	}

	s.saveRun(ctx, job, run)
	return run
}

// saveRun lưu bản ghi lần chạy và cập nhật tóm tắt trên tác vụ.
func (s *CronService) saveRun(ctx context.Context, job *model.CronJob, run *model.CronRun) {
	if err := s.db.WithContext(ctx).Create(run).Error; err != nil {
		slog.Error("không lưu được lịch sử chạy", "job", job.Name, "error", err)
	}

	success := run.Success
	err := s.db.WithContext(ctx).Model(&model.CronJob{}).Where("id = ?", job.ID).Updates(map[string]any{
		"last_run_at":    run.StartedAt,
		"last_success":   &success,
		"last_exit_code": run.ExitCode,
	}).Error
	if err != nil {
		slog.Error("không cập nhật được tóm tắt tác vụ", "job", job.Name, "error", err)
	}

	if !run.Success {
		slog.Warn("tác vụ định kỳ thất bại",
			"job", job.Name, "exitCode", run.ExitCode, "error", run.Error)
	}
}

// pruneLoop dọn lịch sử chạy cũ theo chu kỳ.
func (s *CronService) pruneLoop(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.prune(ctx)
		}
	}
}

// prune xóa lịch sử quá cũ và giữ mỗi tác vụ tối đa maxRunsPerJob bản ghi.
func (s *CronService) prune(ctx context.Context) {
	cutoff := time.Now().Add(-runRetention)
	if err := s.db.WithContext(ctx).Where("started_at < ?", cutoff).Delete(&model.CronRun{}).Error; err != nil {
		slog.Error("không dọn được lịch sử chạy cũ", "error", err)
	}

	// Một tác vụ chạy mỗi phút sẽ tích hàng chục nghìn bản ghi trong thời gian
	// lưu trữ, nên còn phải giới hạn theo số lượng cho từng tác vụ.
	var jobIDs []uint
	if err := s.db.WithContext(ctx).Model(&model.CronJob{}).Pluck("id", &jobIDs).Error; err != nil {
		return
	}

	for _, jobID := range jobIDs {
		var keepFrom time.Time
		err := s.db.WithContext(ctx).Model(&model.CronRun{}).
			Where("job_id = ?", jobID).
			Order("started_at DESC").Offset(maxRunsPerJob).Limit(1).
			Pluck("started_at", &keepFrom).Error
		if err != nil || keepFrom.IsZero() {
			continue
		}
		s.db.WithContext(ctx).
			Where("job_id = ? AND started_at <= ?", jobID, keepFrom).
			Delete(&model.CronRun{})
	}
}

// buildJob áp dữ liệu yêu cầu lên một tác vụ sau khi kiểm tra hợp lệ.
func buildJob(job *model.CronJob, req CronJobRequest) (*model.CronJob, error) {
	name := strings.TrimSpace(req.Name)
	command := strings.TrimSpace(req.Command)
	schedule := strings.TrimSpace(req.Schedule)

	if name == "" || command == "" {
		return nil, apperr.BadRequest
	}
	if _, err := cronParser.Parse(schedule); err != nil {
		return nil, apperr.CronInvalidSchedule.Wrap(err)
	}

	job.Name = name
	job.Schedule = schedule
	job.Command = command
	job.WorkDir = strings.TrimSpace(req.WorkDir)
	job.TimeoutSeconds = req.TimeoutSeconds
	if req.Enabled != nil {
		job.Enabled = *req.Enabled
	} else if job.ID == 0 {
		job.Enabled = true
	}

	return job, nil
}
