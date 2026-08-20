// Package dnscheck tra cứu tên miền và so với địa chỉ của chính máy chủ.
//
// Lý do số một khiến việc cấp chứng chỉ Let's Encrypt thất bại là tên miền
// chưa trỏ về máy này. Thông báo lỗi của ACME khi đó nói về "xác thực thất
// bại" chứ không nói ra điều đó, nên người dùng thường mất cả buổi sửa cấu
// hình nginx cho một chuyện nằm ở bảng DNS.
package dnscheck

import (
	"context"
	"errors"
	"net"
	"sort"
	"strings"
	"time"
)

// lookupTimeout là thời gian chờ tối đa cho một lần tra cứu.
//
// Máy chủ DNS không trả lời là chuyện thường gặp hơn người ta tưởng; chờ vô
// hạn nghĩa là trang kiểm tra treo cứng cho tới khi trình duyệt bỏ cuộc.
const lookupTimeout = 5 * time.Second

// Status là kết luận của một lần kiểm tra.
type Status string

const (
	// StatusPointsHere là tên miền trỏ về đúng máy chủ này.
	StatusPointsHere Status = "here"
	// StatusElsewhere là tên miền có bản ghi nhưng trỏ đi nơi khác.
	StatusElsewhere Status = "elsewhere"
	// StatusMissing là tên miền chưa có bản ghi nào.
	StatusMissing Status = "missing"
	// StatusUnknown là không tra cứu được, ví dụ máy chủ DNS không trả lời.
	StatusUnknown Status = "unknown"
)

// Result là kết quả kiểm tra một tên miền.
type Result struct {
	Domain string   `json:"domain"`
	Status Status   `json:"status"`
	IPs    []string `json:"ips"`
	// CNAME là tên miền đích nếu bản ghi là CNAME.
	CNAME string `json:"cname,omitempty"`
	// Error là mô tả ngắn khi không tra cứu được.
	Error string `json:"error,omitempty"`
	// TookMs là thời gian tra cứu, để nhận ra máy chủ DNS đang chậm.
	TookMs int64 `json:"tookMs"`
}

// Report là kết quả kiểm tra toàn bộ tên miền của một website.
type Report struct {
	// ServerIPs là các địa chỉ panel nhìn thấy trên chính máy này.
	ServerIPs []string `json:"serverIps"`
	Results   []Result `json:"results"`
}

// Resolver là phần tra cứu DNS, tách ra để bài kiểm thử thay được.
type Resolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
	LookupCNAME(ctx context.Context, host string) (string, error)
}

// Checker tra cứu tên miền.
type Checker struct {
	resolver Resolver
	// localIPs trả về địa chỉ của máy chủ; tách ra để test không phụ thuộc máy chạy.
	localIPs func() []string
}

// New tạo bộ kiểm tra dùng bộ tra cứu của hệ điều hành.
func New() *Checker {
	return &Checker{resolver: net.DefaultResolver, localIPs: LocalAddresses}
}

// NewWith tạo bộ kiểm tra với bộ tra cứu và danh sách địa chỉ tự chọn.
func NewWith(resolver Resolver, localIPs func() []string) *Checker {
	return &Checker{resolver: resolver, localIPs: localIPs}
}

// Check tra cứu lần lượt các tên miền.
//
// Tra cứu tuần tự chứ không song song: danh sách tên miền của một website đếm
// trên đầu ngón tay, còn bắn cùng lúc nhiều truy vấn tới máy chủ DNS của nhà
// cung cấp là cách nhanh nhất để bị giới hạn tần suất.
func (c *Checker) Check(ctx context.Context, domains []string) Report {
	server := c.localIPs()
	report := Report{ServerIPs: server, Results: make([]Result, 0, len(domains))}

	local := make(map[string]bool, len(server))
	for _, ip := range server {
		local[ip] = true
	}

	for _, domain := range domains {
		report.Results = append(report.Results, c.check(ctx, domain, local))
	}
	return report
}

func (c *Checker) check(ctx context.Context, domain string, local map[string]bool) Result {
	domain = strings.TrimSpace(strings.TrimSuffix(domain, "."))
	result := Result{Domain: domain, Status: StatusUnknown, IPs: []string{}}

	// Tên miền chứa dấu sao là bản ghi đại diện; nó không tra cứu được bằng một
	// truy vấn thường, và báo "không có bản ghi" ở đây là báo sai.
	if strings.HasPrefix(domain, "*.") {
		result.Error = "tên miền đại diện không tra cứu trực tiếp được"
		return result
	}

	ctx, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()

	started := time.Now()
	addrs, err := c.resolver.LookupHost(ctx, domain)
	result.TookMs = time.Since(started).Milliseconds()

	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && dnsErr.IsNotFound {
			result.Status = StatusMissing
			return result
		}
		result.Error = describeError(err)
		return result
	}

	sort.Strings(addrs)
	result.IPs = addrs
	if cname, err := c.resolver.LookupCNAME(ctx, domain); err == nil {
		if trimmed := strings.TrimSuffix(cname, "."); trimmed != domain {
			result.CNAME = trimmed
		}
	}

	result.Status = StatusElsewhere
	for _, addr := range addrs {
		if local[addr] {
			result.Status = StatusPointsHere
			break
		}
	}
	return result
}

// describeError rút gọn lỗi mạng dài dòng của Go thành một câu đọc được.
func describeError(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "máy chủ DNS không trả lời kịp"
	default:
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) {
			if dnsErr.IsTimeout {
				return "máy chủ DNS không trả lời kịp"
			}
			return dnsErr.Err
		}
		return err.Error()
	}
}

// LocalAddresses liệt kê địa chỉ IP của các card mạng trên máy.
//
// Bỏ địa chỉ nội bộ và địa chỉ liên kết cục bộ: chúng không bao giờ là thứ một
// bản ghi DNS trỏ tới, và giữ lại chỉ làm mọi tên miền trỏ về 127.0.0.1 trong
// lúc thử nghiệm được báo là "đã trỏ đúng".
func LocalAddresses() []string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return []string{}
	}

	out := make([]string, 0, 4)
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			network, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := network.IP
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				continue
			}
			out = append(out, ip.String())
		}
	}
	sort.Strings(out)
	return out
}
