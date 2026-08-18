// Package diskscan đo dung lượng từng thư mục con để tìm ra chỗ đang chiếm đĩa.
//
// Khi ổ đĩa gần đầy, câu hỏi duy nhất là "cái gì đang chiếm chỗ". Trả lời được
// nó bằng `du -sh *` đòi hỏi SSH và một chuỗi lệnh lặp đi lặp lại theo từng cấp
// thư mục; gói này làm đúng việc đó nhưng trả về dữ liệu để giao diện bấm sâu
// xuống từng cấp.
package diskscan

import (
	"context"
	"errors"
	"path"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// ErrNotDirectory là đường dẫn không phải thư mục.
var ErrNotDirectory = errors.New("diskscan: đường dẫn không phải thư mục")

// defaultTimeout là thời gian tối đa cho một lần quét.
//
// Quét /var trên một máy chủ đầy dữ liệu có thể mất hàng phút. Người dùng đang
// ngồi chờ trước màn hình, nên thà trả về con số gần đúng kèm dấu "chưa quét
// xong" còn hơn để trang treo không phản hồi.
const defaultTimeout = 15 * time.Second

// defaultNodeBudget là số mục tối đa được duyệt trong một lần quét.
//
// Cùng mục đích với thời gian chờ, nhưng chặn theo khối lượng công việc: một
// thư mục node_modules có thể chứa hàng trăm nghìn tệp nhỏ xíu. Đặt cao hơn
// hẳn số mục của một máy chủ thông thường (một bản Linux đủ dùng kèm vài ứng
// dụng rơi vào khoảng ba trăm nghìn) để mốc dừng thật sự là thời gian chờ,
// còn ngân sách này chỉ chặn trường hợp bệnh lý.
const defaultNodeBudget = 2_000_000

// workers là số nhánh được quét song song.
//
// Quét thư mục là công việc chờ đĩa chứ không phải tính toán, nên vài luồng đã
// đủ để lấp thời gian chờ; nhiều hơn chỉ làm ổ đĩa cơ quay như chong chóng.
const workers = 4

// pseudoDirs là các thư mục do nhân dựng ra chứ không nằm trên đĩa.
//
// /proc và /sys là cửa sổ nhìn vào nhân: tệp trong đó có kích thước bịa (nhiều
// tệp báo 0, vài tệp báo 140 TB) và số lượng thì thay đổi theo từng tiến trình
// đang chạy. Quét chúng vừa tốn phần lớn thời gian vừa cho ra con số vô nghĩa
// trên một trang mà mục đích là tìm chỗ đang thật sự chiếm đĩa.
var pseudoDirs = map[string]bool{
	"/proc": true, "/sys": true, "/dev": true, "/run": true,
	"/lost+found": true,
}

// Entry là một mục con kèm dung lượng đã tính.
type Entry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	IsDir bool   `json:"isDir"`
	// Files là số tệp bên trong, chỉ có nghĩa với thư mục.
	Files int64 `json:"files"`
	// Percent là tỉ lệ so với tổng dung lượng của thư mục đang xem.
	Percent float64 `json:"percent"`
}

// Report là kết quả quét một thư mục.
type Report struct {
	Path    string  `json:"path"`
	Total   int64   `json:"total"`
	Entries []Entry `json:"entries"`
	// Partial cho biết lần quét đã dừng sớm vì hết thời gian hoặc hết ngân sách,
	// nên các con số là mức tối thiểu chứ không phải con số cuối cùng.
	Partial bool `json:"partial"`
	// Files là tổng số tệp đã duyệt.
	Files int64 `json:"files"`
	// DurationMs là thời gian quét, để người dùng biết vì sao phải chờ.
	DurationMs int64 `json:"durationMs"`
}

// Scanner quét dung lượng qua lớp host.
type Scanner struct {
	fs      host.FileSystem
	timeout time.Duration
	budget  int64
}

// New tạo bộ quét dung lượng.
func New(fs host.FileSystem) *Scanner {
	return &Scanner{fs: fs, timeout: defaultTimeout, budget: defaultNodeBudget}
}

// Scan đo dung lượng từng mục con của một thư mục.
func (s *Scanner) Scan(ctx context.Context, dir string) (Report, error) {
	info, err := s.fs.Stat(ctx, dir)
	if err != nil {
		return Report{}, err
	}
	if !info.IsDir {
		return Report{}, ErrNotDirectory
	}

	children, err := s.fs.List(ctx, dir)
	if err != nil {
		return Report{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	started := time.Now()
	state := &scanState{budget: s.budget}

	entries := make([]Entry, len(children))
	queue := make(chan int)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range queue {
				child := children[index]
				full := path.Join(dir, child.Name)

				entry := Entry{Name: child.Name, Path: full, IsDir: child.IsDir}
				switch {
				case pseudoDirs[full]:
					entry.Size = 0
				// Liên kết mềm được tính bằng chính nó, không đi theo: đi theo thì
				// một liên kết trỏ về thư mục cha thành vòng lặp vô tận, và dung
				// lượng của thứ nó trỏ tới bị đếm hai lần.
				case child.IsLink:
					entry.Size = child.Size
				case child.IsDir:
					entry.Size, entry.Files = s.walk(ctx, full, state)
				default:
					entry.Size, entry.Files = child.Size, 1
				}
				entries[index] = entry
			}
		}()
	}

	for i := range children {
		queue <- i
	}
	close(queue)
	wg.Wait()

	report := Report{
		Path:       dir,
		Entries:    entries,
		Partial:    state.stopped.Load(),
		Files:      state.files.Load(),
		DurationMs: time.Since(started).Milliseconds(),
	}
	for _, entry := range entries {
		report.Total += entry.Size
	}

	// Mục lớn nhất lên đầu: đó là thứ duy nhất người đang dọn đĩa cần thấy.
	sort.Slice(report.Entries, func(i, j int) bool {
		return report.Entries[i].Size > report.Entries[j].Size
	})
	for i := range report.Entries {
		if report.Total > 0 {
			report.Entries[i].Percent = float64(report.Entries[i].Size) * 100 / float64(report.Total)
		}
	}
	return report, nil
}

// scanState là ngân sách dùng chung giữa các luồng quét.
type scanState struct {
	budget  int64
	visited atomic.Int64
	files   atomic.Int64
	stopped atomic.Bool
}

// take xin phép duyệt thêm một mục; trả về false khi đã hết ngân sách.
func (s *scanState) take() bool {
	if s.visited.Add(1) > s.budget {
		s.stopped.Store(true)
		return false
	}
	return true
}

// walk cộng dồn dung lượng của cả cây thư mục.
func (s *Scanner) walk(ctx context.Context, dir string, state *scanState) (int64, int64) {
	if ctx.Err() != nil {
		state.stopped.Store(true)
		return 0, 0
	}

	children, err := s.fs.List(ctx, dir)
	if err != nil {
		// Thư mục không đọc được (thiếu quyền, vừa bị xóa) không làm hỏng cả lần
		// quét: phần còn lại vẫn cho ra con số dùng được.
		return 0, 0
	}

	var size, files int64
	for _, child := range children {
		if !state.take() {
			return size, files
		}

		full := path.Join(dir, child.Name)
		switch {
		case pseudoDirs[full]:
			continue
		case child.IsLink:
			size += child.Size
		case child.IsDir:
			childSize, childFiles := s.walk(ctx, full, state)
			size += childSize
			files += childFiles
		default:
			size += child.Size
			files++
			state.files.Add(1)
		}
	}
	return size, files
}
