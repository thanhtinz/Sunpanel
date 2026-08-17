// Package v1 hiện thực các endpoint HTTP phiên bản 1 của SunPanel.
//
// Handler ở đây cố tình mỏng: chỉ đọc dữ liệu vào, gọi service, rồi trả phản hồi.
// Toàn bộ nghiệp vụ nằm ở internal/service.
package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// AuthHandler xử lý các endpoint xác thực.
type AuthHandler struct {
	auth  *service.AuthService
	users *service.UserService
}

// NewAuthHandler tạo handler xác thực.
func NewAuthHandler(auth *service.AuthService, users *service.UserService) *AuthHandler {
	return &AuthHandler{auth: auth, users: users}
}

// Login xử lý POST /api/v1/auth/login.
func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	// IP và User-Agent luôn lấy từ kết nối, không bao giờ tin dữ liệu người gửi khai.
	req.IP = c.ClientIP()
	req.UserAgent = c.Request.UserAgent()

	pair, err := h.auth.Login(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, pair)
}

// refreshRequest là thân yêu cầu của endpoint làm mới và đăng xuất.
type refreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// Refresh xử lý POST /api/v1/auth/refresh.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	pair, err := h.auth.Refresh(c.Request.Context(), req.RefreshToken, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, pair)
}

// Logout xử lý POST /api/v1/auth/logout.
func (h *AuthHandler) Logout(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.auth.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// Me xử lý GET /api/v1/auth/me.
func (h *AuthHandler) Me(c *gin.Context) {
	user, err := h.users.Get(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, user)
}

// ListSessions xử lý GET /api/v1/auth/sessions.
func (h *AuthHandler) ListSessions(c *gin.Context) {
	sessions, err := h.auth.ListSessions(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, sessions)
}

// RevokeSession xử lý DELETE /api/v1/auth/sessions/:id.
func (h *AuthHandler) RevokeSession(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.auth.RevokeSession(c.Request.Context(), middleware.UserID(c), id); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// BeginTOTP xử lý POST /api/v1/auth/totp/setup.
func (h *AuthHandler) BeginTOTP(c *gin.Context) {
	setup, err := h.auth.BeginTOTPSetup(c.Request.Context(), middleware.UserID(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, setup)
}

// ConfirmTOTP xử lý POST /api/v1/auth/totp/confirm.
func (h *AuthHandler) ConfirmTOTP(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.auth.ConfirmTOTPSetup(c.Request.Context(), middleware.UserID(c), req.Code); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// DisableTOTP xử lý POST /api/v1/auth/totp/disable.
func (h *AuthHandler) DisableTOTP(c *gin.Context) {
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.auth.DisableTOTP(c.Request.Context(), middleware.UserID(c), req.Password); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// ChangePassword xử lý POST /api/v1/auth/password.
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req struct {
		CurrentPassword string `json:"currentPassword" binding:"required"`
		NewPassword     string `json:"newPassword" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	err := h.users.ChangePassword(c.Request.Context(), middleware.UserID(c), req.CurrentPassword, req.NewPassword)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}
