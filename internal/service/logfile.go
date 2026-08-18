package service

import (
	"context"
	"errors"
	"strings"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/pkg/logs"
)

// defaultLogLines là số dòng trả về khi giao diện không nói rõ.
const defaultLogLines = 300

// maxLogLines chặn một lần đọc quá dài.
//
// Vài nghìn dòng đã vượt xa thứ đọc được bằng mắt; muốn nhiều hơn thì tải tệp
// về bằng trình quản lý tệp chứ không nhồi vào một khung cuộn trong trình duyệt.
const maxLogLines = 5000

// LogService đọc nhật ký hệ thống cho giao diện.
type LogService struct {
	reader *logs.Reader
}

// NewLogService tạo dịch vụ đọc nhật ký.
func NewLogService(reader *logs.Reader) *LogService {
	return &LogService{reader: reader}
}

// Sources liệt kê các tệp nhật ký đọc được.
func (s *LogService) Sources(ctx context.Context) ([]logs.Source, error) {
	sources, err := s.reader.Sources(ctx)
	if err != nil {
		return nil, translateLogError(err)
	}
	if sources == nil {
		sources = []logs.Source{}
	}
	return sources, nil
}

// Tail đọc phần cuối một tệp nhật ký.
func (s *LogService) Tail(ctx context.Context, name string, lines int) (logs.Chunk, error) {
	if lines <= 0 {
		lines = defaultLogLines
	}
	if lines > maxLogLines {
		lines = maxLogLines
	}

	chunk, err := s.reader.Tail(ctx, strings.TrimSpace(name), lines)
	return chunk, translateLogError(err)
}

// Since đọc phần mới thêm vào kể từ vị trí offset.
func (s *LogService) Since(ctx context.Context, name string, offset int64) (logs.Chunk, error) {
	chunk, err := s.reader.Since(ctx, strings.TrimSpace(name), offset)
	return chunk, translateLogError(err)
}

func translateLogError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, logs.ErrOutsideRoot):
		return apperr.Forbidden.Wrap(err)
	case errors.Is(err, logs.ErrNotReadable):
		return apperr.FileNotText
	default:
		return translateFSError(err)
	}
}
