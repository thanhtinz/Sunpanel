// Package response chuẩn hóa định dạng phản hồi JSON của API.
package response

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
)

// Body là khung phản hồi chung cho mọi endpoint.
type Body struct {
	// Success cho biết yêu cầu có thành công hay không.
	Success bool `json:"success"`
	// Data là dữ liệu trả về khi thành công.
	Data any `json:"data,omitempty"`
	// Code là mã lỗi để giao diện tự dịch sang ngôn ngữ người dùng.
	Code string `json:"code,omitempty"`
	// Params là các tham số chèn vào chuỗi dịch.
	Params map[string]any `json:"params,omitempty"`
	// RequestID giúp đối chiếu phản hồi với log máy chủ khi cần hỗ trợ.
	RequestID string `json:"requestId,omitempty"`
}

// OK trả về phản hồi thành công kèm dữ liệu.
func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Success: true, Data: data, RequestID: requestID(c)})
}

// Created trả về phản hồi tạo mới thành công.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Body{Success: true, Data: data, RequestID: requestID(c)})
}

// NoContent trả về phản hồi thành công không có dữ liệu.
func NoContent(c *gin.Context) {
	c.JSON(http.StatusOK, Body{Success: true, RequestID: requestID(c)})
}

// Fail trả về phản hồi lỗi.
//
// Chỉ mã lỗi được gửi ra ngoài; chi tiết kỹ thuật của lỗi gốc chỉ đi vào log máy
// chủ, tránh rò rỉ cấu trúc nội bộ cho kẻ đang dò tìm điểm yếu.
func Fail(c *gin.Context, err error) {
	appErr := apperr.From(err)

	if appErr.Status >= http.StatusInternalServerError {
		slog.Error("yêu cầu thất bại",
			"code", appErr.Code,
			"path", c.Request.URL.Path,
			"requestId", requestID(c),
			"error", appErr.Err)
	}

	c.JSON(appErr.Status, Body{
		Success:   false,
		Code:      appErr.Code,
		Params:    appErr.Params,
		RequestID: requestID(c),
	})
}

func requestID(c *gin.Context) string {
	if v, ok := c.Get("sunpanel.requestID"); ok {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}
