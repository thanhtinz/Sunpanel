package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// HealthHandler xử lý endpoint rà soát tình trạng máy chủ.
type HealthHandler struct {
	health *service.HealthService
}

// NewHealthHandler tạo handler rà soát.
func NewHealthHandler(health *service.HealthService) *HealthHandler {
	return &HealthHandler{health: health}
}

// Report xử lý GET /api/v1/health/report.
func (h *HealthHandler) Report(c *gin.Context) {
	response.OK(c, h.health.Check(c.Request.Context()))
}
