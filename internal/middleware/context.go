// Package middleware chứa các lớp trung gian HTTP của panel.
package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/model"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// Các khóa lưu trong context của Gin.
const (
	ctxClaims    = "sunpanel.claims"
	ctxRequestID = "sunpanel.requestID"
	ctxLanguage  = "sunpanel.language"
)

// Claims lấy thông tin người dùng đã xác thực từ context.
// Trả về nil nếu yêu cầu chưa qua lớp xác thực.
func Claims(c *gin.Context) *service.Claims {
	if v, ok := c.Get(ctxClaims); ok {
		if claims, ok := v.(*service.Claims); ok {
			return claims
		}
	}
	return nil
}

// UserID trả về ID người dùng đang đăng nhập, hoặc 0 nếu chưa xác thực.
func UserID(c *gin.Context) uint {
	if claims := Claims(c); claims != nil {
		return claims.UserID
	}
	return 0
}

// Username trả về tên đăng nhập, hoặc chuỗi rỗng nếu chưa xác thực.
func Username(c *gin.Context) string {
	if claims := Claims(c); claims != nil {
		return claims.Username
	}
	return ""
}

// UserRole trả về vai trò của người dùng đang đăng nhập.
func UserRole(c *gin.Context) model.Role {
	if claims := Claims(c); claims != nil {
		return claims.Role
	}
	return ""
}

// RequestID trả về mã định danh của yêu cầu, dùng để nối các dòng log với nhau.
func RequestID(c *gin.Context) string {
	if v, ok := c.Get(ctxRequestID); ok {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

// Language trả về mã ngôn ngữ mà người dùng yêu cầu.
func Language(c *gin.Context) string {
	if v, ok := c.Get(ctxLanguage); ok {
		if lang, ok := v.(string); ok {
			return lang
		}
	}
	return "vi"
}
