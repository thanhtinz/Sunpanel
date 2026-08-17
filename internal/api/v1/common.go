package v1

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/thanhtinz/sunpanel/internal/apperr"
)

// uintParam đọc một tham số đường dẫn dạng số nguyên dương.
func uintParam(c *gin.Context, name string) (uint, error) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		return 0, apperr.BadRequest.Wrap(err)
	}
	return uint(value), nil
}
