package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// SystemUserHandler xử lý các endpoint tài khoản máy chủ và khóa SSH.
type SystemUserHandler struct {
	users *service.SystemUserService
}

// NewSystemUserHandler tạo handler tài khoản máy chủ.
func NewSystemUserHandler(users *service.SystemUserService) *SystemUserHandler {
	return &SystemUserHandler{users: users}
}

func (h *SystemUserHandler) actor(c *gin.Context) service.AuditEntry {
	return service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		IP:       c.ClientIP(),
	}
}

// Status xử lý GET /api/v1/system-users/status.
func (h *SystemUserHandler) Status(c *gin.Context) {
	response.OK(c, h.users.Status(c.Request.Context()))
}

// List xử lý GET /api/v1/system-users.
func (h *SystemUserHandler) List(c *gin.Context) {
	users, err := h.users.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, users)
}

// Create xử lý POST /api/v1/system-users.
func (h *SystemUserHandler) Create(c *gin.Context) {
	var req service.SystemUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.users.Create(c.Request.Context(), req, h.actor(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// Password xử lý POST /api/v1/system-users/:name/password.
func (h *SystemUserHandler) Password(c *gin.Context) {
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.users.SetPassword(c.Request.Context(), c.Param("name"), req.Password, h.actor(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// Locked xử lý POST /api/v1/system-users/:name/locked.
func (h *SystemUserHandler) Locked(c *gin.Context) {
	var req struct {
		Locked bool `json:"locked"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.users.SetLocked(c.Request.Context(), c.Param("name"), req.Locked, h.actor(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// Sudo xử lý POST /api/v1/system-users/:name/sudo.
func (h *SystemUserHandler) Sudo(c *gin.Context) {
	var req struct {
		Sudo bool `json:"sudo"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.users.SetSudo(c.Request.Context(), c.Param("name"), req.Sudo, h.actor(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// Delete xử lý DELETE /api/v1/system-users/:name?removeHome=true.
func (h *SystemUserHandler) Delete(c *gin.Context) {
	removeHome := c.Query("removeHome") == "true"

	if err := h.users.Delete(c.Request.Context(), c.Param("name"), removeHome, h.actor(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// Keys xử lý GET /api/v1/system-users/:name/keys.
func (h *SystemUserHandler) Keys(c *gin.Context) {
	keys, err := h.users.Keys(c.Request.Context(), c.Param("name"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, keys)
}

// AddKey xử lý POST /api/v1/system-users/:name/keys.
func (h *SystemUserHandler) AddKey(c *gin.Context) {
	var req struct {
		Key string `json:"key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	key, err := h.users.AddKey(c.Request.Context(), c.Param("name"), req.Key, h.actor(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, key)
}

// RemoveKey xử lý DELETE /api/v1/system-users/:name/keys?fingerprint=...
func (h *SystemUserHandler) RemoveKey(c *gin.Context) {
	fingerprint := c.Query("fingerprint")
	if fingerprint == "" {
		response.Fail(c, apperr.BadRequest)
		return
	}

	if err := h.users.RemoveKey(c.Request.Context(), c.Param("name"), fingerprint, h.actor(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}
