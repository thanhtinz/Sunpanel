package sshx

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Metrics là mức sử dụng tài nguyên tại một thời điểm.
type Metrics struct {
	CPUPercent    float64 `json:"cpu"`
	MemoryPercent float64 `json:"memory"`
	DiskPercent   float64 `json:"disk"`
	Load1         float64 `json:"load1"`
}

// cpuSampleGap là khoảng cách giữa hai lần đọc /proc/stat.
//
// Mức dùng CPU là hiệu của hai lần đọc chứ không phải một con số đọc được
// thẳng: /proc/stat đếm tổng thời gian từ lúc máy khởi động, nên đọc một lần
// chỉ cho ra mức trung bình kể từ ngày bật máy.
const cpuSampleGap = 300 * time.Millisecond

// metricsCommand đọc hai lần /proc/stat cùng bộ nhớ và ổ đĩa trong một lần chạy.
//
// Dựng từ cpuSampleGap chứ không viết cứng số giây: khoảng cách giữa hai lần
// đọc là thứ quyết định con số CPU, nên nó chỉ được khai ở đúng một chỗ.
var metricsCommand = fmt.Sprintf(`
awk '/^cpu /{print "SP_CPU1=" $2+$3+$4+$6+$7+$8 " " $5}' /proc/stat 2>/dev/null
sleep %.1f
awk '/^cpu /{print "SP_CPU2=" $2+$3+$4+$6+$7+$8 " " $5}' /proc/stat 2>/dev/null
awk '/^MemTotal:/{t=$2} /^MemAvailable:/{a=$2} END{print "SP_MEM=" t " " a}' /proc/meminfo 2>/dev/null
df -kP / 2>/dev/null | awk 'NR==2{print "SP_DISK=" $2 " " $3}'
echo "SP_LOAD=$(cut -d' ' -f1 /proc/loadavg 2>/dev/null)"
`, cpuSampleGap.Seconds())

// Metrics đo mức sử dụng tài nguyên của máy chủ từ xa.
func (c *Client) Metrics(ctx context.Context) (Metrics, error) {
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	result, err := c.Run(ctx, metricsCommand)
	if err != nil {
		return Metrics{}, err
	}
	return parseMetrics(result.Stdout), nil
}

// parseMetrics đọc kết quả của metricsCommand.
func parseMetrics(output string) Metrics {
	var metrics Metrics
	var busy1, idle1, busy2, idle2 int64

	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}

		switch key {
		case "SP_CPU1":
			busy1, idle1 = twoNumbers(value)
		case "SP_CPU2":
			busy2, idle2 = twoNumbers(value)
		case "SP_MEM":
			total, available := twoNumbers(value)
			if total > 0 {
				metrics.MemoryPercent = percent(total-available, total)
			}
		case "SP_DISK":
			total, used := twoNumbers(value)
			if total > 0 {
				metrics.DiskPercent = percent(used, total)
			}
		case "SP_LOAD":
			metrics.Load1, _ = strconv.ParseFloat(strings.TrimSpace(value), 64)
		}
	}

	if total := (busy2 + idle2) - (busy1 + idle1); total > 0 {
		metrics.CPUPercent = percent(busy2-busy1, total)
	}
	return metrics
}

// percent tính tỉ lệ phần trăm và kẹp vào khoảng 0-100.
//
// Số liệu đọc từ một máy khác không phải lúc nào cũng nhất quán: hai lần đọc
// /proc/stat có thể rơi vào lúc nhân vừa đặt lại bộ đếm, và một cột âm vẽ ra
// biểu đồ lộn ngược.
func percent(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	value := float64(part) * 100 / float64(total)
	switch {
	case value < 0:
		return 0
	case value > 100:
		return 100
	default:
		return value
	}
}
