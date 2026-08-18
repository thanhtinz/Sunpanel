package v1

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// ProcessHandler xử lý các endpoint bảng tiến trình.
type ProcessHandler struct {
	processes *service.ProcessService
}

// NewProcessHandler tạo handler bảng tiến trình.
func NewProcessHandler(processes *service.ProcessService) *ProcessHandler {
	return &ProcessHandler{processes: processes}
}

// List xử lý GET /api/v1/processes?keyword=nginx.
func (h *ProcessHandler) List(c *gin.Context) {
	list, err := h.processes.List(c.Request.Context(), c.Query("keyword"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// Listeners xử lý GET /api/v1/processes/listeners.
func (h *ProcessHandler) Listeners(c *gin.Context) {
	items, err := h.processes.Listeners(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, items)
}

// Kill xử lý DELETE /api/v1/processes/:pid?force=true.
func (h *ProcessHandler) Kill(c *gin.Context) {
	pid, err := strconv.ParseInt(c.Param("pid"), 10, 32)
	if err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	err = h.processes.Kill(c.Request.Context(), int32(pid), c.Query("force") == "true", service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		IP:       c.ClientIP(),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}
