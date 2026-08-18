package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// APIKeyHandler xử lý các endpoint khóa API.
type APIKeyHandler struct {
	keys  *service.APIKeyService
	audit *service.AuditService
}

// NewAPIKeyHandler tạo handler khóa API.
func NewAPIKeyHandler(keys *service.APIKeyService, audit *service.AuditService) *APIKeyHandler {
	return &APIKeyHandler{keys: keys, audit: audit}
}

// List xử lý GET /api/v1/apikeys.
func (h *APIKeyHandler) List(c *gin.Context) {
	// Quản trị viên thấy mọi khóa trên hệ thống; người khác chỉ thấy khóa của mình.
	all := middleware.UserRole(c) == model.RoleAdmin
	keys, err := h.keys.List(c.Request.Context(), middleware.UserID(c), all)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, keys)
}

// Create xử lý POST /api/v1/apikeys.
func (h *APIKeyHandler) Create(c *gin.Context) {
	// Một khóa bị lộ không được dùng để tự nhân bản thêm khóa mới; việc tạo khóa
	// chỉ cho phép từ phiên đăng nhập thật.
	if middleware.IsAPIKey(c) {
		response.Fail(c, apperr.Forbidden)
		return
	}

	var req service.APIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	created, err := h.keys.Create(
		c.Request.Context(), middleware.UserID(c), middleware.UserRole(c), req,
	)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "apikey.create", created.Name)
	response.Created(c, created)
}

// SetEnabled xử lý POST /api/v1/apikeys/:id/enabled.
func (h *APIKeyHandler) SetEnabled(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	key, err := h.keys.SetEnabled(c.Request.Context(), id, h.scope(c), req.Enabled)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "apikey.set_enabled", key.Name)
	response.OK(c, key)
}

// Delete xử lý DELETE /api/v1/apikeys/:id.
func (h *APIKeyHandler) Delete(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.keys.Delete(c.Request.Context(), id, h.scope(c)); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "apikey.delete", c.Param("id"))
	response.NoContent(c)
}

// scope trả về giới hạn chủ sở hữu cho thao tác hiện tại.
//
// Quản trị viên thao tác được với mọi khóa (trả về 0 nghĩa là không giới hạn);
// người dùng khác chỉ đụng được vào khóa của chính mình.
func (h *APIKeyHandler) scope(c *gin.Context) uint {
	if middleware.UserRole(c) == model.RoleAdmin {
		return 0
	}
	return middleware.UserID(c)
}

func (h *APIKeyHandler) record(c *gin.Context, action, resource string) {
	h.audit.Record(c.Request.Context(), service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		Action:   action,
		Resource: resource,
		IP:       c.ClientIP(),
		Success:  true,
	})
}
