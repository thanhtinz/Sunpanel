package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
	"github.com/thanhtinz/sunpanel/pkg/firewall"
)

// FirewallHandler xử lý các endpoint tường lửa.
type FirewallHandler struct {
	firewall *service.FirewallService
}

// NewFirewallHandler tạo handler tường lửa.
func NewFirewallHandler(fw *service.FirewallService) *FirewallHandler {
	return &FirewallHandler{firewall: fw}
}

// Status xử lý GET /api/v1/firewall/status.
func (h *FirewallHandler) Status(c *gin.Context) {
	status, err := h.firewall.Status(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, status)
}

// ListRules xử lý GET /api/v1/firewall/rules.
func (h *FirewallHandler) ListRules(c *gin.Context) {
	rules, err := h.firewall.ListRules(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, rules)
}

// AddRule xử lý POST /api/v1/firewall/rules.
func (h *FirewallHandler) AddRule(c *gin.Context) {
	var req struct {
		Action      firewall.Action   `json:"action" binding:"required"`
		Protocol    firewall.Protocol `json:"protocol" binding:"required"`
		Port        string            `json:"port"`
		Source      string            `json:"source"`
		Description string            `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	rule := firewall.Rule{
		Action:      req.Action,
		Protocol:    req.Protocol,
		Port:        req.Port,
		Source:      req.Source,
		Description: req.Description,
	}

	if err := h.firewall.AddRule(c.Request.Context(), rule, h.actor(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// DeleteRule xử lý DELETE /api/v1/firewall/rules/:id.
func (h *FirewallHandler) DeleteRule(c *gin.Context) {
	if err := h.firewall.DeleteRule(c.Request.Context(), c.Param("id"), h.actor(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// SetEnabled xử lý POST /api/v1/firewall/enabled.
func (h *FirewallHandler) SetEnabled(c *gin.Context) {
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.firewall.SetEnabled(c.Request.Context(), req.Enabled, h.actor(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

func (h *FirewallHandler) actor(c *gin.Context) service.AuditEntry {
	return service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		IP:       c.ClientIP(),
	}
}
