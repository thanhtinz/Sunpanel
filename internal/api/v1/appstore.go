package v1

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// AppStoreHandler xử lý các endpoint chợ ứng dụng.
type AppStoreHandler struct {
	apps  *service.AppStoreService
	audit *service.AuditService
}

// NewAppStoreHandler tạo handler chợ ứng dụng.
func NewAppStoreHandler(apps *service.AppStoreService, audit *service.AuditService) *AppStoreHandler {
	return &AppStoreHandler{apps: apps, audit: audit}
}

// Status xử lý GET /api/v1/apps/status.
func (h *AppStoreHandler) Status(c *gin.Context) {
	response.OK(c, h.apps.Status(c.Request.Context()))
}

// Catalog xử lý GET /api/v1/apps/catalog.
func (h *AppStoreHandler) Catalog(c *gin.Context) {
	response.OK(c, gin.H{
		"apps":       h.apps.Catalog(),
		"categories": h.apps.Categories(),
	})
}

// List xử lý GET /api/v1/apps.
func (h *AppStoreHandler) List(c *gin.Context) {
	apps, err := h.apps.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, apps)
}

// Get xử lý GET /api/v1/apps/:id.
func (h *AppStoreHandler) Get(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	app, err := h.apps.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, app)
}

// Params xử lý GET /api/v1/apps/:id/params.
//
// Trả về cả mật khẩu ứng dụng tự sinh, nên chỉ dành cho quyền vận hành và luôn
// được ghi vào nhật ký kiểm toán.
func (h *AppStoreHandler) Params(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	values, err := h.apps.Params(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "app.view_params", c.Param("id"))
	response.OK(c, values)
}

// Install xử lý POST /api/v1/apps.
func (h *AppStoreHandler) Install(c *gin.Context) {
	var req service.InstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	app, err := h.apps.Install(c.Request.Context(), req)
	if err != nil {
		// Cài thất bại vẫn ghi nhật ký: việc một ứng dụng được thử cài là thông
		// tin đáng lưu, kể cả khi nó không lên được.
		h.record(c, "app.install_failed", req.Name)
		response.Fail(c, err)
		return
	}

	h.record(c, "app.install", app.Name)
	response.Created(c, app)
}

// Control xử lý POST /api/v1/apps/:id/:action.
func (h *AppStoreHandler) Control(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	action := c.Param("action")
	if err := h.apps.Control(c.Request.Context(), id, action); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "app."+action, c.Param("id"))
	response.OK(c, gin.H{"action": action})
}

// Logs xử lý GET /api/v1/apps/:id/logs.
func (h *AppStoreHandler) Logs(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	lines, _ := strconv.Atoi(c.DefaultQuery("lines", "200"))
	out, err := h.apps.Logs(c.Request.Context(), id, lines)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"logs": out})
}

// Leftovers xử lý GET /api/v1/apps/leftovers.
func (h *AppStoreHandler) Leftovers(c *gin.Context) {
	items, err := h.apps.Leftovers(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, items)
}

// RemoveLeftover xử lý DELETE /api/v1/apps/leftovers/:name.
func (h *AppStoreHandler) RemoveLeftover(c *gin.Context) {
	name := c.Param("name")
	if err := h.apps.RemoveLeftover(c.Request.Context(), name); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "app.remove_leftover", name)
	response.NoContent(c)
}

// Uninstall xử lý DELETE /api/v1/apps/:id.
func (h *AppStoreHandler) Uninstall(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	// Xóa dữ liệu là thao tác không thể hoàn tác nên phải yêu cầu rõ ràng.
	removeData := c.Query("removeData") == "true"

	if err := h.apps.Uninstall(c.Request.Context(), id, removeData); err != nil {
		response.Fail(c, err)
		return
	}

	action := "app.uninstall"
	if removeData {
		action = "app.uninstall_with_data"
	}
	h.record(c, action, c.Param("id"))
	response.NoContent(c)
}

func (h *AppStoreHandler) record(c *gin.Context, action, resource string) {
	h.audit.Record(c.Request.Context(), service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		Action:   action,
		Resource: resource,
		IP:       c.ClientIP(),
		Success:  true,
	})
}
