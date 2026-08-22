// Package procs liệt kê tiến trình đang chạy và kết thúc chúng khi cần.
//
// Bảng tiến trình là thứ đầu tiên người quản trị mở khi máy chủ chậm bất
// thường, và cho tới giờ panel chỉ nói được "CPU 90%" mà không nói được ai
// đang ăn hết. Gói này đứng cùng lớp với pkg/monitor: nó chạm thẳng vào hệ
// điều hành để lớp internal/service không phải làm việc đó.
package procs

import (
	"context"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// ErrProtected trả về khi tiến trình nằm trong nhóm không được phép kết thúc.
var ErrProtected = errors.New("procs: tiến trình được bảo vệ")

// Process là một dòng trong bảng tiến trình.
type Process struct {
	PID      int32  `json:"pid"`
	PPID     int32  `json:"ppid"`
	Name     string `json:"name"`
	Username string `json:"username"`
	// Command là dòng lệnh đầy đủ, đã cắt bớt nếu quá dài.
	Command string `json:"command"`
	Status  string `json:"status"`
	// CPUPercent là phần trăm của một lõi, tính từ hiệu hai lần lấy mẫu.
	CPUPercent    float64 `json:"cpu"`
	MemoryPercent float64 `json:"memoryPercent"`
	MemoryRSS     uint64  `json:"memoryRss"`
	Threads       int32   `json:"threads"`
	// Started là thời điểm khởi chạy, tính bằng mili giây Unix.
	Started int64 `json:"started"`
	// Protected đánh dấu tiến trình mà panel từ chối kết thúc.
	Protected bool `json:"protected"`
}

// Listener là một cổng đang được lắng nghe.
type Listener struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     uint32 `json:"port"`
	PID      int32  `json:"pid"`
	Process  string `json:"process"`
}

// maxCommandLength giới hạn độ dài dòng lệnh gửi xuống giao diện.
//
// Một tiến trình Java hay Node có thể mang dòng lệnh dài vài nghìn ký tự;
// gửi nguyên vẹn cho hàng trăm tiến trình thì phần lớn dữ liệu chỉ để bảng
// hiển thị dấu ba chấm.
const maxCommandLength = 512

// Sampler lấy mẫu bảng tiến trình và nhớ lần trước để tính phần trăm CPU.
//
// gopsutil chỉ tính được phần trăm CPU tức thời khi có hai mốc để trừ nhau.
// Không nhớ mốc trước thì con số trả về là trung bình từ lúc tiến trình khởi
// chạy — một tiến trình ngốn CPU suốt đêm qua rồi ngủ yên vẫn hiện 90%, đúng
// lúc người dùng cần biết cái gì đang ăn CPU *bây giờ*.
type Sampler struct {
	mu     sync.Mutex
	last   map[int32]sample
	lastAt time.Time
	// self là PID của chính panel, không cho phép tự kết thúc.
	self int32
}

type sample struct {
	cpuSeconds float64
	started    int64
}

// NewSampler tạo bộ lấy mẫu tiến trình.
func NewSampler() *Sampler {
	// PID nằm gọn trong int32 trên mọi nền tảng panel chạy; đây là số của chính
	// tiến trình này, không phải dữ liệu bên ngoài.
	return &Sampler{last: make(map[int32]sample), self: int32(os.Getpid())} //nolint:gosec
}

// protected cho biết tiến trình có được phép kết thúc từ giao diện hay không.
func (s *Sampler) protected(pid int32) bool {
	return pid <= 1 || pid == s.self
}

// List trả về toàn bộ tiến trình đang chạy.
func (s *Sampler) List(ctx context.Context) ([]Process, error) {
	all, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	s.mu.Lock()
	previous, previousAt := s.last, s.lastAt
	current := make(map[int32]sample, len(all))
	s.mu.Unlock()

	elapsed := now.Sub(previousAt).Seconds()
	if previousAt.IsZero() {
		elapsed = 0
	}

	out := make([]Process, 0, len(all))
	for _, p := range all {
		item, snap, ok := describe(ctx, p)
		if !ok {
			continue
		}
		current[item.PID] = snap

		// Chỉ tính được phần trăm khi lần trước cũng thấy đúng tiến trình này:
		// PID được dùng lại sau khi tiến trình cũ chết, nên phải so cả thời điểm
		// khởi chạy, nếu không một PID vừa tái sinh sẽ hiện phần trăm âm.
		if before, seen := previous[item.PID]; seen && elapsed > 0 && before.started == snap.started {
			item.CPUPercent = (snap.cpuSeconds - before.cpuSeconds) / elapsed * 100
			if item.CPUPercent < 0 {
				item.CPUPercent = 0
			}
		}
		item.Protected = s.protected(item.PID)
		out = append(out, item)
	}

	s.mu.Lock()
	s.last, s.lastAt = current, now
	s.mu.Unlock()

	sort.Slice(out, func(i, j int) bool {
		if out[i].CPUPercent != out[j].CPUPercent {
			return out[i].CPUPercent > out[j].CPUPercent
		}
		return out[i].MemoryRSS > out[j].MemoryRSS
	})
	return out, nil
}

// describe đọc thông tin một tiến trình, bỏ qua nếu nó vừa kết thúc.
func describe(ctx context.Context, p *process.Process) (Process, sample, bool) {
	// Tên là trường tối thiểu: đọc không được nghĩa là tiến trình đã biến mất
	// giữa lúc liệt kê và lúc đọc, chuyện hoàn toàn bình thường.
	name, err := p.NameWithContext(ctx)
	if err != nil {
		return Process{}, sample{}, false
	}

	item := Process{PID: p.Pid, Name: name}
	if ppid, err := p.PpidWithContext(ctx); err == nil {
		item.PPID = ppid
	}
	if username, err := p.UsernameWithContext(ctx); err == nil {
		item.Username = username
	}
	if status, err := p.StatusWithContext(ctx); err == nil && len(status) > 0 {
		item.Status = status[0]
	}
	if threads, err := p.NumThreadsWithContext(ctx); err == nil {
		item.Threads = threads
	}
	if cmdline, err := p.CmdlineWithContext(ctx); err == nil && cmdline != "" {
		item.Command = truncate(cmdline)
	} else {
		item.Command = name
	}
	if mem, err := p.MemoryInfoWithContext(ctx); err == nil && mem != nil {
		item.MemoryRSS = mem.RSS
	}
	if percent, err := p.MemoryPercentWithContext(ctx); err == nil {
		item.MemoryPercent = float64(percent)
	}

	snap := sample{}
	if created, err := p.CreateTimeWithContext(ctx); err == nil {
		item.Started = created
		snap.started = created
	}
	if times, err := p.TimesWithContext(ctx); err == nil && times != nil {
		snap.cpuSeconds = times.User + times.System
	}
	return item, snap, true
}

func truncate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= maxCommandLength {
		return value
	}
	return value[:maxCommandLength] + "…"
}

// Kill kết thúc một tiến trình. force gửi SIGKILL thay vì SIGTERM.
//
// Mặc định là tín hiệu lịch sự: SIGTERM cho tiến trình cơ hội đóng tệp và ghi
// nốt dữ liệu. SIGKILL chỉ dành cho khi tiến trình không còn phản hồi.
func (s *Sampler) Kill(ctx context.Context, pid int32, force bool) error {
	if s.protected(pid) {
		return ErrProtected
	}

	p, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return err
	}
	if force {
		return p.KillWithContext(ctx)
	}
	return p.TerminateWithContext(ctx)
}

// Listeners liệt kê các cổng đang được lắng nghe kèm tiến trình sở hữu.
func (s *Sampler) Listeners(ctx context.Context) ([]Listener, error) {
	connections, err := net.ConnectionsWithContext(ctx, "inet")
	if err != nil {
		return nil, err
	}

	names := make(map[int32]string)
	out := make([]Listener, 0, 16)
	for _, conn := range connections {
		// UDP không có trạng thái LISTEN; một socket UDP không có đầu kia đã là
		// một cổng đang mở, nên nó cũng phải xuất hiện trong bảng này.
		udp := conn.Type == 2
		if !udp && conn.Status != "LISTEN" {
			continue
		}
		if udp && conn.Raddr.Port != 0 {
			continue
		}

		item := Listener{
			Protocol: protocolName(conn.Family, conn.Type),
			Address:  conn.Laddr.IP,
			Port:     conn.Laddr.Port,
			PID:      conn.Pid,
		}
		if conn.Pid > 0 {
			name, seen := names[conn.Pid]
			if !seen {
				if p, err := process.NewProcessWithContext(ctx, conn.Pid); err == nil {
					name, _ = p.NameWithContext(ctx)
				}
				names[conn.Pid] = name
			}
			item.Process = name
		}
		out = append(out, item)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Port != out[j].Port {
			return out[i].Port < out[j].Port
		}
		return out[i].Protocol < out[j].Protocol
	})
	return out, nil
}

func protocolName(family, kind uint32) string {
	protocol := "tcp"
	if kind == 2 {
		protocol = "udp"
	}
	if family == 10 {
		protocol += "6"
	}
	return protocol
}
