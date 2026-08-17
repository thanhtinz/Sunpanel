// Package monitor thu thập số liệu tài nguyên của máy chủ theo chu kỳ.
package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// Snapshot là ảnh chụp tài nguyên tại một thời điểm.
type Snapshot struct {
	Time time.Time `json:"time"`

	CPUPercent float64   `json:"cpu"`
	CPUPerCore []float64 `json:"cpuPerCore"`
	LoadAvg1   float64   `json:"load1"`
	LoadAvg5   float64   `json:"load5"`
	LoadAvg15  float64   `json:"load15"`

	MemoryTotal   uint64  `json:"memoryTotal"`
	MemoryUsed    uint64  `json:"memoryUsed"`
	MemoryPercent float64 `json:"memory"`
	SwapTotal     uint64  `json:"swapTotal"`
	SwapUsed      uint64  `json:"swapUsed"`
	SwapPercent   float64 `json:"swap"`

	Disks       []DiskUsage `json:"disks"`
	DiskPercent float64     `json:"disk"`

	// Các trường tốc độ tính bằng byte mỗi giây, suy ra từ hiệu hai lần lấy mẫu.
	NetSentRate   float64 `json:"netSent"`
	NetRecvRate   float64 `json:"netRecv"`
	DiskReadRate  float64 `json:"diskRead"`
	DiskWriteRate float64 `json:"diskWrite"`

	Uptime uint64 `json:"uptime"`
}

// DiskUsage là mức sử dụng của một điểm gắn kết.
type DiskUsage struct {
	Mountpoint string  `json:"mountpoint"`
	Device     string  `json:"device"`
	FSType     string  `json:"fstype"`
	Total      uint64  `json:"total"`
	Used       uint64  `json:"used"`
	Free       uint64  `json:"free"`
	Percent    float64 `json:"percent"`
}

// counters lưu các bộ đếm tích lũy của lần lấy mẫu trước, để tính tốc độ.
type counters struct {
	time        time.Time
	netSent     uint64
	netRecv     uint64
	diskRead    uint64
	diskWrite   uint64
	hasPrevious bool
}

// Collector thu thập số liệu và tự nhớ lần lấy mẫu trước để tính tốc độ.
// An toàn khi dùng từ nhiều goroutine.
type Collector struct {
	mu   sync.Mutex
	prev counters
}

// NewCollector tạo bộ thu thập mới.
func NewCollector() *Collector { return &Collector{} }

// Collect lấy một ảnh chụp tài nguyên.
//
// Lỗi ở từng chỉ số riêng lẻ không làm hỏng cả ảnh chụp: dashboard hiển thị
// được phần nào tốt phần đó, quan trọng hơn là báo lỗi toàn bộ.
func (c *Collector) Collect(ctx context.Context) (Snapshot, error) {
	now := time.Now()
	snap := Snapshot{Time: now}

	// CPU: gọi với khoảng thời gian 0 nghĩa là so với lần gọi trước, không chặn.
	if percents, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(percents) > 0 {
		snap.CPUPercent = round2(percents[0])
	}
	if perCore, err := cpu.PercentWithContext(ctx, 0, true); err == nil {
		snap.CPUPerCore = make([]float64, len(perCore))
		for i, v := range perCore {
			snap.CPUPerCore[i] = round2(v)
		}
	}
	if avg, err := load.AvgWithContext(ctx); err == nil {
		snap.LoadAvg1, snap.LoadAvg5, snap.LoadAvg15 = round2(avg.Load1), round2(avg.Load5), round2(avg.Load15)
	}

	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		snap.MemoryTotal, snap.MemoryUsed = vm.Total, vm.Used
		snap.MemoryPercent = round2(vm.UsedPercent)
	}
	if sm, err := mem.SwapMemoryWithContext(ctx); err == nil {
		snap.SwapTotal, snap.SwapUsed = sm.Total, sm.Used
		snap.SwapPercent = round2(sm.UsedPercent)
	}

	snap.Disks, snap.DiskPercent = collectDisks(ctx)

	c.fillRates(ctx, &snap, now)

	return snap, nil
}

// fillRates tính tốc độ mạng và đĩa từ hiệu số với lần lấy mẫu trước.
func (c *Collector) fillRates(ctx context.Context, snap *Snapshot, now time.Time) {
	cur := counters{time: now}

	if stats, err := net.IOCountersWithContext(ctx, false); err == nil && len(stats) > 0 {
		cur.netSent, cur.netRecv = stats[0].BytesSent, stats[0].BytesRecv
	}
	if stats, err := disk.IOCountersWithContext(ctx); err == nil {
		for _, s := range stats {
			cur.diskRead += s.ReadBytes
			cur.diskWrite += s.WriteBytes
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.prev.hasPrevious {
		if elapsed := now.Sub(c.prev.time).Seconds(); elapsed > 0 {
			snap.NetSentRate = rate(cur.netSent, c.prev.netSent, elapsed)
			snap.NetRecvRate = rate(cur.netRecv, c.prev.netRecv, elapsed)
			snap.DiskReadRate = rate(cur.diskRead, c.prev.diskRead, elapsed)
			snap.DiskWriteRate = rate(cur.diskWrite, c.prev.diskWrite, elapsed)
		}
	}

	cur.hasPrevious = true
	c.prev = cur
}

// rate tính tốc độ byte mỗi giây giữa hai bộ đếm tích lũy.
// Bộ đếm nhỏ đi nghĩa là nó vừa bị đặt lại (card mạng khởi động lại), trả về 0.
func rate(current, previous uint64, seconds float64) float64 {
	if current < previous {
		return 0
	}
	return round2(float64(current-previous) / seconds)
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

// Describe trả về mô tả ngắn gọn dùng cho log.
func (s Snapshot) Describe() string {
	return fmt.Sprintf("cpu=%.1f%% ram=%.1f%% disk=%.1f%%", s.CPUPercent, s.MemoryPercent, s.DiskPercent)
}
