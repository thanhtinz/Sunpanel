package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/backup"
	"github.com/thanhtinz/sunpanel/pkg/crypto"
	"github.com/thanhtinz/sunpanel/pkg/dbx"
	"github.com/thanhtinz/sunpanel/pkg/notify"
)

// Các loại nguồn sao lưu.
const (
	// BackupSourceDatabase sao lưu một cơ sở dữ liệu.
	BackupSourceDatabase = "database"
	// BackupSourceDirectory sao lưu một thư mục trên đĩa.
	BackupSourceDirectory = "directory"
)

// backupRunRetention là thời gian giữ lịch sử chạy.
const backupRunRetention = 90 * 24 * time.Hour

// BackupPlanRequest là dữ liệu tạo hoặc sửa một kế hoạch sao lưu.
type BackupPlanRequest struct {
	Name     string `json:"name" binding:"required"`
	Source   string `json:"source" binding:"required"`
	ServerID uint   `json:"serverId"`
	Database string `json:"database"`
	Path     string `json:"path"`
	Exclude  string `json:"exclude"`

	Destination string `json:"destination" binding:"required"`
	// DestConfig là cấu hình nơi lưu trữ; để trống khi sửa nghĩa là giữ nguyên.
	DestConfig map[string]string `json:"destConfig"`

	Schedule  string `json:"schedule"`
	KeepCount int    `json:"keepCount"`
	KeepDays  int    `json:"keepDays"`
	Enabled   bool   `json:"enabled"`
}

// BackupService quản lý và chạy các kế hoạch sao lưu.
type BackupService struct {
	db        *gorm.DB
	databases *DatabaseService
	sealer    *crypto.Sealer
	audit     *AuditService
	// workDir là nơi ghi tệp tạm trong lúc dựng bản sao lưu.
	workDir string
	// localRoot là thư mục mặc định cho nơi lưu trữ dạng local.
	localRoot string

	// alerts nhận thông báo khi sao lưu thất bại; có thể nil khi chưa cấu hình.
	//
	// Một kế hoạch sao lưu hỏng âm thầm là trường hợp tệ nhất: người dùng tin
	// rằng dữ liệu đang được bảo vệ cho tới ngày cần khôi phục.
	alerts *AlertService

	scheduler *cron.Cron
	entries   map[uint]cron.EntryID
	mu        sync.Mutex
}

// SetAlerts gắn dịch vụ cảnh báo.
//
// Gắn sau khi khởi tạo thay vì truyền vào hàm dựng: dịch vụ cảnh báo cần bộ giám
// sát, còn dịch vụ sao lưu cần cảnh báo, nên truyền thẳng sẽ tạo phụ thuộc vòng.
func (s *BackupService) SetAlerts(alerts *AlertService) { s.alerts = alerts }

// NewBackupService tạo dịch vụ sao lưu.
func NewBackupService(
	db *gorm.DB, databases *DatabaseService, sealer *crypto.Sealer,
	workDir, localRoot string, audit *AuditService,
) *BackupService {
	return &BackupService{
		db: db, databases: databases, sealer: sealer, audit: audit,
		workDir: workDir, localRoot: localRoot,
		scheduler: cron.New(), entries: make(map[uint]cron.EntryID),
	}
}

// Start khởi động bộ lập lịch và nạp các kế hoạch đã bật.
func (s *BackupService) Start(ctx context.Context) error {
	var plans []model.BackupPlan
	if err := s.db.WithContext(ctx).Where("enabled = ?", true).Find(&plans).Error; err != nil {
		return fmt.Errorf("đọc danh sách kế hoạch sao lưu: %w", err)
	}

	for i := range plans {
		if plans[i].Schedule == "" {
			continue
		}
		if err := s.schedule(&plans[i]); err != nil {
			// Một kế hoạch có lịch sai không được ngăn các kế hoạch khác chạy.
			slog.Error("không lên lịch được kế hoạch sao lưu",
				"plan", plans[i].Name, "error", err)
		}
	}

	s.scheduler.Start()
	slog.Info("bộ lập lịch sao lưu đã khởi động", "plans", len(s.entries))

	go s.cleanupLoop(ctx)
	return nil
}

// Stop dừng bộ lập lịch.
func (s *BackupService) Stop() {
	stopCtx := s.scheduler.Stop()
	<-stopCtx.Done()
}

// schedule đăng ký một kế hoạch vào bộ lập lịch.
func (s *BackupService) schedule(plan *model.BackupPlan) error {
	schedule, err := cronParser.Parse(plan.Schedule)
	if err != nil {
		return apperr.BackupInvalidSchedule.WithParam("schedule", plan.Schedule)
	}

	id := plan.ID
	entryID := s.scheduler.Schedule(schedule, cron.FuncJob(func() {
		// Mỗi lần chạy có ngữ cảnh riêng: một lần sao lưu dài không được kéo theo
		// hạn thời gian của lần trước.
		if _, err := s.Run(context.Background(), id, "schedule"); err != nil {
			slog.Error("sao lưu theo lịch thất bại", "plan", id, "error", err)
		}
	}))

	s.mu.Lock()
	s.entries[plan.ID] = entryID
	s.mu.Unlock()
	return nil
}

// unschedule gỡ một kế hoạch khỏi bộ lập lịch.
func (s *BackupService) unschedule(id uint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.entries[id]; ok {
		s.scheduler.Remove(entryID)
		delete(s.entries, id)
	}
}

// nextRun tra thời điểm chạy kế tiếp của một kế hoạch.
func (s *BackupService) nextRun(id uint) *time.Time {
	s.mu.Lock()
	entryID, ok := s.entries[id]
	s.mu.Unlock()
	if !ok {
		return nil
	}

	entry := s.scheduler.Entry(entryID)
	if entry.Next.IsZero() {
		return nil
	}
	next := entry.Next
	return &next
}

// List liệt kê các kế hoạch sao lưu.
func (s *BackupService) List(ctx context.Context) ([]model.BackupPlan, error) {
	var plans []model.BackupPlan
	if err := s.db.WithContext(ctx).Order("name").Find(&plans).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	for i := range plans {
		plans[i].NextRunAt = s.nextRun(plans[i].ID)
	}
	return plans, nil
}

// Get đọc một kế hoạch.
func (s *BackupService) Get(ctx context.Context, id uint) (model.BackupPlan, error) {
	var plan model.BackupPlan
	err := s.db.WithContext(ctx).First(&plan, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.BackupPlan{}, apperr.BackupPlanNotFound
	}
	if err != nil {
		return model.BackupPlan{}, apperr.Internal.Wrap(err)
	}
	plan.NextRunAt = s.nextRun(plan.ID)
	return plan, nil
}

// Create tạo một kế hoạch sao lưu.
func (s *BackupService) Create(ctx context.Context, req BackupPlanRequest) (model.BackupPlan, error) {
	plan, err := s.build(ctx, model.BackupPlan{}, req, 0)
	if err != nil {
		return model.BackupPlan{}, err
	}

	// Kiểm nơi lưu trữ trước khi lưu kế hoạch: sai khóa S3 mà chỉ lộ ra ở lần
	// chạy theo lịch lúc 3 giờ sáng thì không ai biết cho tới khi cần khôi phục.
	if err := s.checkDestination(ctx, plan); err != nil {
		return model.BackupPlan{}, err
	}

	if err := s.db.WithContext(ctx).Create(&plan).Error; err != nil {
		return model.BackupPlan{}, apperr.Internal.Wrap(err)
	}
	if plan.Enabled && plan.Schedule != "" {
		if err := s.schedule(&plan); err != nil {
			return model.BackupPlan{}, err
		}
	}
	plan.NextRunAt = s.nextRun(plan.ID)
	return plan, nil
}

// Update sửa một kế hoạch.
func (s *BackupService) Update(ctx context.Context, id uint, req BackupPlanRequest) (model.BackupPlan, error) {
	existing, err := s.Get(ctx, id)
	if err != nil {
		return model.BackupPlan{}, err
	}

	plan, err := s.build(ctx, existing, req, id)
	if err != nil {
		return model.BackupPlan{}, err
	}
	if err := s.checkDestination(ctx, plan); err != nil {
		return model.BackupPlan{}, err
	}

	if err := s.db.WithContext(ctx).Save(&plan).Error; err != nil {
		return model.BackupPlan{}, apperr.Internal.Wrap(err)
	}

	s.unschedule(id)
	if plan.Enabled && plan.Schedule != "" {
		if err := s.schedule(&plan); err != nil {
			return model.BackupPlan{}, err
		}
	}
	plan.NextRunAt = s.nextRun(plan.ID)
	return plan, nil
}

// Delete xóa một kế hoạch.
//
// Chỉ xóa kế hoạch; các bản sao lưu đã tạo vẫn nằm nguyên ở nơi lưu trữ.
func (s *BackupService) Delete(ctx context.Context, id uint) error {
	plan, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	s.unschedule(id)
	if err := s.db.WithContext(ctx).Delete(&plan).Error; err != nil {
		return apperr.Internal.Wrap(err)
	}
	return nil
}

// SetEnabled bật hoặc tắt một kế hoạch.
func (s *BackupService) SetEnabled(ctx context.Context, id uint, enabled bool) (model.BackupPlan, error) {
	plan, err := s.Get(ctx, id)
	if err != nil {
		return model.BackupPlan{}, err
	}

	plan.Enabled = enabled
	if err := s.db.WithContext(ctx).Model(&plan).Update("enabled", enabled).Error; err != nil {
		return model.BackupPlan{}, apperr.Internal.Wrap(err)
	}

	s.unschedule(id)
	if enabled && plan.Schedule != "" {
		if err := s.schedule(&plan); err != nil {
			return model.BackupPlan{}, err
		}
	}
	plan.NextRunAt = s.nextRun(plan.ID)
	return plan, nil
}

// build kiểm tra yêu cầu và dựng bản ghi kế hoạch.
func (s *BackupService) build(
	ctx context.Context, base model.BackupPlan, req BackupPlanRequest, selfID uint,
) (model.BackupPlan, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return model.BackupPlan{}, apperr.BackupInvalidConfig.WithParam("field", "name")
	}

	var count int64
	err := s.db.WithContext(ctx).Model(&model.BackupPlan{}).
		Where("name = ? AND id <> ?", name, selfID).Count(&count).Error
	if err != nil {
		return model.BackupPlan{}, apperr.Internal.Wrap(err)
	}
	if count > 0 {
		return model.BackupPlan{}, apperr.BackupNameExists.WithParam("name", name)
	}

	kind := backup.Kind(req.Destination)
	if !kind.Valid() {
		return model.BackupPlan{}, apperr.BackupInvalidConfig.WithParam("field", "destination")
	}

	plan := base
	plan.Name = name
	plan.Source = req.Source
	plan.Destination = req.Destination
	plan.Schedule = strings.TrimSpace(req.Schedule)
	plan.KeepCount = req.KeepCount
	plan.KeepDays = req.KeepDays
	plan.Enabled = req.Enabled

	switch req.Source {
	case BackupSourceDatabase:
		if req.ServerID == 0 || strings.TrimSpace(req.Database) == "" {
			return model.BackupPlan{}, apperr.BackupInvalidConfig.WithParam("field", "database")
		}
		if err := dbx.ValidateIdentifier(req.Database); err != nil {
			return model.BackupPlan{}, apperr.DBInvalidName.WithParam("name", req.Database)
		}
		plan.ServerID = req.ServerID
		plan.Database = strings.TrimSpace(req.Database)
		plan.Path, plan.Exclude = "", ""

	case BackupSourceDirectory:
		path := strings.TrimSpace(req.Path)
		if path == "" || !filepath.IsAbs(path) {
			return model.BackupPlan{}, apperr.BackupInvalidConfig.WithParam("field", "path")
		}
		plan.Path = filepath.Clean(path)
		plan.Exclude = strings.TrimSpace(req.Exclude)
		plan.ServerID, plan.Database = 0, ""

	default:
		return model.BackupPlan{}, apperr.BackupInvalidConfig.WithParam("field", "source")
	}

	if plan.Schedule != "" {
		if _, err := cronParser.Parse(plan.Schedule); err != nil {
			return model.BackupPlan{}, apperr.BackupInvalidSchedule.WithParam("schedule", plan.Schedule)
		}
	}

	// Cấu hình nơi lưu để trống khi sửa nghĩa là giữ nguyên; đây là cách duy nhất
	// để sửa lịch mà không phải gõ lại khóa bí mật S3.
	if len(req.DestConfig) > 0 {
		encoded, err := json.Marshal(req.DestConfig)
		if err != nil {
			return model.BackupPlan{}, apperr.Internal.Wrap(err)
		}
		sealed, err := s.sealer.Seal(string(encoded))
		if err != nil {
			return model.BackupPlan{}, apperr.Internal.Wrap(err)
		}
		plan.DestConfig = sealed
	}
	if plan.DestConfig == "" && kind != backup.Local {
		return model.BackupPlan{}, apperr.BackupInvalidConfig.WithParam("field", "destConfig")
	}

	return plan, nil
}

// destinationFor dựng nơi lưu trữ của một kế hoạch.
func (s *BackupService) destinationFor(plan model.BackupPlan) (backup.Destination, error) {
	config := map[string]string{}
	if plan.DestConfig != "" {
		opened, err := s.sealer.Open(plan.DestConfig)
		if err != nil {
			return nil, apperr.Internal.Wrap(err)
		}
		if err := json.Unmarshal([]byte(opened), &config); err != nil {
			return nil, apperr.Internal.Wrap(err)
		}
	}

	switch backup.Kind(plan.Destination) {
	case backup.Local:
		dir := config["dir"]
		if dir == "" {
			dir = filepath.Join(s.localRoot, plan.Name)
		}
		return backup.NewLocal(dir), nil

	case backup.S3:
		return backup.NewS3(backup.S3Config{
			Endpoint: config["endpoint"], Region: config["region"], Bucket: config["bucket"],
			Prefix: config["prefix"], AccessKeyID: config["accessKeyId"],
			SecretAccessKey: config["secretAccessKey"], PathStyle: config["pathStyle"] == "true",
		}), nil

	case backup.WebDAV:
		return backup.NewWebDAV(backup.WebDAVConfig{
			URL: config["url"], Username: config["username"], Password: config["password"],
		}), nil
	}
	return nil, apperr.BackupInvalidConfig.WithParam("field", "destination")
}

// checkDestination xác nhận nơi lưu trữ dùng được.
func (s *BackupService) checkDestination(ctx context.Context, plan model.BackupPlan) error {
	destination, err := s.destinationFor(plan)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := destination.Check(ctx); err != nil {
		return apperr.BackupDestinationFailed.WithParam("message", trimMessage(err.Error()))
	}
	return nil
}

// CheckDestination kiểm tra nơi lưu trữ của một cấu hình chưa lưu.
func (s *BackupService) CheckDestination(ctx context.Context, req BackupPlanRequest) error {
	plan, err := s.build(ctx, model.BackupPlan{}, req, 0)
	if err != nil {
		return err
	}
	return s.checkDestination(ctx, plan)
}

// Run chạy một lần sao lưu.
func (s *BackupService) Run(ctx context.Context, id uint, triggeredBy string) (model.BackupRun, error) {
	plan, err := s.Get(ctx, id)
	if err != nil {
		return model.BackupRun{}, err
	}

	started := time.Now()
	run := model.BackupRun{
		PlanID: plan.ID, PlanName: plan.Name,
		StartedAt: started, TriggeredBy: triggeredBy,
	}

	object, size, removed, err := s.perform(ctx, plan)
	finished := time.Now()
	run.FinishedAt = &finished
	run.DurationMS = finished.Sub(started).Milliseconds()
	run.Object = object
	run.SizeBytes = size
	run.Removed = strings.Join(removed, ", ")
	run.Success = err == nil
	if err != nil {
		run.Error = trimMessage(err.Error())
	}

	if createErr := s.db.WithContext(ctx).Create(&run).Error; createErr != nil {
		slog.Error("không ghi được lịch sử sao lưu", "plan", plan.Name, "error", createErr)
	}

	// Tóm tắt lần chạy gần nhất nằm ngay trong kế hoạch để danh sách hiển thị
	// được mà không phải truy vấn bảng lịch sử.
	success := run.Success
	plan.LastRunAt = &started
	plan.LastSuccess = &success
	plan.LastError = run.Error
	if saveErr := s.db.WithContext(ctx).Save(&plan).Error; saveErr != nil {
		slog.Error("không cập nhật được kế hoạch sao lưu", "plan", plan.Name, "error", saveErr)
	}

	if err != nil {
		s.alertFailure(ctx, plan, run)
		return run, apperr.BackupFailed.WithParam("message", run.Error)
	}
	return run, nil
}

// alertFailure phát cảnh báo khi một lần sao lưu thất bại.
func (s *BackupService) alertFailure(ctx context.Context, plan model.BackupPlan, run model.BackupRun) {
	if s.alerts == nil {
		return
	}

	source := plan.Database
	if plan.Source == BackupSourceDirectory {
		source = plan.Path
	}

	s.alerts.Notify(ctx, "backup", notify.Message{
		Title: "Sao lưu thất bại: " + plan.Name,
		Body:  run.Error,
		Level: notify.LevelCritical,
		At:    run.StartedAt,
		Fields: map[string]string{
			"Kế hoạch": plan.Name,
			"Nguồn":    source,
			"Nơi lưu":  plan.Destination,
		},
	})
}

// perform thực hiện một lần sao lưu và trả về tên tệp cùng dung lượng.
func (s *BackupService) perform(
	ctx context.Context, plan model.BackupPlan,
) (object string, size int64, removed []string, err error) {
	destination, err := s.destinationFor(plan)
	if err != nil {
		return "", 0, nil, err
	}

	if err := os.MkdirAll(s.workDir, 0o700); err != nil {
		return "", 0, nil, fmt.Errorf("tạo thư mục tạm: %w", err)
	}

	// Dựng bản sao lưu ra tệp tạm trước rồi mới tải lên: một cơ sở dữ liệu lớn
	// không vừa bộ nhớ, và tải lên theo luồng thì không biết trước dung lượng.
	temp, err := os.CreateTemp(s.workDir, "sunpanel-sao-luu-*.tmp")
	if err != nil {
		return "", 0, nil, fmt.Errorf("tạo tệp tạm: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		// Tệp có thể đã được đóng ở nhánh thư mục; đóng lần nữa chỉ trả lỗi vô
		// hại, còn xóa thì luôn cần vì bản sao lưu đã tải lên xong.
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()

	stamp := time.Now().Format("20060102-150405")
	switch plan.Source {
	case BackupSourceDatabase:
		object = fmt.Sprintf("%s-%s-%s.sql.gz", plan.Name, plan.Database, stamp)
		size, err = s.dumpDatabase(ctx, plan, temp)
		if err == nil {
			// Đóng trước khi mở lại để đọc: lỗi lúc đóng nghĩa là bản kết xuất
			// cụt, mà tải một bản cụt lên còn tệ hơn báo hỏng ngay.
			if closeErr := temp.Close(); closeErr != nil {
				err = fmt.Errorf("đóng tệp sao lưu tạm: %w", closeErr)
			}
		}
	case BackupSourceDirectory:
		object = fmt.Sprintf("%s-%s.tar.gz", plan.Name, stamp)
		_ = temp.Close()
		size, err = backup.ArchiveDirectory(ctx, plan.Path, tempPath, splitList(plan.Exclude))
	default:
		err = apperr.BackupInvalidConfig.WithParam("field", "source")
	}
	if err != nil {
		return "", 0, nil, err
	}
	if err := backup.ValidateName(object); err != nil {
		return "", 0, nil, err
	}

	file, err := os.Open(tempPath)
	if err != nil {
		return "", 0, nil, fmt.Errorf("mở tệp sao lưu tạm: %w", err)
	}
	defer func() { _ = file.Close() }()

	if err := destination.Put(ctx, object, file, size); err != nil {
		return "", 0, nil, err
	}

	// Dọn bản cũ SAU khi bản mới đã lên đến nơi: dọn trước rồi tải hỏng nghĩa là
	// mất cả bản cũ lẫn bản mới.
	removed, err = backup.ApplyRetention(ctx, destination, plan.KeepCount, plan.KeepDays)
	if err != nil {
		// Bản sao lưu mới đã an toàn; việc dọn hỏng chỉ đáng ghi log.
		slog.Warn("không dọn được bản sao lưu cũ", "plan", plan.Name, "error", err)
	}

	return object, size, removed, nil
}

// dumpDatabase kết xuất cơ sở dữ liệu ra tệp, có nén.
func (s *BackupService) dumpDatabase(ctx context.Context, plan model.BackupPlan, w *os.File) (int64, error) {
	record, err := s.databases.findServer(ctx, plan.ServerID)
	if err != nil {
		return 0, err
	}
	server, err := s.databases.connection(record)
	if err != nil {
		return 0, err
	}

	if !server.Kind.DumpAvailable() {
		return 0, apperr.BackupDumpUnavailable.WithParam("tool", server.Kind.DumpTool())
	}

	gzipWriter := newGzipWriter(w)
	if err := server.Dump(ctx, plan.Database, gzipWriter); err != nil {
		_ = gzipWriter.Close()
		return 0, err
	}
	if err := gzipWriter.Close(); err != nil {
		return 0, fmt.Errorf("đóng luồng nén: %w", err)
	}

	info, err := w.Stat()
	if err != nil {
		return 0, fmt.Errorf("đọc kích thước bản kết xuất: %w", err)
	}
	return info.Size(), nil
}

// Objects liệt kê các bản sao lưu hiện có ở nơi lưu trữ của một kế hoạch.
func (s *BackupService) Objects(ctx context.Context, id uint) ([]backup.Object, error) {
	plan, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	destination, err := s.destinationFor(plan)
	if err != nil {
		return nil, err
	}

	objects, err := destination.List(ctx)
	if err != nil {
		return nil, apperr.BackupDestinationFailed.WithParam("message", trimMessage(err.Error()))
	}
	return objects, nil
}

// DeleteObject xóa một bản sao lưu ở nơi lưu trữ.
func (s *BackupService) DeleteObject(ctx context.Context, id uint, object string) error {
	plan, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	destination, err := s.destinationFor(plan)
	if err != nil {
		return err
	}

	if err := destination.Delete(ctx, object); err != nil {
		return apperr.BackupDestinationFailed.WithParam("message", trimMessage(err.Error()))
	}
	return nil
}

// Restore khôi phục một bản sao lưu.
//
// target là cơ sở dữ liệu hoặc thư mục đích; để trống thì khôi phục về đúng chỗ
// đã sao lưu. Khôi phục là thao tác GHI ĐÈ và không thể hoàn tác.
func (s *BackupService) Restore(ctx context.Context, id uint, object, target string) error {
	plan, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := backup.ValidateName(object); err != nil {
		return apperr.BackupInvalidConfig.WithParam("field", "object")
	}

	destination, err := s.destinationFor(plan)
	if err != nil {
		return err
	}

	reader, err := destination.Get(ctx, object)
	if err != nil {
		if errors.Is(err, backup.ErrNotFound) {
			return apperr.BackupObjectNotFound.WithParam("object", object)
		}
		return apperr.BackupDestinationFailed.WithParam("message", trimMessage(err.Error()))
	}
	defer func() { _ = reader.Close() }()

	switch plan.Source {
	case BackupSourceDatabase:
		return s.restoreDatabase(ctx, plan, reader, target)
	case BackupSourceDirectory:
		return s.restoreDirectory(ctx, plan, reader, target)
	}
	return apperr.BackupInvalidConfig.WithParam("field", "source")
}

func (s *BackupService) restoreDatabase(
	ctx context.Context, plan model.BackupPlan, reader io.Reader, target string,
) error {
	database := plan.Database
	if target != "" {
		if err := dbx.ValidateIdentifier(target); err != nil {
			return apperr.DBInvalidName.WithParam("name", target)
		}
		database = target
	}

	record, err := s.databases.findServer(ctx, plan.ServerID)
	if err != nil {
		return err
	}
	server, err := s.databases.connection(record)
	if err != nil {
		return err
	}

	gzipReader, err := newGzipReader(reader)
	if err != nil {
		return apperr.BackupRestoreFailed.WithParam("message", trimMessage(err.Error()))
	}
	defer func() { _ = gzipReader.Close() }()

	if err := server.Restore(ctx, database, gzipReader); err != nil {
		return apperr.BackupRestoreFailed.WithParam("message", trimMessage(err.Error()))
	}
	return nil
}

func (s *BackupService) restoreDirectory(
	ctx context.Context, plan model.BackupPlan, reader io.Reader, target string,
) error {
	path := plan.Path
	if target != "" {
		if !filepath.IsAbs(target) {
			return apperr.BackupInvalidConfig.WithParam("field", "target")
		}
		path = filepath.Clean(target)
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return apperr.BackupRestoreFailed.WithParam("message", trimMessage(err.Error()))
	}
	if err := backup.ExtractArchive(ctx, reader, path); err != nil {
		return apperr.BackupRestoreFailed.WithParam("message", trimMessage(err.Error()))
	}
	return nil
}

// Runs đọc lịch sử chạy của một kế hoạch.
func (s *BackupService) Runs(ctx context.Context, id uint, limit int) ([]model.BackupRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	var runs []model.BackupRun
	err := s.db.WithContext(ctx).Where("plan_id = ?", id).
		Order("started_at DESC").Limit(limit).Find(&runs).Error
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	return runs, nil
}

// cleanupLoop dọn lịch sử chạy quá cũ.
func (s *BackupService) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-backupRunRetention)
			if err := s.db.Where("started_at < ?", cutoff).Delete(&model.BackupRun{}).Error; err != nil {
				slog.Warn("không dọn được lịch sử sao lưu", "error", err)
			}
		}
	}
}

// splitList tách một danh sách ngăn cách bằng dấu phẩy.
func splitList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// trimMessage cắt bớt thông báo lỗi dài trước khi lưu hoặc trả về.
func trimMessage(message string) string {
	const limit = 2000
	message = strings.TrimSpace(message)
	if len(message) <= limit {
		return message
	}
	return message[:limit] + "…"
}
