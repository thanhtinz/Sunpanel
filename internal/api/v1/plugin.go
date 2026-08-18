package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// PluginHandler xử lý các endpoint plugin.
type PluginHandler struct {
	plugins *service.PluginService
	tokens  *service.TokenIssuer
	audit   *service.AuditService
}

// NewPluginHandler tạo handler plugin.
func NewPluginHandler(
	plugins *service.PluginService, tokens *service.TokenIssuer, audit *service.AuditService,
) *PluginHandler {
	return &PluginHandler{plugins: plugins, tokens: tokens, audit: audit}
}

// Ticket xử lý POST /api/v1/plugins/:key/ticket.
//
// Cấp vé ngắn hạn để khung nhúng mở được giao diện plugin: khung nhúng không đặt
// được header Authorization.
func (h *PluginHandler) Ticket(c *gin.Context) {
	key := c.Param("key")

	manifest, err := h.plugins.Get(key)
	if err != nil {
		response.Fail(c, err)
		return
	}

	role := middleware.UserRole(c)
	if !h.plugins.Allowed(manifest, role) {
		response.Fail(c, apperr.Forbidden)
		return
	}

	ticket, err := h.tokens.IssuePluginTicket(
		middleware.UserID(c), middleware.Username(c), string(role), manifest.Key,
	)
	if err != nil {
		response.Fail(c, apperr.Internal.Wrap(err))
		return
	}
	response.OK(c, gin.H{"ticket": ticket})
}

// List xử lý GET /api/v1/plugins.
func (h *PluginHandler) List(c *gin.Context) {
	role := middleware.UserRole(c)

	// Chỉ liệt kê plugin mà người dùng hiện tại được phép dùng: hiện một mục menu
	// rồi chặn ở lần bấm là cách làm khó chịu và không thêm bảo mật gì.
	manifests := h.plugins.Enabled()
	visible := make([]any, 0, len(manifests))
	for _, manifest := range manifests {
		if h.plugins.Allowed(manifest, role) {
			visible = append(visible, manifest)
		}
	}

	response.OK(c, gin.H{"plugins": visible, "dir": h.plugins.Dir()})
}

// ListAll xử lý GET /api/v1/plugins/all — gồm cả plugin đang tắt.
func (h *PluginHandler) ListAll(c *gin.Context) {
	response.OK(c, gin.H{"plugins": h.plugins.List(), "dir": h.plugins.Dir()})
}

// Reload xử lý POST /api/v1/plugins/reload.
func (h *PluginHandler) Reload(c *gin.Context) {
	if err := h.plugins.Reload(c.Request.Context()); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "plugin.reload", "")
	response.OK(c, gin.H{"plugins": h.plugins.List()})
}

// Proxy xử lý mọi yêu cầu dưới /api/v1/plugins/:key/proxy/*path.
func (h *PluginHandler) Proxy(c *gin.Context) {
	key := c.Param("key")

	manifest, err := h.plugins.Get(key)
	if err != nil {
		response.Fail(c, err)
		return
	}

	// Danh tính lấy từ phiên đăng nhập, hoặc từ vé khi yêu cầu đến từ khung nhúng.
	userID, username, role := middleware.UserID(c), middleware.Username(c), middleware.UserRole(c)
	if role == "" {
		claims, err := h.tokens.ParsePluginTicket(c.Query("ticket"))
		if err != nil || claims.Plugin != key {
			response.Fail(c, apperr.Unauthorized)
			return
		}
		userID, username, role = claims.UserID, claims.Username, model.Role(claims.Role)
	}

	if !h.plugins.Allowed(manifest, role) {
		response.Fail(c, apperr.Forbidden)
		return
	}

	prefix := "/api/v1/plugins/" + key + "/proxy"
	if err := h.plugins.Forward(
		c.Writer, c.Request, manifest, prefix, username, role, userID,
	); err != nil {
		response.Fail(c, err)
	}
}

func (h *PluginHandler) record(c *gin.Context, action, resource string) {
	h.audit.Record(c.Request.Context(), service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		Action:   action,
		Resource: resource,
		IP:       c.ClientIP(),
		Success:  true,
	})
}
