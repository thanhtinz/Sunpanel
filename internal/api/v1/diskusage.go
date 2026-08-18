package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// DiskHandler xử lý các endpoint phân tích dung lượng ổ đĩa.
type DiskHandler struct {
	disks *service.DiskService
}

// NewDiskHandler tạo handler phân tích dung lượng.
func NewDiskHandler(disks *service.DiskService) *DiskHandler {
	return &DiskHandler{disks: disks}
}

// Partitions xử lý GET /api/v1/disk/partitions.
func (h *DiskHandler) Partitions(c *gin.Context) {
	response.OK(c, h.disks.Partitions())
}

// Usage xử lý GET /api/v1/disk/usage?path=/var.
func (h *DiskHandler) Usage(c *gin.Context) {
	report, err := h.disks.Usage(c.Request.Context(), c.DefaultQuery("path", "/"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, report)
}
