package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// UserHandler xử lý các endpoint quản lý người dùng.
type UserHandler struct {
	users *service.UserService
	audit *service.AuditService
}

// NewUserHandler tạo handler quản lý người dùng.
func NewUserHandler(users *service.UserService, audit *service.AuditService) *UserHandler {
	return &UserHandler{users: users, audit: audit}
}

// List xử lý GET /api/v1/users.
func (h *UserHandler) List(c *gin.Context) {
	users, err := h.users.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, users)
}

// Get xử lý GET /api/v1/users/:id.
func (h *UserHandler) Get(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	user, err := h.users.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, user)
}

// Create xử lý POST /api/v1/users.
func (h *UserHandler) Create(c *gin.Context) {
	var req service.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	user, err := h.users.Create(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "user.create", user.Username, true)
	response.Created(c, user)
}

// Update xử lý PATCH /api/v1/users/:id.
func (h *UserHandler) Update(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req service.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	user, err := h.users.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "user.update", user.Username, true)
	response.OK(c, user)
}

// Delete xử lý DELETE /api/v1/users/:id.
func (h *UserHandler) Delete(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.users.Delete(c.Request.Context(), id, middleware.UserID(c)); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "user.delete", c.Param("id"), true)
	response.NoContent(c)
}

// ResetPassword xử lý POST /api/v1/users/:id/password.
func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req struct {
		NewPassword string `json:"newPassword" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.users.ResetPassword(c.Request.Context(), id, req.NewPassword); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "user.reset_password", c.Param("id"), true)
	response.NoContent(c)
}

// UpdatePreferences xử lý PATCH /api/v1/users/me/preferences — người dùng tự đổi
// ngôn ngữ và giao diện của chính mình, không cần quyền quản trị.
func (h *UserHandler) UpdatePreferences(c *gin.Context) {
	var req struct {
		Language *string `json:"language"`
		Theme    *string `json:"theme"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	user, err := h.users.Update(c.Request.Context(), middleware.UserID(c), service.UpdateUserRequest{
		Language: req.Language,
		Theme:    req.Theme,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, user)
}

// ListAudit xử lý GET /api/v1/audit.
func (h *UserHandler) ListAudit(c *gin.Context) {
	var q service.AuditQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	page, err := h.audit.List(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, page)
}

// ListLoginLogs xử lý GET /api/v1/audit/logins.
func (h *UserHandler) ListLoginLogs(c *gin.Context) {
	var q service.AuditQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	page, err := h.audit.ListLogins(c.Request.Context(), q)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, page)
}

func (h *UserHandler) record(c *gin.Context, action, resource string, success bool) {
	h.audit.Record(c.Request.Context(), service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		Action:   action,
		Resource: resource,
		IP:       c.ClientIP(),
		Success:  success,
	})
}
