package service

import (
	"compress/gzip"
	"io"
)

// newGzipWriter tạo bộ nén cho bản kết xuất cơ sở dữ liệu.
//
// Bản kết xuất SQL là văn bản thuần nên nén rất tốt — thường còn dưới một phần
// mười, và đó là khác biệt giữa việc lưu được ba mươi bản sao lưu hay ba bản.
func newGzipWriter(w io.Writer) *gzip.Writer { return gzip.NewWriter(w) }

// newGzipReader mở một luồng đã nén.
func newGzipReader(r io.Reader) (*gzip.Reader, error) { return gzip.NewReader(r) }
