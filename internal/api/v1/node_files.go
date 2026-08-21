package v1

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// maxRemoteUpload giới hạn kích thước một tệp tải lên máy chủ từ xa.
const maxRemoteUpload = 2 << 30

// actor dựng thông tin người thao tác cho nhật ký kiểm toán.
func actorOf(c *gin.Context) service.AuditEntry {
	return service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		IP:       c.ClientIP(),
	}
}

// ListFiles xử lý GET /api/v1/nodes/:id/files?path=/etc
func (h *NodeHandler) ListFiles(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	entries, err := h.nodes.ListFiles(c.Request.Context(), id, c.Query("path"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, entries)
}

// ReadFile xử lý GET /api/v1/nodes/:id/files/content?path=
func (h *NodeHandler) ReadFile(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	content, err := h.nodes.ReadFile(c.Request.Context(), id, c.Query("path"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"content": content})
}

// fileAction gom các thao tác thay đổi có cùng khuôn: đọc thân yêu cầu rồi gọi
// đúng một hàm của tầng dịch vụ.
func (h *NodeHandler) fileAction(
	c *gin.Context, run func(id uint, req service.NodeFileRequest) error,
) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req service.NodeFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}
	if req.Path == "" {
		response.Fail(c, apperr.BadRequest.WithParam("field", "path"))
		return
	}

	if err := run(id, req); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// WriteFile xử lý PUT /api/v1/nodes/:id/files/content.
func (h *NodeHandler) WriteFile(c *gin.Context) {
	h.fileAction(c, func(id uint, req service.NodeFileRequest) error {
		return h.nodes.WriteFile(c.Request.Context(), id, req, actorOf(c))
	})
}

// Mkdir xử lý POST /api/v1/nodes/:id/files/mkdir.
func (h *NodeHandler) Mkdir(c *gin.Context) {
	h.fileAction(c, func(id uint, req service.NodeFileRequest) error {
		return h.nodes.Mkdir(c.Request.Context(), id, req, actorOf(c))
	})
}

// RenameFile xử lý POST /api/v1/nodes/:id/files/rename.
func (h *NodeHandler) RenameFile(c *gin.Context) {
	h.fileAction(c, func(id uint, req service.NodeFileRequest) error {
		return h.nodes.RenameFile(c.Request.Context(), id, req, actorOf(c))
	})
}

// ChmodFile xử lý POST /api/v1/nodes/:id/files/chmod.
func (h *NodeHandler) ChmodFile(c *gin.Context) {
	h.fileAction(c, func(id uint, req service.NodeFileRequest) error {
		return h.nodes.ChmodFile(c.Request.Context(), id, req, actorOf(c))
	})
}

// RemoveFile xử lý DELETE /api/v1/nodes/:id/files?path=
func (h *NodeHandler) RemoveFile(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	target := c.Query("path")
	if target == "" {
		response.Fail(c, apperr.BadRequest.WithParam("field", "path"))
		return
	}

	if err := h.nodes.RemoveFile(c.Request.Context(), id, target, actorOf(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// UploadFile xử lý POST /api/v1/nodes/:id/files/upload.
func (h *NodeHandler) UploadFile(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	header, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}
	if header.Size > maxRemoteUpload {
		response.Fail(c, apperr.FileTooLarge)
		return
	}

	source, err := header.Open()
	if err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}
	defer func() { _ = source.Close() }()

	written, err := h.nodes.UploadFile(
		c.Request.Context(), id, c.PostForm("path"), header.Filename, source, actorOf(c),
	)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"size": written})
}

// DownloadTicket xử lý POST /api/v1/nodes/:id/files/ticket.
func (h *NodeHandler) DownloadTicket(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req service.NodeFileRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Path == "" {
		response.Fail(c, apperr.BadRequest.WithParam("field", "path"))
		return
	}

	ticket, err := h.tokens.IssueNodeDownloadTicket(middleware.UserID(c), id, req.Path)
	if err != nil {
		response.Fail(c, apperr.Internal.Wrap(err))
		return
	}
	response.OK(c, gin.H{"ticket": ticket})
}

// Download xử lý GET /api/v1/nodes/files/download?ticket=… — không qua lớp xác
// thực vì trình duyệt không gửi được header khi điều hướng tới URL tải tệp.
func (h *NodeHandler) Download(c *gin.Context) {
	claims, err := h.tokens.ParseDownloadTicket(c.Query("ticket"))
	if err != nil {
		response.Fail(c, apperr.Unauthorized)
		return
	}
	if claims.NodeID == 0 {
		// Vé của tệp trên máy này không mở được tệp trên máy khác, và ngược lại.
		response.Fail(c, apperr.Unauthorized)
		return
	}

	reader, size, done, err := h.nodes.OpenFile(c.Request.Context(), claims.NodeID, claims.Path)
	if err != nil {
		response.Fail(c, err)
		return
	}
	defer done()

	name := path.Base(claims.Path)
	// Tên tệp có dấu tiếng Việt phải đi qua dạng mã hóa, nếu không trình duyệt
	// lưu về với một cái tên đầy ký tự lạ.
	c.Header("Content-Disposition", fmt.Sprintf(
		"attachment; filename*=UTF-8''%s", url.PathEscape(name),
	))
	c.Header("Content-Length", fmt.Sprint(size))
	c.Status(http.StatusOK)

	if _, err := io.Copy(c.Writer, reader); err != nil {
		// Đã gửi header rồi nên không đổi được sang phản hồi lỗi; chỉ còn cách
		// đóng kết nối để trình duyệt biết tệp tải về chưa trọn vẹn.
		c.Abort()
	}
}
