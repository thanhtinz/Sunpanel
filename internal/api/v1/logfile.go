package v1

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// LogHandler xử lý các endpoint xem nhật ký hệ thống.
type LogHandler struct {
	logs *service.LogService
}

// NewLogHandler tạo handler xem nhật ký.
func NewLogHandler(logs *service.LogService) *LogHandler {
	return &LogHandler{logs: logs}
}

// Sources xử lý GET /api/v1/logs.
func (h *LogHandler) Sources(c *gin.Context) {
	sources, err := h.logs.Sources(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, sources)
}

// Tail xử lý GET /api/v1/logs/content?path=&lines=&offset=
//
// Có offset thì chỉ trả phần mới thêm vào — đó là cách giao diện theo dõi trực
// tiếp mà không phải tải lại cả tệp mỗi vài giây.
func (h *LogHandler) Tail(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		response.Fail(c, apperr.BadRequest)
		return
	}

	if raw := c.Query("offset"); raw != "" {
		offset, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || offset < 0 {
			response.Fail(c, apperr.BadRequest)
			return
		}

		chunk, err := h.logs.Since(c.Request.Context(), path, offset)
		if err != nil {
			response.Fail(c, err)
			return
		}
		response.OK(c, chunk)
		return
	}

	lines, _ := strconv.Atoi(c.Query("lines"))
	chunk, err := h.logs.Tail(c.Request.Context(), path, lines)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, chunk)
}
