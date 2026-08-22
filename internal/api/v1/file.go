package v1

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// maxUploadSize giới hạn dung lượng một lần tải lên.
const maxUploadSize int64 = 2 << 30 // 2 GB

// FileHandler xử lý các endpoint quản lý tệp.
type FileHandler struct {
	files  *service.FileService
	audit  *service.AuditService
	tokens *service.TokenIssuer
}

// NewFileHandler tạo handler quản lý tệp.
func NewFileHandler(files *service.FileService, audit *service.AuditService, tokens *service.TokenIssuer) *FileHandler {
	return &FileHandler{files: files, audit: audit, tokens: tokens}
}

// Stat xử lý GET /api/v1/files/stat?path=…
func (h *FileHandler) Stat(c *gin.Context) {
	info, err := h.files.Stat(c.Request.Context(), c.Query("path"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, info)
}

// List xử lý GET /api/v1/files?path=/www.
func (h *FileHandler) List(c *gin.Context) {
	result, err := h.files.List(c.Request.Context(), c.DefaultQuery("path", "/"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// Read xử lý GET /api/v1/files/content?path=/www/index.html.
func (h *FileHandler) Read(c *gin.Context) {
	content, err := h.files.Read(c.Request.Context(), c.Query("path"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, content)
}

// Write xử lý PUT /api/v1/files/content.
func (h *FileHandler) Write(c *gin.Context) {
	var req struct {
		Path    string `json:"path" binding:"required"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.files.Write(c.Request.Context(), req.Path, req.Content); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "file.write", req.Path)
	response.NoContent(c)
}

// Ticket xử lý POST /api/v1/files/ticket — cấp vé tải xuống ngắn hạn.
func (h *FileHandler) Ticket(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	// Kiểm tra tệp tồn tại và đọc được ngay lúc cấp vé, để lỗi hiện ra dưới dạng
	// thông báo trong giao diện thay vì một trang tải hỏng.
	if _, err := h.files.Stat(c.Request.Context(), req.Path); err != nil {
		response.Fail(c, err)
		return
	}

	ticket, err := h.tokens.IssueDownloadTicket(middleware.UserID(c), req.Path)
	if err != nil {
		response.Fail(c, apperr.Internal.Wrap(err))
		return
	}

	response.OK(c, gin.H{"ticket": ticket})
}

// Download xử lý GET /api/v1/files/download?ticket=… — không qua lớp xác thực
// thông thường vì trình duyệt không gửi được header khi tải tệp; chính vé đã
// mang theo quyền cho đúng một đường dẫn.
func (h *FileHandler) Download(c *gin.Context) {
	claims, err := h.tokens.ParseDownloadTicket(c.Query("ticket"))
	if err != nil {
		response.Fail(c, err)
		return
	}

	reader, info, err := h.files.Open(c.Request.Context(), claims.Path)
	if err != nil {
		response.Fail(c, err)
		return
	}
	defer func() { _ = reader.Close() }()

	// Tên tệp tiếng Việt có dấu phải đi qua filename* theo RFC 5987, nếu không
	// trình duyệt sẽ lưu thành tên bị hỏng dấu.
	c.Header("Content-Disposition", fmt.Sprintf(
		`attachment; filename="%s"; filename*=UTF-8''%s`,
		sanitizeASCIIName(info.Name), url.PathEscape(info.Name),
	))
	c.Header("Content-Type", contentTypeFor(info.Name))
	c.Header("Content-Length", fmt.Sprint(info.Size))

	h.audit.Record(c.Request.Context(), service.AuditEntry{
		UserID:   claims.UserID,
		Action:   "file.download",
		Resource: info.Path,
		IP:       c.ClientIP(),
		Success:  true,
	})

	if _, err := io.Copy(c.Writer, reader); err != nil {
		// Header đã gửi đi rồi, không trả JSON lỗi được nữa; ngắt kết nối để phía
		// trình duyệt biết tệp tải về không toàn vẹn.
		c.Abort()
	}
}

// Upload xử lý POST /api/v1/files/upload (multipart).
func (h *FileHandler) Upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadSize)

	dir := c.DefaultPostForm("path", "/")
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		response.Fail(c, apperr.Internal.Wrap(err))
		return
	}
	defer func() { _ = src.Close() }()

	if err := h.files.Upload(c.Request.Context(), dir, fileHeader.Filename, src); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "file.upload", path.Join(dir, fileHeader.Filename))
	response.NoContent(c)
}

// Mkdir xử lý POST /api/v1/files/mkdir.
func (h *FileHandler) Mkdir(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.files.Mkdir(c.Request.Context(), req.Path); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "file.mkdir", req.Path)
	response.NoContent(c)
}

// Remove xử lý POST /api/v1/files/remove.
func (h *FileHandler) Remove(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.files.Remove(c.Request.Context(), req.Paths); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "file.remove", strings.Join(req.Paths, ", "))
	response.NoContent(c)
}

// Move xử lý POST /api/v1/files/move.
func (h *FileHandler) Move(c *gin.Context) {
	var req struct {
		From string `json:"from" binding:"required"`
		To   string `json:"to" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.files.Move(c.Request.Context(), req.From, req.To); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "file.move", req.From+" → "+req.To)
	response.NoContent(c)
}

// Chmod xử lý POST /api/v1/files/chmod.
func (h *FileHandler) Chmod(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
		Mode string `json:"mode" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.files.Chmod(c.Request.Context(), req.Path, req.Mode); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "file.chmod", req.Path+" "+req.Mode)
	response.NoContent(c)
}

// Compress xử lý POST /api/v1/files/compress.
func (h *FileHandler) Compress(c *gin.Context) {
	var req struct {
		Paths  []string `json:"paths" binding:"required"`
		Target string   `json:"target" binding:"required"`
		Format string   `json:"format"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	format := service.ArchiveFormat(req.Format)
	if format == "" {
		format = service.FormatZip
	}

	if err := h.files.Compress(c.Request.Context(), req.Paths, req.Target, format); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "file.compress", req.Target)
	response.NoContent(c)
}

// Formats xử lý GET /api/v1/files/formats.
func (h *FileHandler) Formats(c *gin.Context) {
	response.OK(c, service.SupportedFormats())
}

// Extract xử lý POST /api/v1/files/extract.
func (h *FileHandler) Extract(c *gin.Context) {
	var req struct {
		Path   string `json:"path" binding:"required"`
		Target string `json:"target" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	result, err := h.files.Extract(c.Request.Context(), req.Path, req.Target)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "file.extract", req.Path)
	response.OK(c, result)
}

func (h *FileHandler) record(c *gin.Context, action, resource string) {
	h.audit.Record(c.Request.Context(), service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		Action:   action,
		Resource: resource,
		IP:       c.ClientIP(),
		Success:  true,
	})
}

// contentTypeFor đoán kiểu nội dung từ phần mở rộng.
//
// Mọi tệp không nhận ra đều là octet-stream: để trình duyệt tự đoán kiểu của một
// tệp do người dùng tải lên là con đường dẫn tới tấn công XSS lưu trữ.
func contentTypeFor(name string) string {
	if ext := path.Ext(name); ext != "" {
		if ctype := mime.TypeByExtension(ext); ctype != "" {
			return ctype
		}
	}
	return "application/octet-stream"
}

// sanitizeASCIIName tạo tên dự phòng cho các trình duyệt cũ không đọc filename*.
func sanitizeASCIIName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r < 32 || r == '"' || r == '\\' || r > 126:
			b.WriteByte('_')
		default:
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "download"
	}
	return b.String()
}
