package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ArchiveDirectory nén một thư mục thành tệp .tar.gz tại đường dẫn target.
//
// Ghi thẳng ra tệp chứ không giữ trong bộ nhớ: thư mục mã nguồn website có thể
// lớn hơn nhiều lần bộ nhớ của một máy chủ nhỏ.
func ArchiveDirectory(ctx context.Context, source, target string, exclude []string) (int64, error) {
	info, err := os.Stat(source)
	if err != nil {
		return 0, fmt.Errorf("đọc thư mục nguồn: %w", err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("%s không phải thư mục", source)
	}

	file, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return 0, fmt.Errorf("tạo tệp nén: %w", err)
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)

	walkErr := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Một tệp không đọc được không nên làm hỏng cả bản sao lưu; bỏ qua nó
			// và đi tiếp, vì phần còn lại vẫn có giá trị khi cần khôi phục.
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		relative, err := filepath.Rel(source, path)
		if err != nil || relative == "." {
			return nil
		}
		if matchesExclude(relative, exclude) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Liên kết tượng trưng được lưu nguyên dạng liên kết, không đi theo nó:
		// một liên kết trỏ ra ngoài sẽ kéo cả hệ thống tệp vào bản sao lưu.
		link := ""
		if info.Mode()&os.ModeSymlink != 0 {
			if link, err = os.Readlink(path); err != nil {
				return nil
			}
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return nil
		}
		header.Name = filepath.ToSlash(relative)
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("ghi tiêu đề tệp: %w", err)
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		source, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer source.Close()

		if _, err := io.Copy(tarWriter, source); err != nil {
			return fmt.Errorf("nén tệp %s: %w", relative, err)
		}
		return nil
	})
	if walkErr != nil {
		tarWriter.Close()
		gzipWriter.Close()
		return 0, walkErr
	}

	if err := tarWriter.Close(); err != nil {
		return 0, fmt.Errorf("đóng tệp tar: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return 0, fmt.Errorf("đóng luồng nén: %w", err)
	}

	written, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("đọc kích thước tệp nén: %w", err)
	}
	return written.Size(), nil
}

// matchesExclude cho biết một đường dẫn có bị loại trừ hay không.
func matchesExclude(relative string, exclude []string) bool {
	normalized := filepath.ToSlash(relative)
	for _, pattern := range exclude {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if matched, _ := filepath.Match(pattern, filepath.Base(normalized)); matched {
			return true
		}
		// Mẫu có gạch chéo được so với cả đường dẫn tương đối, để loại được cả
		// một nhánh như "node_modules/*" hay "var/cache".
		if strings.Contains(pattern, "/") && strings.HasPrefix(normalized+"/", strings.TrimSuffix(pattern, "/")+"/") {
			return true
		}
	}
	return false
}

// ExtractArchive giải nén một tệp .tar.gz vào thư mục đích.
func ExtractArchive(ctx context.Context, r io.Reader, target string) error {
	gzipReader, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("mở luồng nén: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		header, err := tarReader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("đọc tệp nén: %w", err)
		}

		path, err := safeExtractPath(target, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("tạo thư mục: %w", err)
			}
		case tar.TypeSymlink:
			// Liên kết tượng trưng trong tệp nén có thể trỏ ra bất cứ đâu; tạo lại
			// nó nghĩa là mở một lối ghi ra ngoài thư mục đích ở lần ghi sau.
			continue
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return fmt.Errorf("tạo thư mục cha: %w", err)
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("tạo tệp: %w", err)
			}
			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return fmt.Errorf("ghi tệp: %w", err)
			}
			file.Close()
		}
	}
}

// safeExtractPath dựng đường dẫn giải nén và chặn việc ghi ra ngoài thư mục đích.
//
// Đây là lỗ hổng "zip slip": một tệp nén do người khác tạo có thể chứa mục tên
// "../../etc/cron.d/x" và ghi đè tệp hệ thống. Phải kiểm trên tên GỐC, vì
// filepath.Clean sẽ nuốt mất dấu ".." và biến nó thành đường dẫn trông vô hại.
func safeExtractPath(target, name string) (string, error) {
	normalized := strings.ReplaceAll(name, "\\", "/")
	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return "", fmt.Errorf("tệp nén chứa mục ghi ra ngoài thư mục đích: %q", name)
		}
	}
	if strings.HasPrefix(normalized, "/") {
		return "", fmt.Errorf("tệp nén chứa đường dẫn tuyệt đối: %q", name)
	}
	return filepath.Join(target, filepath.FromSlash(normalized)), nil
}

// retentionCutoff tính mốc thời gian mà bản sao lưu cũ hơn nó sẽ bị xóa.
//
// Mốc là nửa đêm của ngày cách đây keepDays ngày, chứ không phải "đúng lúc này
// trừ đi keepDays ngày". So sánh tới từng micro giây khiến bản sao lưu tạo cách
// đây vừa đúng hai ngày bị xóa khi người dùng đặt "giữ 2 ngày" — sai lệch vài
// giây quyết định việc mất một bản sao lưu không lấy lại được.
func retentionCutoff(now time.Time, keepDays int) time.Time {
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return midnight.AddDate(0, 0, -keepDays)
}

// ApplyRetention xóa các bản sao lưu cũ theo chính sách giữ lại.
//
// keepCount là số bản mới nhất luôn giữ; keepDays là số ngày giữ. Bản sao lưu bị
// xóa khi vi phạm CẢ HAI điều kiện — giữ lại nhầm còn hơn xóa nhầm, vì bản sao
// lưu đã xóa thì không lấy lại được.
func ApplyRetention(
	ctx context.Context, destination Destination, keepCount, keepDays int,
) ([]string, error) {
	if keepCount <= 0 && keepDays <= 0 {
		return nil, nil
	}

	objects, err := destination.List(ctx)
	if err != nil {
		return nil, err
	}
	// List đã xếp mới nhất lên đầu.

	var removed []string
	cutoff := retentionCutoff(time.Now(), keepDays)

	for index, object := range objects {
		tooMany := keepCount > 0 && index >= keepCount
		tooOld := keepDays > 0 && object.ModTime.Before(cutoff)

		// Chỉ đặt một trong hai ngưỡng thì ngưỡng đó quyết định; đặt cả hai thì
		// phải vi phạm cả hai mới xóa.
		var expired bool
		switch {
		case keepCount > 0 && keepDays > 0:
			expired = tooMany && tooOld
		case keepCount > 0:
			expired = tooMany
		default:
			expired = tooOld
		}

		if !expired {
			continue
		}
		if err := destination.Delete(ctx, object.Name); err != nil {
			return removed, fmt.Errorf("xóa bản sao lưu cũ %s: %w", object.Name, err)
		}
		removed = append(removed, object.Name)
	}
	return removed, nil
}
