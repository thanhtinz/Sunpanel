package v1

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// WebsiteHandler xử lý các endpoint website và chứng chỉ TLS.
type WebsiteHandler struct {
	websites *service.WebsiteService
	certs    *service.CertificateService
	audit    *service.AuditService
}

// NewWebsiteHandler tạo handler website.
func NewWebsiteHandler(
	websites *service.WebsiteService, certificates *service.CertificateService,
	audit *service.AuditService,
) *WebsiteHandler {
	return &WebsiteHandler{websites: websites, certs: certificates, audit: audit}
}

// Status xử lý GET /api/v1/websites/status.
func (h *WebsiteHandler) Status(c *gin.Context) {
	response.OK(c, gin.H{
		"server":    h.websites.ServerName(),
		"available": h.websites.Available(c.Request.Context()),
	})
}

// List xử lý GET /api/v1/websites.
func (h *WebsiteHandler) List(c *gin.Context) {
	sites, err := h.websites.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, sites)
}

// Get xử lý GET /api/v1/websites/:id.
func (h *WebsiteHandler) Get(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	site, err := h.websites.Get(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, site)
}

// Config xử lý GET /api/v1/websites/:id/config.
func (h *WebsiteHandler) Config(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	content, err := h.websites.Render(c.Request.Context(), id)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, gin.H{"content": content})
}

// Traffic xử lý GET /api/v1/websites/:id/traffic?window=24h.
func (h *WebsiteHandler) Traffic(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	report, err := h.websites.Traffic(c.Request.Context(), id, c.Query("window"))
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, report)
}

// Create xử lý POST /api/v1/websites.
func (h *WebsiteHandler) Create(c *gin.Context) {
	var req service.WebsiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	site, err := h.websites.Create(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "website.create", site.Name)
	response.Created(c, site)
}

// Update xử lý PUT /api/v1/websites/:id.
func (h *WebsiteHandler) Update(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req service.WebsiteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	site, err := h.websites.Update(c.Request.Context(), id, req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "website.update", site.Name)
	response.OK(c, site)
}

// Delete xử lý DELETE /api/v1/websites/:id.
func (h *WebsiteHandler) Delete(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.websites.Delete(c.Request.Context(), id); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "website.delete", c.Param("id"))
	response.NoContent(c)
}

// SetEnabled xử lý POST /api/v1/websites/:id/enabled.
func (h *WebsiteHandler) SetEnabled(c *gin.Context) {
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

	site, err := h.websites.SetEnabled(c.Request.Context(), id, req.Enabled)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "website.set_enabled", site.Name)
	response.OK(c, site)
}

// maxSourceUploadSize giới hạn dung lượng tệp mã nguồn tải lên một lần.
const maxSourceUploadSize = 2 << 30 // 2 GB

// DeploySource xử lý POST /api/v1/websites/:id/source.
//
// Nhận cả hai cách đưa mã nguồn vào: tải tệp lên trực tiếp (multipart) hoặc chỉ
// đường dẫn tới tệp nén đã có sẵn trên máy chủ. Người dùng có mã nguồn trên máy
// mình thì tải lên; người vừa tải về thẳng máy chủ bằng terminal thì chỉ đường.
func (h *WebsiteHandler) DeploySource(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	var req service.SourceRequest
	var upload *service.Upload

	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSourceUploadSize)

		req.Path = c.PostForm("path")
		req.Clean = c.PostForm("clean") == "true"
		req.KeepWrapper = c.PostForm("keepWrapper") == "true"

		if header, err := c.FormFile("file"); err == nil {
			src, err := header.Open()
			if err != nil {
				response.Fail(c, apperr.Internal.Wrap(err))
				return
			}
			defer func() { _ = src.Close() }()
			upload = &service.Upload{Name: header.Filename, Reader: src}
		}
	} else if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	result, err := h.websites.DeploySource(c.Request.Context(), id, req, upload, service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		IP:       c.ClientIP(),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// Reload xử lý POST /api/v1/websites/reload.
func (h *WebsiteHandler) Reload(c *gin.Context) {
	if err := h.websites.Reload(c.Request.Context()); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "website.reload", h.websites.ServerName())
	response.OK(c, gin.H{"reloaded": true})
}

// ListCerts xử lý GET /api/v1/certificates.
func (h *WebsiteHandler) ListCerts(c *gin.Context) {
	list, err := h.certs.List(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, list)
}

// IssueCert xử lý POST /api/v1/certificates.
func (h *WebsiteHandler) IssueCert(c *gin.Context) {
	var req service.CertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, apperr.BadRequest.Wrap(err))
		return
	}

	info, err := h.certs.Issue(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "cert.issue", req.Name)
	response.Created(c, info)
}

// RenewCert xử lý POST /api/v1/certificates/:name/renew.
func (h *WebsiteHandler) RenewCert(c *gin.Context) {
	name := c.Param("name")

	info, err := h.certs.Renew(c.Request.Context(), name)
	if err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "cert.renew", name)
	response.OK(c, info)
}

// DeleteCert xử lý DELETE /api/v1/certificates/:name.
func (h *WebsiteHandler) DeleteCert(c *gin.Context) {
	name := c.Param("name")
	ctx := c.Request.Context()

	// Chứng chỉ đang được website dùng thì không cho xóa: nginx sẽ từ chối khởi
	// động khi tệp biến mất, kéo theo mọi website khác cùng ngừng phục vụ.
	usedBy, err := h.websites.UsedCertificates(ctx, name)
	if err != nil {
		response.Fail(c, err)
		return
	}

	if err := h.certs.Delete(ctx, name, usedBy); err != nil {
		response.Fail(c, err)
		return
	}

	h.record(c, "cert.delete", name)
	response.NoContent(c)
}

func (h *WebsiteHandler) record(c *gin.Context, action, resource string) {
	h.audit.Record(c.Request.Context(), service.AuditEntry{
		UserID:   middleware.UserID(c),
		Username: middleware.Username(c),
		Action:   action,
		Resource: resource,
		IP:       c.ClientIP(),
		Success:  true,
	})
}
