package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/pkg/appstore"
	"github.com/thanhtinz/sunpanel/pkg/compose"
	"github.com/thanhtinz/sunpanel/pkg/crypto"
	"github.com/thanhtinz/sunpanel/pkg/dockerx"
	"github.com/thanhtinz/sunpanel/pkg/host"
)

// validAppName khớp tên ứng dụng, vốn được dùng làm tên thư mục và tên dự án
// compose. Compose chỉ chấp nhận chữ thường, số, gạch ngang và gạch dưới.
var validAppName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,39}$`)

// InstallRequest là yêu cầu cài một ứng dụng.
type InstallRequest struct {
	// AppKey là định danh ứng dụng trong danh mục.
	AppKey string `json:"appKey" binding:"required"`
	// Name là tên người dùng đặt cho lần cài này.
	Name string `json:"name" binding:"required"`
	// Version là phiên bản muốn cài; để trống thì lấy phiên bản mới nhất.
	Version string `json:"version"`
	// Values là các giá trị điền vào biểu mẫu.
	Values map[string]string `json:"values"`
	Remark string            `json:"remark"`
}

// AppStatus là trạng thái hiện tại của một ứng dụng đã cài.
type AppStatus struct {
	model.InstalledApp
	// App là định nghĩa trong danh mục; rỗng nếu định nghĩa đã bị gỡ khỏi danh mục.
	App *appstore.App `json:"app,omitempty"`
	// Running cho biết container chính có đang chạy hay không.
	Running bool `json:"running"`
	// State là trạng thái Docker báo, ví dụ running, exited.
	State string `json:"state"`
}

// AppStoreService quản lý danh mục và các ứng dụng đã cài.
type AppStoreService struct {
	db      *gorm.DB
	catalog *appstore.Catalog
	compose *compose.Compose
	docker  *dockerx.Client
	host    host.Host
	sealer  *crypto.Sealer
	// root là thư mục chứa tệp cài đặt của mọi ứng dụng.
	root  string
	audit *AuditService
}

// NewAppStoreService tạo dịch vụ chợ ứng dụng.
func NewAppStoreService(
	db *gorm.DB, catalog *appstore.Catalog, composer *compose.Compose,
	docker *dockerx.Client, h host.Host, sealer *crypto.Sealer, root string, audit *AuditService,
) *AppStoreService {
	return &AppStoreService{
		db: db, catalog: catalog, compose: composer, docker: docker,
		host: h, sealer: sealer, root: root, audit: audit,
	}
}

// ComposeStatus là tình trạng của công cụ compose trên máy chủ.
type ComposeStatus struct {
	Available bool   `json:"available"`
	Version   string `json:"version"`
}

// Status cho biết máy chủ có cài được ứng dụng không.
func (s *AppStoreService) Status(ctx context.Context) ComposeStatus {
	if !s.compose.Available(ctx) {
		return ComposeStatus{}
	}
	version, err := s.compose.Version(ctx)
	if err != nil {
		slog.Warn("không đọc được phiên bản compose", "error", err)
	}
	return ComposeStatus{Available: true, Version: version}
}

// Catalog trả về danh mục ứng dụng.
func (s *AppStoreService) Catalog() []appstore.App { return s.catalog.Apps() }

// Categories trả về các nhóm ứng dụng.
func (s *AppStoreService) Categories() []string { return s.catalog.Categories() }

// List liệt kê các ứng dụng đã cài kèm trạng thái thật từ Docker.
func (s *AppStoreService) List(ctx context.Context) ([]AppStatus, error) {
	var records []model.InstalledApp
	if err := s.db.WithContext(ctx).Order("name").Find(&records).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}

	// Lấy danh sách container một lần rồi tra cứu, thay vì hỏi Docker cho từng
	// ứng dụng: một máy chủ có vài chục ứng dụng sẽ tạo vài chục lần gọi.
	states := s.containerStates(ctx)

	out := make([]AppStatus, 0, len(records))
	for _, record := range records {
		status := AppStatus{InstalledApp: record}
		if app, err := s.catalog.Get(record.AppKey); err == nil {
			status.App = &app
		}
		if state, ok := states[record.ContainerName]; ok {
			status.State = state
			status.Running = strings.EqualFold(state, "running")
		}
		out = append(out, status)
	}
	return out, nil
}

// containerStates ánh xạ tên container sang trạng thái của nó.
func (s *AppStoreService) containerStates(ctx context.Context) map[string]string {
	containers, err := s.docker.ListContainers(ctx, true)
	if err != nil {
		// Docker không chạy thì mọi ứng dụng hiện là đã dừng, chứ không phải là
		// cả danh sách hỏng.
		slog.Warn("không đọc được danh sách container", "error", err)
		return nil
	}

	states := make(map[string]string, len(containers))
	for _, container := range containers {
		states[strings.TrimPrefix(container.Name, "/")] = container.State
	}
	return states
}

// Get đọc một ứng dụng đã cài.
func (s *AppStoreService) Get(ctx context.Context, id uint) (AppStatus, error) {
	record, err := s.find(ctx, id)
	if err != nil {
		return AppStatus{}, err
	}

	status := AppStatus{InstalledApp: record}
	if app, err := s.catalog.Get(record.AppKey); err == nil {
		status.App = &app
	}
	if state, ok := s.containerStates(ctx)[record.ContainerName]; ok {
		status.State = state
		status.Running = strings.EqualFold(state, "running")
	}
	return status, nil
}

func (s *AppStoreService) find(ctx context.Context, id uint) (model.InstalledApp, error) {
	var record model.InstalledApp
	err := s.db.WithContext(ctx).First(&record, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.InstalledApp{}, apperr.AppNotFound
	}
	if err != nil {
		return model.InstalledApp{}, apperr.Internal.Wrap(err)
	}
	return record, nil
}

// Install cài một ứng dụng mới.
func (s *AppStoreService) Install(ctx context.Context, req InstallRequest) (model.InstalledApp, error) {
	if !s.compose.Available(ctx) {
		return model.InstalledApp{}, apperr.AppComposeUnavailable
	}

	name := strings.ToLower(strings.TrimSpace(req.Name))
	if !validAppName.MatchString(name) {
		return model.InstalledApp{}, apperr.AppInvalidName.WithParam("name", req.Name)
	}

	app, err := s.catalog.Get(req.AppKey)
	if err != nil {
		return model.InstalledApp{}, apperr.AppCatalogNotFound.WithParam("app", req.AppKey)
	}

	var count int64
	if err := s.db.WithContext(ctx).Model(&model.InstalledApp{}).
		Where("name = ?", name).Count(&count).Error; err != nil {
		return model.InstalledApp{}, apperr.Internal.Wrap(err)
	}
	if count > 0 {
		return model.InstalledApp{}, apperr.AppNameExists.WithParam("name", name)
	}

	version, values, err := app.ResolveVersion(req.Version, req.Values)
	if err != nil {
		return model.InstalledApp{}, translateAppError(err)
	}

	dir := filepath.Join(s.root, name)
	containerName := name

	// Gỡ ứng dụng mà giữ dữ liệu sẽ để lại cả thư mục lẫn volume. Cài đè lên đó
	// sẽ sinh mật khẩu mới trong .env trong khi cơ sở dữ liệu cũ vẫn giữ mật khẩu
	// cũ, và ứng dụng hỏng theo cách rất khó lần ra. Bắt người dùng chọn: dùng
	// tên khác, hoặc xóa hẳn dữ liệu cũ.
	if _, err := s.host.FS().Stat(ctx, dir); err == nil {
		return model.InstalledApp{}, apperr.AppDirExists.WithParam("dir", dir)
	}

	// Kiểm cổng trước khi ghi bất cứ thứ gì: compose sẽ báo lỗi "port is already
	// allocated" ở tận bước cuối, sau khi đã tải hàng trăm megabyte image.
	port, err := s.checkPorts(app, values)
	if err != nil {
		return model.InstalledApp{}, err
	}

	full := make(map[string]string, len(values)+len(appstore.ReservedVariables))
	for key, value := range values {
		full[key] = value
	}
	full["CONTAINER_NAME"] = containerName
	full["APP_KEY"] = app.Key
	full["DATA_DIR"] = dir

	if err := s.writeProject(ctx, dir, app, full); err != nil {
		return model.InstalledApp{}, err
	}

	// Kiểm tra tệp compose trước khi chạy: sai cú pháp phải lộ ra ngay chứ không
	// phải sau khi đã tải image.
	if _, err := s.compose.Config(ctx, dir); err != nil {
		s.cleanup(ctx, dir)
		return model.InstalledApp{}, apperr.AppInstallFailed.Wrap(err)
	}

	sealed, err := s.sealParams(values)
	if err != nil {
		s.cleanup(ctx, dir)
		return model.InstalledApp{}, err
	}

	record := model.InstalledApp{
		Name: name, AppKey: app.Key, Version: version.Name, Dir: dir,
		ContainerName: containerName, Port: port, Params: sealed,
		Remark: strings.TrimSpace(req.Remark),
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		s.cleanup(ctx, dir)
		return model.InstalledApp{}, apperr.Internal.Wrap(err)
	}

	if _, err := s.compose.Up(ctx, dir); err != nil {
		// Giữ lại bản ghi và tệp: người dùng cần xem được nhật ký để biết vì sao
		// hỏng, và thử lại mà không phải điền lại toàn bộ biểu mẫu.
		record.LastError = err.Error()
		s.db.WithContext(ctx).Save(&record)
		return record, apperr.AppInstallFailed.Wrap(err)
	}

	return record, nil
}

// checkPorts kiểm tra các cổng ứng dụng yêu cầu chưa bị chiếm.
//
// Trả về cổng chính của ứng dụng.
func (s *AppStoreService) checkPorts(app appstore.App, values map[string]string) (int, error) {
	for _, field := range app.Fields {
		if field.Type != appstore.FieldPort {
			continue
		}
		port, err := strconv.Atoi(values[field.Key])
		if err != nil {
			continue
		}
		if err := checkPortFree(port); err != nil {
			return 0, err
		}
	}

	if app.PortField == "" {
		return 0, nil
	}
	port, _ := strconv.Atoi(values[app.PortField])
	return port, nil
}

// checkPortFree cho biết một cổng còn trống hay không.
//
// Thử mở đúng cổng đó là cách kiểm đáng tin nhất: đọc danh sách container chỉ
// thấy cổng do Docker chiếm, còn một tiến trình thường trên máy chủ thì không.
func checkPortFree(port int) error {
	listener, err := net.Listen("tcp", net.JoinHostPort("", strconv.Itoa(port)))
	if err != nil {
		return apperr.AppPortInUse.WithParam("port", strconv.Itoa(port))
	}
	return listener.Close()
}

// writeProject ghi thư mục dự án compose của một ứng dụng.
func (s *AppStoreService) writeProject(
	ctx context.Context, dir string, app appstore.App, values map[string]string,
) error {
	fs := s.host.FS()
	if err := fs.Mkdir(ctx, dir, 0o700); err != nil {
		return apperr.AppInstallFailed.Wrap(err)
	}

	files := map[string]struct {
		content string
		mode    uint32
	}{
		"docker-compose.yml": {app.Compose, 0o600},
		// Tệp .env chứa mọi mật khẩu của ứng dụng nên chỉ chủ sở hữu đọc được.
		".env": {appstore.RenderEnv(values), 0o600},
	}
	for name, file := range files {
		path := filepath.Join(dir, name)
		if err := fs.Write(ctx, path, strings.NewReader(file.content), fileMode(file.mode)); err != nil {
			return apperr.AppInstallFailed.Wrap(err)
		}
	}
	return nil
}

// cleanup dọn thư mục dự án khi việc cài thất bại từ sớm.
func (s *AppStoreService) cleanup(ctx context.Context, dir string) {
	if err := s.host.FS().Remove(ctx, dir, true); err != nil {
		slog.Warn("không dọn được thư mục ứng dụng", "dir", dir, "error", err)
	}
}

// sealParams mã hóa các giá trị người dùng nhập để lưu vào cơ sở dữ liệu.
func (s *AppStoreService) sealParams(values map[string]string) (string, error) {
	encoded, err := json.Marshal(values)
	if err != nil {
		return "", apperr.Internal.Wrap(err)
	}
	sealed, err := s.sealer.Seal(string(encoded))
	if err != nil {
		return "", apperr.Internal.Wrap(err)
	}
	return sealed, nil
}

// Params đọc lại các giá trị đã dùng khi cài.
//
// Dùng để hiện lại mật khẩu ứng dụng tự sinh — người dùng không có cách nào khác
// để biết chúng.
func (s *AppStoreService) Params(ctx context.Context, id uint) (map[string]string, error) {
	record, err := s.find(ctx, id)
	if err != nil {
		return nil, err
	}
	if record.Params == "" {
		return map[string]string{}, nil
	}

	opened, err := s.sealer.Open(record.Params)
	if err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(opened), &values); err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	return values, nil
}

// Các thao tác điều khiển ứng dụng.
const (
	AppActionStart   = "start"
	AppActionStop    = "stop"
	AppActionRestart = "restart"
)

// Control khởi động, dừng hoặc khởi động lại một ứng dụng.
func (s *AppStoreService) Control(ctx context.Context, id uint, action string) error {
	record, err := s.find(ctx, id)
	if err != nil {
		return err
	}

	var out string
	switch action {
	case AppActionStart:
		out, err = s.compose.Start(ctx, record.Dir)
	case AppActionStop:
		out, err = s.compose.Stop(ctx, record.Dir)
	case AppActionRestart:
		out, err = s.compose.Restart(ctx, record.Dir)
	default:
		return apperr.BadRequest.WithParam("action", action)
	}
	_ = out

	if err != nil {
		record.LastError = err.Error()
		s.db.WithContext(ctx).Save(&record)
		return apperr.AppActionFailed.Wrap(err)
	}

	if record.LastError != "" {
		record.LastError = ""
		s.db.WithContext(ctx).Save(&record)
	}
	return nil
}

// Logs đọc nhật ký của một ứng dụng.
func (s *AppStoreService) Logs(ctx context.Context, id uint, lines int) (string, error) {
	record, err := s.find(ctx, id)
	if err != nil {
		return "", err
	}

	out, err := s.compose.Logs(ctx, record.Dir, lines)
	if err != nil {
		return "", apperr.AppActionFailed.Wrap(err)
	}
	return out, nil
}

// Leftover là thư mục cài đặt còn nằm lại sau khi ứng dụng đã bị gỡ.
type Leftover struct {
	// Name là tên thư mục, cũng là tên lần cài đã bị gỡ.
	Name string `json:"name"`
	Dir  string `json:"dir"`
	// ModifiedAt là lần thay đổi cuối, để đoán được nó có từ bao giờ.
	ModifiedAt time.Time `json:"modifiedAt"`
}

// Leftovers liệt kê thư mục cài đặt không còn ứng dụng nào trỏ tới.
//
// Gỡ ứng dụng mà giữ dữ liệu sẽ để lại nguyên thư mục và volume, nhưng bản ghi
// thì biến mất — panel quên hẳn nó trong khi dữ liệu vẫn chiếm chỗ, và cài lại
// đúng tên cũ thì bị chặn mà không có đường nào dọn từ giao diện. Danh sách này
// đưa chúng trở lại tầm nhìn.
func (s *AppStoreService) Leftovers(ctx context.Context) ([]Leftover, error) {
	entries, err := s.host.FS().List(ctx, s.root)
	if err != nil {
		// Chưa cài ứng dụng nào thì thư mục gốc còn chưa tồn tại.
		return []Leftover{}, nil
	}

	var installed []model.InstalledApp
	if err := s.db.WithContext(ctx).Find(&installed).Error; err != nil {
		return nil, apperr.Internal.Wrap(err)
	}
	known := make(map[string]struct{}, len(installed))
	for _, app := range installed {
		known[app.Name] = struct{}{}
	}

	out := make([]Leftover, 0)
	for _, entry := range entries {
		if !entry.IsDir {
			continue
		}
		if _, ok := known[entry.Name]; ok {
			continue
		}
		out = append(out, Leftover{
			Name:       entry.Name,
			Dir:        filepath.Join(s.root, entry.Name),
			ModifiedAt: entry.ModTime,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// RemoveLeftover xóa hẳn một thư mục còn sót lại cùng volume của nó.
func (s *AppStoreService) RemoveLeftover(ctx context.Context, name string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if !validAppName.MatchString(name) {
		return apperr.AppInvalidName.WithParam("name", name)
	}

	// Chỉ xóa được thứ panel đã quên. Một ứng dụng đang chạy phải đi qua đường
	// gỡ cài đặt, nơi container được dừng tử tế trước khi tệp biến mất.
	var count int64
	if err := s.db.WithContext(ctx).Model(&model.InstalledApp{}).
		Where("name = ?", name).Count(&count).Error; err != nil {
		return apperr.Internal.Wrap(err)
	}
	if count > 0 {
		return apperr.AppNameExists.WithParam("name", name)
	}

	dir := filepath.Join(s.root, name)
	if _, err := s.host.FS().Stat(ctx, dir); err != nil {
		return apperr.AppNotFound
	}

	// Volume vẫn còn dù container đã bị xóa từ lần gỡ trước; chạy lại down kèm
	// volume mới thu hồi được chỗ đĩa mà chúng đang giữ.
	if _, err := s.compose.Down(ctx, dir, true); err != nil {
		slog.Warn("không gỡ được volume của thư mục còn sót", "dir", dir, "error", err)
	}

	if err := s.host.FS().Remove(ctx, dir, true); err != nil {
		return apperr.AppActionFailed.Wrap(err)
	}
	return nil
}

// Uninstall gỡ một ứng dụng.
//
// removeData quyết định có xóa cả volume và thư mục dự án hay không. Mặc định
// giữ lại: gỡ nhầm ứng dụng còn cài lại được, mất dữ liệu thì không.
func (s *AppStoreService) Uninstall(ctx context.Context, id uint, removeData bool) error {
	record, err := s.find(ctx, id)
	if err != nil {
		return err
	}

	if _, err := s.compose.Down(ctx, record.Dir, removeData); err != nil {
		return apperr.AppActionFailed.Wrap(err)
	}

	if removeData {
		if err := s.host.FS().Remove(ctx, record.Dir, true); err != nil {
			slog.Warn("không xóa được thư mục ứng dụng", "dir", record.Dir, "error", err)
		}
	}

	if err := s.db.WithContext(ctx).Delete(&record).Error; err != nil {
		return apperr.Internal.Wrap(err)
	}
	return nil
}

// translateAppError đổi lỗi của gói appstore sang mã lỗi của API.
func translateAppError(err error) error {
	switch {
	case errors.Is(err, appstore.ErrMissingValue), errors.Is(err, appstore.ErrInvalidValue):
		return apperr.AppInvalidValue.Wrap(err)
	case errors.Is(err, appstore.ErrNotFound):
		return apperr.AppCatalogNotFound.Wrap(err)
	default:
		return apperr.AppInstallFailed.Wrap(err)
	}
}

// fileMode đổi số quyền sang kiểu của thư viện chuẩn.
func fileMode(mode uint32) os.FileMode { return os.FileMode(mode) }
