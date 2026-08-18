package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// NodeHandler xử lý các endpoint quản lý node.
type NodeHandler struct {
	nodes *service.NodeService
	audit *service.AuditService
}

// NewNodeHandler tạo handler node.
func NewNodeHandler(nodes *service.NodeService, audit *service.AuditService) *NodeHandler {
	return &NodeHandler{nodes: nodes, audit: audit}
}

// List xử lý GET /api/v1/nodes.
func (h *NodeHandler) List(c *gin.Context) {
	nodes, err := h.nodes.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, nodes)
}

// Get xử lý GET /api/v1/nodes/:id.
func (h *NodeHandler) Get(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	node, err := h.nodes.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, node)
}

// Create xử lý POST /api/v1/nodes.
func (h *NodeHandler) Create(c *gin.Context) {
	var req service.NodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	node, err := h.nodes.Create(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "node.create", node.Name)
	response.Created(c, node)
}

// Update xử lý PUT /api/v1/nodes/:id.
func (h *NodeHandler) Update(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req service.NodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	node, err := h.nodes.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "node.update", node.Name)
	response.OK(c, node)
}

// Delete xử lý DELETE /api/v1/nodes/:id.
func (h *NodeHandler) Delete(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.nodes.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "node.delete", c.Param("id"))
	response.NoContent(c)
}

func (h *NodeHandler) record(c *gin.Context, action, resource string) {
	h.audit.Record(c.Request.Context(), service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		Action:   action,
		Resource: resource,
		IP:       c.ClientIP(),
		Success:  true,
	})
}
