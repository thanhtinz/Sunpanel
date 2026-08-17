package monitor

import (
	"context"
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
)

// virtualFSTypes là các hệ thống tệp ảo không phản ánh dung lượng lưu trữ thật,
// nên bị loại khỏi danh sách hiển thị.
var virtualFSTypes = map[string]bool{
	"autofs": true, "binfmt_misc": true, "bpf": true, "cgroup": true, "cgroup2": true,
	"configfs": true, "debugfs": true, "devpts": true, "devtmpfs": true, "efivarfs": true,
	"fuse.gvfsd-fuse": true, "fusectl": true, "hugetlbfs": true, "mqueue": true,
	"overlay": true, "proc": true, "pstore": true, "ramfs": true, "securityfs": true,
	"squashfs": true, "sysfs": true, "tmpfs": true, "tracefs": true,
}

// collectDisks liệt kê các phân vùng thật và trả về kèm mức sử dụng của phân
// vùng gốc, vốn là con số hiển thị nổi bật trên dashboard.
func collectDisks(ctx context.Context) ([]DiskUsage, float64) {
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		return nil, 0
	}

	var (
		out      []DiskUsage
		rootUsed float64
		seen     = make(map[string]bool)
	)

	for _, p := range partitions {
		if virtualFSTypes[strings.ToLower(p.Fstype)] {
			continue
		}
		// Cùng một thiết bị có thể được gắn ở nhiều nơi (bind mount); chỉ đếm một lần.
		if p.Device != "" && seen[p.Device] {
			continue
		}

		usage, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil || usage.Total == 0 {
			continue
		}
		seen[p.Device] = true

		out = append(out, DiskUsage{
			Mountpoint: p.Mountpoint,
			Device:     p.Device,
			FSType:     p.Fstype,
			Total:      usage.Total,
			Used:       usage.Used,
			Free:       usage.Free,
			Percent:    round2(usage.UsedPercent),
		})

		if isRootMount(p.Mountpoint) {
			rootUsed = round2(usage.UsedPercent)
		}
	}

	// Không tìm thấy phân vùng gốc thì lấy phân vùng đầy nhất làm số liệu đại diện.
	if rootUsed == 0 {
		for _, d := range out {
			if d.Percent > rootUsed {
				rootUsed = d.Percent
			}
		}
	}

	return out, rootUsed
}
