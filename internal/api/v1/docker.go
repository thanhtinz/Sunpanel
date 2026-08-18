package v1

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// DockerHandler xử lý các endpoint Docker.
type DockerHandler struct {
	docker *service.DockerService
}

// NewDockerHandler tạo handler Docker.
func NewDockerHandler(docker *service.DockerService) *DockerHandler {
	return &DockerHandler{docker: docker}
}

// Status xử lý GET /api/v1/docker/status.
func (h *DockerHandler) Status(c *gin.Context) {
	response.OK(c, h.docker.Status(c.Request.Context()))
}

// ListContainers xử lý GET /api/v1/docker/containers?all=true.
func (h *DockerHandler) ListContainers(c *gin.Context) {
	all := c.DefaultQuery("all", "true") == "true"

	containers, err := h.docker.ListContainers(c.Request.Context(), all)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, containers)
}

// ControlContainer xử lý POST /api/v1/docker/containers/:id/:action.
func (h *DockerHandler) ControlContainer(c *gin.Context) {
	action := service.ContainerAction(c.Param("action"))

	err := h.docker.ControlContainer(c.Request.Context(), action, c.Param("id"), h.actor(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// ContainerLogs xử lý GET /api/v1/docker/containers/:id/logs.
func (h *DockerHandler) ContainerLogs(c *gin.Context) {
	lines := 200
	if raw := c.Query("lines"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			response.Fail(c, apperr.BadRequest.Wrap(err))
			return
		}
		lines = parsed
	}

	logs, err := h.docker.ContainerLogs(c.Request.Context(), c.Param("id"), lines)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"logs": logs})
}

// ContainerStats xử lý GET /api/v1/docker/containers/:id/stats.
func (h *DockerHandler) ContainerStats(c *gin.Context) {
	stats, err := h.docker.ContainerStats(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, stats)
}

// ListImages xử lý GET /api/v1/docker/images.
func (h *DockerHandler) ListImages(c *gin.Context) {
	images, err := h.docker.ListImages(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, images)
}

// PullImage xử lý POST /api/v1/docker/images/pull.
func (h *DockerHandler) PullImage(c *gin.Context) {
	var req struct {
		Image string `json:"image" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.docker.PullImage(c.Request.Context(), req.Image, h.actor(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// RemoveImage xử lý DELETE /api/v1/docker/images/:id.
func (h *DockerHandler) RemoveImage(c *gin.Context) {
	force := c.Query("force") == "true"

	if err := h.docker.RemoveImage(c.Request.Context(), c.Param("id"), force, h.actor(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// ListVolumes xử lý GET /api/v1/docker/volumes.
func (h *DockerHandler) ListVolumes(c *gin.Context) {
	volumes, err := h.docker.ListVolumes(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, volumes)
}

// RemoveVolume xử lý DELETE /api/v1/docker/volumes/:name.
func (h *DockerHandler) RemoveVolume(c *gin.Context) {
	if err := h.docker.RemoveVolume(c.Request.Context(), c.Param("name"), h.actor(c)); err != nil {
		response.Fail(c, err)
		return
	}
	response.NoContent(c)
}

// ListNetworks xử lý GET /api/v1/docker/networks.
func (h *DockerHandler) ListNetworks(c *gin.Context) {
	networks, err := h.docker.ListNetworks(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, networks)
}

// Prune xử lý POST /api/v1/docker/prune.
func (h *DockerHandler) Prune(c *gin.Context) {
	freed, err := h.docker.Prune(c.Request.Context(), h.actor(c))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"freed": freed})
}

func (h *DockerHandler) actor(c *gin.Context) service.AuditEntry {
	return service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		IP:       c.ClientIP(),
	}
}
