package v1

import (
	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// DatabaseHandler xử lý các endpoint cơ sở dữ liệu.
type DatabaseHandler struct {
	databases *service.DatabaseService
	audit     *service.AuditService
}

// NewDatabaseHandler tạo handler cơ sở dữ liệu.
func NewDatabaseHandler(databases *service.DatabaseService, audit *service.AuditService) *DatabaseHandler {
	return &DatabaseHandler{databases: databases, audit: audit}
}

// ListServers xử lý GET /api/v1/db/servers.
func (h *DatabaseHandler) ListServers(c *gin.Context) {
	servers, err := h.databases.ListServers(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, servers)
}

// CreateServer xử lý POST /api/v1/db/servers.
func (h *DatabaseHandler) CreateServer(c *gin.Context) {
	var req service.ServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	server, err := h.databases.CreateServer(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "db.server_create", server.Name)
	response.Created(c, server)
}

// UpdateServer xử lý PUT /api/v1/db/servers/:id.
func (h *DatabaseHandler) UpdateServer(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req service.ServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	server, err := h.databases.UpdateServer(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "db.server_update", server.Name)
	response.OK(c, server)
}

// DeleteServer xử lý DELETE /api/v1/db/servers/:id.
func (h *DatabaseHandler) DeleteServer(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.databases.DeleteServer(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "db.server_delete", c.Param("id"))
	response.NoContent(c)
}

// ListDatabases xử lý GET /api/v1/db/servers/:id/databases.
func (h *DatabaseHandler) ListDatabases(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	list, err := h.databases.ListDatabases(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// CreateDatabase xử lý POST /api/v1/db/servers/:id/databases.
func (h *DatabaseHandler) CreateDatabase(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	if err := h.databases.CreateDatabase(c.Request.Context(), id, req.Name); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "db.create", req.Name)
	response.Created(c, gin.H{"name": req.Name})
}

// DropDatabase xử lý DELETE /api/v1/db/servers/:id/databases/:name.
func (h *DatabaseHandler) DropDatabase(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	name := c.Param("name")
	if err := h.databases.DropDatabase(c.Request.Context(), id, name); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "db.drop", name)
	response.NoContent(c)
}

// ListTables xử lý GET /api/v1/db/servers/:id/databases/:name/tables.
func (h *DatabaseHandler) ListTables(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	tables, err := h.databases.ListTables(c.Request.Context(), id, c.Param("name"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, tables)
}

// ListUsers xử lý GET /api/v1/db/servers/:id/users.
func (h *DatabaseHandler) ListUsers(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	users, err := h.databases.ListUsers(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, users)
}

// CreateUser xử lý POST /api/v1/db/servers/:id/users.
func (h *DatabaseHandler) CreateUser(c *gin.Context) {
	id, req, ok := h.bindUser(c)
	if !ok {
		return
	}

	if err := h.databases.CreateUser(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "db.user_create", req.Name)
	response.Created(c, gin.H{"name": req.Name})
}

// ChangePassword xử lý POST /api/v1/db/servers/:id/users/password.
func (h *DatabaseHandler) ChangePassword(c *gin.Context) {
	id, req, ok := h.bindUser(c)
	if !ok {
		return
	}

	if err := h.databases.ChangePassword(c.Request.Context(), id, req); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "db.user_password", req.Name)
	response.OK(c, gin.H{"name": req.Name})
}

// DropUser xử lý DELETE /api/v1/db/servers/:id/users/:name.
func (h *DatabaseHandler) DropUser(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	name := c.Param("name")
	if err := h.databases.DropUser(c.Request.Context(), id, name, c.Query("host")); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "db.user_drop", name)
	response.NoContent(c)
}

func (h *DatabaseHandler) bindUser(c *gin.Context) (uint, service.DBUserRequest, bool) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return 0, service.DBUserRequest{}, false
	}

	var req service.DBUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return 0, service.DBUserRequest{}, false
	}
	return id, req, true
}

// Query xử lý POST /api/v1/db/servers/:id/query.
func (h *DatabaseHandler) Query(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req struct {
		Database  string `json:"database"`
		Statement string `json:"statement" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	result, err := h.databases.Query(c.Request.Context(), id, req.Database, req.Statement)
	if err != nil {
		// Câu lệnh chạy được cả DROP lẫn DELETE, nên mọi lần chạy đều ghi nhật ký
		// kể cả khi thất bại.
		h.record(c, "db.query_failed", req.Database)
		response.Fail(c, err)
		return
	}

	h.record(c, "db.query", req.Database)
	response.OK(c, result)
}

func (h *DatabaseHandler) record(c *gin.Context, action, resource string) {
	h.audit.Record(c.Request.Context(), service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		Action:   action,
		Resource: resource,
		IP:       c.ClientIP(),
		Success:  true,
	})
}
