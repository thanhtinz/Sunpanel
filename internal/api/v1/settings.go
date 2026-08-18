package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// SettingsHandler xử lý các endpoint cấu hình của chính panel.
type SettingsHandler struct {
	settings *service.SettingsService
}

// NewSettingsHandler tạo handler cấu hình panel.
func NewSettingsHandler(settings *service.SettingsService) *SettingsHandler {
	return &SettingsHandler{settings: settings}
}

// Get xử lý GET /api/v1/settings.
func (h *SettingsHandler) Get(c *gin.Context) {
	response.OK(c, h.settings.Get())
}

// Update xử lý PUT /api/v1/settings.
func (h *SettingsHandler) Update(c *gin.Context) {
	var req service.Settings
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	result, err := h.settings.Update(c.Request.Context(), req, service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		IP:       c.ClientIP(),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// EntryPath xử lý POST /api/v1/settings/entry-path.
func (h *SettingsHandler) EntryPath(c *gin.Context) {
	entry, err := h.settings.GenerateEntryPath()
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"entryPath": entry})
}

// Restart xử lý POST /api/v1/settings/restart.
//
// Trả lời trước khi panel thật sự tắt: nếu chờ tắt xong mới trả lời thì trình
// duyệt chỉ nhận được một kết nối bị ngắt và người dùng không biết chuyện gì
// vừa xảy ra.
func (h *SettingsHandler) Restart(c *gin.Context) {
	err := h.settings.Restart(c.Request.Context(), service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		IP:       c.ClientIP(),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"url": h.settings.URL()})
}
