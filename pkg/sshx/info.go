package sshx

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// Info là thông tin tóm tắt của máy chủ từ xa.
type Info struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Kernel   string `json:"kernel"`
	Arch     string `json:"arch"`
	// CPUCores là số nhân, Load1 là tải trung bình một phút.
	CPUCores int     `json:"cpuCores"`
	Load1    float64 `json:"load1"`
	// UptimeSeconds là thời gian máy đã chạy.
	UptimeSeconds int64 `json:"uptimeSeconds"`
	// Bộ nhớ và ổ đĩa tính bằng byte.
	MemoryTotal int64 `json:"memoryTotal"`
	MemoryUsed  int64 `json:"memoryUsed"`
	DiskTotal   int64 `json:"diskTotal"`
	DiskUsed    int64 `json:"diskUsed"`
}

// infoCommand đọc mọi thứ cần biết trong đúng một lần chạy.
//
// Gộp thành một lệnh chứ không chạy tám lệnh: mỗi lần mở phiên SSH là một vòng
// đi về qua mạng, và với một máy chủ ở nước ngoài thì tám vòng đủ để người dùng
// thấy trang đứng hình.
//
// Mọi phần đọc được đều có thể vắng mặt trên một bản Linux gọn nhẹ, nên lệnh
// nào cũng nuốt lỗi và trường tương ứng chỉ đơn giản là để trống.
const infoCommand = `
echo "SP_HOST=$(hostname 2>/dev/null)"
echo "SP_KERNEL=$(uname -r 2>/dev/null)"
echo "SP_ARCH=$(uname -m 2>/dev/null)"
. /etc/os-release 2>/dev/null; echo "SP_OS=${PRETTY_NAME:-$(uname -s)}"
echo "SP_CPU=$(nproc 2>/dev/null || echo 0)"
echo "SP_UPTIME=$(cut -d' ' -f1 /proc/uptime 2>/dev/null)"
echo "SP_LOAD=$(cut -d' ' -f1 /proc/loadavg 2>/dev/null)"
awk '/^MemTotal:/{t=$2} /^MemAvailable:/{a=$2} END{print "SP_MEM=" t " " a}' /proc/meminfo 2>/dev/null
df -kP / 2>/dev/null | awk 'NR==2{print "SP_DISK=" $2 " " $3}'
`

// SystemInfo đọc thông tin máy chủ từ xa.
func (c *Client) SystemInfo(ctx context.Context) (Info, error) {
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	result, err := c.Run(ctx, infoCommand)
	if err != nil {
		return Info{}, err
	}
	return parseInfo(result.Stdout), nil
}

// parseInfo đọc kết quả của infoCommand.
func parseInfo(output string) Info {
	var info Info

	for _, line := range strings.Split(output, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)

		switch key {
		case "SP_HOST":
			info.Hostname = value
		case "SP_KERNEL":
			info.Kernel = value
		case "SP_ARCH":
			info.Arch = value
		case "SP_OS":
			info.OS = strings.Trim(value, `"`)
		case "SP_CPU":
			info.CPUCores, _ = strconv.Atoi(value)
		case "SP_UPTIME":
			seconds, _ := strconv.ParseFloat(value, 64)
			info.UptimeSeconds = int64(seconds)
		case "SP_LOAD":
			info.Load1, _ = strconv.ParseFloat(value, 64)
		case "SP_MEM":
			// /proc/meminfo tính bằng kilobyte, và cái nó gọi là "khả dụng" mới là
			// phần thật sự còn trống — phần "free" không tính bộ đệm đĩa nên luôn
			// làm máy chủ trông như sắp hết bộ nhớ.
			total, available := twoNumbers(value)
			info.MemoryTotal = total * 1024
			info.MemoryUsed = (total - available) * 1024
		case "SP_DISK":
			total, used := twoNumbers(value)
			info.DiskTotal = total * 1024
			info.DiskUsed = used * 1024
		}
	}
	return info
}

// twoNumbers đọc hai số nguyên cách nhau bằng khoảng trắng.
func twoNumbers(value string) (int64, int64) {
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return 0, 0
	}
	first, _ := strconv.ParseInt(fields[0], 10, 64)
	second, _ := strconv.ParseInt(fields[1], 10, 64)
	return first, second
}

// Uptime là thời gian máy đã chạy, dạng time.Duration.
func (i Info) Uptime() time.Duration {
	return time.Duration(i.UptimeSeconds) * time.Second
}
