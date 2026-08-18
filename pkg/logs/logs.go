// Package logs đọc tệp nhật ký của hệ thống và các dịch vụ chạy trên máy.
//
// Khi một website trả về 502 hay một dịch vụ không lên, thứ trả lời được câu
// "vì sao" nằm trong /var/log — và cho tới giờ muốn đọc nó vẫn phải mở SSH gõ
// tail. Gói này chỉ đọc, không bao giờ ghi hay xóa.
package logs

import (
	"bufio"
	"context"
	"errors"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// Các lỗi của gói.
var (
	// ErrOutsideRoot là đường dẫn nằm ngoài thư mục nhật ký cho phép.
	ErrOutsideRoot = errors.New("logs: đường dẫn nằm ngoài thư mục nhật ký")
	// ErrNotReadable là tệp không đọc được hoặc không phải tệp văn bản.
	ErrNotReadable = errors.New("logs: không đọc được tệp nhật ký")
)

// maxTailBytes giới hạn lượng dữ liệu đọc ngược từ cuối tệp.
//
// Tệp nhật ký của một máy chủ bận có thể nặng hàng GB. Đọc cả tệp để lấy hai
// trăm dòng cuối là cách chắc chắn nhất để panel ăn hết RAM của máy.
const maxTailBytes = 2 << 20 // 2 MB

// maxDepth là độ sâu tối đa khi dò tìm tệp nhật ký.
//
// /var/log có thư mục con cho từng dịch vụ (nginx, mysql), nhưng sâu hơn nữa
// thường là dữ liệu nhị phân của journald chứ không phải nhật ký đọc được.
const maxDepth = 2

// Source là một tệp nhật ký đọc được.
type Source struct {
	// Name là tên rút gọn để hiển thị, ví dụ "nginx/error.log".
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
	// ModifiedAt là lần ghi gần nhất, tính bằng mili giây Unix.
	ModifiedAt int64 `json:"modifiedAt"`
}

// Chunk là một đoạn nhật ký đã đọc.
type Chunk struct {
	Lines []string `json:"lines"`
	// Offset là vị trí kết thúc của đoạn vừa đọc, dùng cho lần đọc tiếp theo.
	Offset int64 `json:"offset"`
	// Size là kích thước tệp tại thời điểm đọc.
	Size int64 `json:"size"`
	// Truncated cho biết tệp đã bị cắt hoặc xoay vòng kể từ lần đọc trước.
	Truncated bool `json:"truncated"`
}

// Reader đọc nhật ký trong phạm vi một thư mục gốc.
type Reader struct {
	host host.Host
	root string
}

// New tạo bộ đọc nhật ký giới hạn trong thư mục root.
func New(h host.Host, root string) *Reader {
	return &Reader{host: h, root: strings.TrimRight(root, "/")}
}

// textExtensions là các đuôi tệp coi là nhật ký đọc được.
//
// Tệp .gz và .1 là bản xoay vòng đã nén hoặc đã lưu; chúng vẫn đọc được bằng
// trình quản lý tệp, còn ở đây chỉ hiện tệp đang được ghi vào.
var textExtensions = map[string]bool{".log": true, ".err": true, ".out": true, "": true}

// knownNames là các tệp nhật ký không có đuôi nhưng luôn đọc được.
var knownNames = map[string]bool{
	"syslog": true, "messages": true, "auth.log": true, "kern.log": true,
	"dpkg.log": true, "boot.log": true, "cron": true, "secure": true,
}

// Sources liệt kê các tệp nhật ký đọc được, mới ghi nhất lên đầu.
func (r *Reader) Sources(ctx context.Context) ([]Source, error) {
	out := make([]Source, 0, 16)
	r.walk(ctx, r.root, 0, &out)

	// Tệp vừa được ghi là tệp người dùng đang cần: một máy chủ có ba chục tệp
	// nhật ký, nhưng lúc đi tìm nguyên nhân sự cố thì chỉ vài tệp là đáng xem.
	sort.Slice(out, func(i, j int) bool { return out[i].ModifiedAt > out[j].ModifiedAt })
	return out, nil
}

func (r *Reader) walk(ctx context.Context, dir string, depth int, out *[]Source) {
	if depth > maxDepth {
		return
	}

	entries, err := r.host.FS().List(ctx, dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		full := path.Join(dir, entry.Name)
		switch {
		case entry.IsDir:
			r.walk(ctx, full, depth+1, out)
		case entry.IsLink:
			// Liên kết mềm trong /var/log thường trỏ ra ngoài phạm vi; bỏ qua để
			// không mời người dùng mở một tệp mà lớp host sẽ từ chối.
			continue
		case readable(entry.Name):
			*out = append(*out, Source{
				Name:       strings.TrimPrefix(strings.TrimPrefix(full, r.root), "/"),
				Path:       full,
				Size:       entry.Size,
				ModifiedAt: entry.ModTime.UnixMilli(),
			})
		}
	}
}

func readable(name string) bool {
	if knownNames[name] {
		return true
	}
	if strings.HasSuffix(name, ".gz") || strings.HasSuffix(name, ".xz") {
		return false
	}

	dot := strings.LastIndex(name, ".")
	if dot < 0 {
		return false
	}
	return textExtensions[name[dot:]]
}

// Tail đọc phần cuối tệp, tối đa lines dòng.
func (r *Reader) Tail(ctx context.Context, name string, lines int) (Chunk, error) {
	target, err := r.resolve(name)
	if err != nil {
		return Chunk{}, err
	}

	info, err := r.host.FS().Stat(ctx, target)
	if err != nil {
		return Chunk{}, err
	}

	start := int64(0)
	if info.Size > maxTailBytes {
		start = info.Size - maxTailBytes
	}

	chunk, err := r.readFrom(ctx, target, start, info.Size)
	if err != nil {
		return Chunk{}, err
	}

	// Đoạn đọc từ giữa tệp gần như luôn bắt đầu ở giữa một dòng; bỏ dòng cụt đó
	// đi thay vì hiện ra một dòng nhật ký thiếu đầu.
	if start > 0 && len(chunk.Lines) > 0 {
		chunk.Lines = chunk.Lines[1:]
	}
	if lines > 0 && len(chunk.Lines) > lines {
		chunk.Lines = chunk.Lines[len(chunk.Lines)-lines:]
	}
	return chunk, nil
}

// Since đọc phần mới thêm vào tệp kể từ vị trí offset.
//
// Nếu tệp nhỏ đi so với offset thì nó đã bị xoay vòng (logrotate) hoặc bị cắt:
// đọc lại từ đầu và báo cho bên gọi biết, để giao diện không nối nhầm nhật ký
// của tệp mới vào đuôi tệp cũ.
func (r *Reader) Since(ctx context.Context, name string, offset int64) (Chunk, error) {
	target, err := r.resolve(name)
	if err != nil {
		return Chunk{}, err
	}

	info, err := r.host.FS().Stat(ctx, target)
	if err != nil {
		return Chunk{}, err
	}

	if info.Size < offset {
		chunk, err := r.readFrom(ctx, target, 0, info.Size)
		chunk.Truncated = true
		return chunk, err
	}
	if info.Size == offset {
		return Chunk{Lines: []string{}, Offset: offset, Size: info.Size}, nil
	}
	return r.readFrom(ctx, target, offset, info.Size)
}

// readFrom đọc từ vị trí start tới hết tệp.
func (r *Reader) readFrom(ctx context.Context, target string, start, size int64) (Chunk, error) {
	reader, err := r.host.FS().Open(ctx, target)
	if err != nil {
		return Chunk{}, err
	}
	defer func() { _ = reader.Close() }()

	if start > 0 {
		seeker, ok := reader.(io.Seeker)
		if !ok {
			return Chunk{}, ErrNotReadable
		}
		if _, err := seeker.Seek(start, io.SeekStart); err != nil {
			return Chunk{}, err
		}
	}

	chunk := Chunk{Lines: make([]string, 0, 64), Offset: size, Size: size}
	scanner := bufio.NewScanner(io.LimitReader(reader, maxTailBytes))

	// Một dòng nhật ký của ứng dụng Java có thể dài vài chục nghìn ký tự; mức
	// mặc định 64 KB của Scanner sẽ làm cả lần đọc thất bại giữa chừng.
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)

	for scanner.Scan() {
		chunk.Lines = append(chunk.Lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return Chunk{}, ErrNotReadable
	}
	return chunk, nil
}

// resolve kiểm tra đường dẫn nằm trong thư mục nhật ký cho phép.
//
// Lớp host đã chặn đường dẫn thoát ra ngoài gốc của nó, nhưng gốc đó là cả ổ
// đĩa; phép kiểm tra ở đây mới là thứ giữ trình xem nhật ký đúng trong /var/log.
func (r *Reader) resolve(name string) (string, error) {
	target := name
	if !strings.HasPrefix(target, "/") {
		target = path.Join(r.root, target)
	}
	target = path.Clean(target)

	if target != r.root && !strings.HasPrefix(target, r.root+"/") {
		return "", ErrOutsideRoot
	}
	return target, nil
}
