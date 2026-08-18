package v1

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// CronHandler xử lý các endpoint tác vụ định kỳ.
type CronHandler struct {
	cron  *service.CronService
	audit *service.AuditService
}

// NewCronHandler tạo handler tác vụ định kỳ.
func NewCronHandler(cron *service.CronService, audit *service.AuditService) *CronHandler {
	return &CronHandler{cron: cron, audit: audit}
}

// List xử lý GET /api/v1/cron.
func (h *CronHandler) List(c *gin.Context) {
	jobs, err := h.cron.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, jobs)
}

// Get xử lý GET /api/v1/cron/:id.
func (h *CronHandler) Get(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	job, err := h.cron.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, job)
}

// Create xử lý POST /api/v1/cron.
func (h *CronHandler) Create(c *gin.Context) {
	var req service.CronJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	job, err := h.cron.Create(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "cron.create", job.Name)
	response.Created(c, job)
}

// Update xử lý PUT /api/v1/cron/:id.
func (h *CronHandler) Update(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req service.CronJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	job, err := h.cron.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "cron.update", job.Name)
	response.OK(c, job)
}

// Delete xử lý DELETE /api/v1/cron/:id.
func (h *CronHandler) Delete(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.cron.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "cron.delete", c.Param("id"))
	response.NoContent(c)
}

// SetEnabled xử lý POST /api/v1/cron/:id/enabled.
func (h *CronHandler) SetEnabled(c *gin.Context) {
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

	job, err := h.cron.SetEnabled(c.Request.Context(), id, req.Enabled)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "cron.set_enabled", job.Name)
	response.OK(c, job)
}

// RunNow xử lý POST /api/v1/cron/:id/run.
func (h *CronHandler) RunNow(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	run, err := h.cron.RunNow(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "cron.run", run.JobName)
	response.OK(c, run)
}

// Runs xử lý GET /api/v1/cron/:id/runs.
func (h *CronHandler) Runs(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	limit := 50
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			response.Fail(c, apperr.BadRequest.Wrap(err))
			return
		}
		limit = parsed
	}

	runs, err := h.cron.Runs(c.Request.Context(), id, limit)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, runs)
}

// Validate xử lý POST /api/v1/cron/validate — xem trước các lần chạy kế tiếp.
func (h *CronHandler) Validate(c *gin.Context) {
	var req struct {
		Schedule string `json:"schedule" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	next, err := h.cron.ValidateSchedule(req.Schedule, 5)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"next": next})
}

func (h *CronHandler) record(c *gin.Context, action, resource string) {
	h.audit.Record(c.Request.Context(), service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		Action:   action,
		Resource: resource,
		IP:       c.ClientIP(),
		Success:  true,
	})
}
