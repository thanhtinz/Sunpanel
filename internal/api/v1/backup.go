package v1

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// BackupHandler xử lý các endpoint sao lưu.
type BackupHandler struct {
	backups *service.BackupService
	audit   *service.AuditService
}

// NewBackupHandler tạo handler sao lưu.
func NewBackupHandler(backups *service.BackupService, audit *service.AuditService) *BackupHandler {
	return &BackupHandler{backups: backups, audit: audit}
}

// List xử lý GET /api/v1/backups.
func (h *BackupHandler) List(c *gin.Context) {
	plans, err := h.backups.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, plans)
}

// Get xử lý GET /api/v1/backups/:id.
func (h *BackupHandler) Get(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	plan, err := h.backups.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, plan)
}

// Create xử lý POST /api/v1/backups.
func (h *BackupHandler) Create(c *gin.Context) {
	var req service.BackupPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	plan, err := h.backups.Create(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "backup.create", plan.Name)
	response.Created(c, plan)
}

// Update xử lý PUT /api/v1/backups/:id.
func (h *BackupHandler) Update(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req service.BackupPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	plan, err := h.backups.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "backup.update", plan.Name)
	response.OK(c, plan)
}

// Delete xử lý DELETE /api/v1/backups/:id.
func (h *BackupHandler) Delete(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.backups.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "backup.delete", c.Param("id"))
	response.NoContent(c)
}

// SetEnabled xử lý POST /api/v1/backups/:id/enabled.
func (h *BackupHandler) SetEnabled(c *gin.Context) {
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

	plan, err := h.backups.SetEnabled(c.Request.Context(), id, req.Enabled)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "backup.set_enabled", plan.Name)
	response.OK(c, plan)
}

// Check xử lý POST /api/v1/backups/check.
//
// Kiểm nơi lưu trữ trước khi lưu kế hoạch, để người dùng biết ngay mình gõ sai
// khóa hay sai địa chỉ.
func (h *BackupHandler) Check(c *gin.Context) {
	var req service.BackupPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.backups.CheckDestination(c.Request.Context(), req); err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"ok": true})
}

// Run xử lý POST /api/v1/backups/:id/run.
func (h *BackupHandler) Run(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	run, err := h.backups.Run(c.Request.Context(), id, "manual")
	if err != nil {
		h.record(c, "backup.run_failed", c.Param("id"))
		response.Fail(c, err)
		return
	}

	h.record(c, "backup.run", run.PlanName)
	response.OK(c, run)
}

// Runs xử lý GET /api/v1/backups/:id/runs.
func (h *BackupHandler) Runs(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	runs, err := h.backups.Runs(c.Request.Context(), id, limit)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, runs)
}

// Objects xử lý GET /api/v1/backups/:id/objects.
func (h *BackupHandler) Objects(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	objects, err := h.backups.Objects(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, objects)
}

// DeleteObject xử lý DELETE /api/v1/backups/:id/objects/:object.
func (h *BackupHandler) DeleteObject(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	object := c.Param("object")
	if err := h.backups.DeleteObject(c.Request.Context(), id, object); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "backup.delete_object", object)
	response.NoContent(c)
}

// Restore xử lý POST /api/v1/backups/:id/restore.
func (h *BackupHandler) Restore(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req struct {
		Object string `json:"object" binding:"required"`
		// Target là đích khôi phục; để trống thì khôi phục về đúng chỗ đã sao lưu.
		Target string `json:"target"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	// Khôi phục ghi đè dữ liệu đang có và không hoàn tác được, nên luôn ghi nhật
	// ký kể cả khi thất bại.
	if err := h.backups.Restore(c.Request.Context(), id, req.Object, req.Target); err != nil {
		h.record(c, "backup.restore_failed", req.Object)
		response.Fail(c, err)
		return
	}

	h.record(c, "backup.restore", req.Object)
	response.OK(c, gin.H{"restored": req.Object})
}

func (h *BackupHandler) record(c *gin.Context, action, resource string) {
	h.audit.Record(c.Request.Context(), service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		Action:   action,
		Resource: resource,
		IP:       c.ClientIP(),
		Success:  true,
	})
}
