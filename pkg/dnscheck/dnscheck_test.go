package dnscheck

import (
	"context"
	"net"
	"testing"
)

// fakeResolver trả về câu trả lời dựng sẵn.
type fakeResolver struct {
	hosts  map[string][]string
	cnames map[string]string
	errs   map[string]error
}

func (f fakeResolver) LookupHost(_ context.Context, host string) ([]string, error) {
	if err, ok := f.errs[host]; ok {
		return nil, err
	}
	addrs, ok := f.hosts[host]
	if !ok {
		return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
	}
	return addrs, nil
}

func (f fakeResolver) LookupCNAME(_ context.Context, host string) (string, error) {
	if cname, ok := f.cnames[host]; ok {
		return cname, nil
	}
	return host + ".", nil
}

func newChecker(resolver fakeResolver, local ...string) *Checker {
	return NewWith(resolver, func() []string { return local })
}

func TestCheckClassifiesDomains(t *testing.T) {
	checker := newChecker(fakeResolver{
		hosts: map[string][]string{
			"dung.example.com":  {"203.0.113.10"},
			"sai.example.com":   {"198.51.100.5"},
			"cname.example.com": {"203.0.113.10"},
		},
		cnames: map[string]string{"cname.example.com": "dich.example.net."},
		errs:   map[string]error{"cham.example.com": &net.DNSError{Err: "timeout", IsTimeout: true}},
	}, "203.0.113.10", "2001:db8::1")

	report := checker.Check(context.Background(), []string{
		"dung.example.com", "sai.example.com", "chua-co.example.com",
		"cname.example.com", "cham.example.com",
	})

	if len(report.ServerIPs) != 2 {
		t.Errorf("địa chỉ máy chủ = %v", report.ServerIPs)
	}

	want := []Status{StatusPointsHere, StatusElsewhere, StatusMissing, StatusPointsHere, StatusUnknown}
	for i, result := range report.Results {
		if result.Status != want[i] {
			t.Errorf("%s: trạng thái = %q, mong %q", result.Domain, result.Status, want[i])
		}
	}

	if report.Results[3].CNAME != "dich.example.net" {
		t.Errorf("CNAME = %q", report.Results[3].CNAME)
	}
	// Lỗi mạng của Go dài dòng; giao diện cần một câu ngắn.
	if report.Results[4].Error != "máy chủ DNS không trả lời kịp" {
		t.Errorf("mô tả lỗi = %q", report.Results[4].Error)
	}
}

// Tên miền đại diện không tra cứu được bằng một truy vấn thường; báo "chưa có
// bản ghi" ở đây là báo sai và đẩy người dùng đi sửa một thứ không hỏng.
func TestCheckSkipsWildcard(t *testing.T) {
	checker := newChecker(fakeResolver{}, "203.0.113.10")

	report := checker.Check(context.Background(), []string{"*.example.com"})
	if report.Results[0].Status != StatusUnknown || report.Results[0].Error == "" {
		t.Errorf("kết quả = %+v", report.Results[0])
	}
}

// Dấu chấm cuối là cách viết đầy đủ của một tên miền; nó không được làm phép so
// sánh với bản ghi CNAME lệch đi.
func TestCheckTrimsTrailingDot(t *testing.T) {
	checker := newChecker(fakeResolver{
		hosts: map[string][]string{"example.com": {"203.0.113.10"}},
	}, "203.0.113.10")

	report := checker.Check(context.Background(), []string{"example.com."})
	if report.Results[0].Domain != "example.com" || report.Results[0].Status != StatusPointsHere {
		t.Errorf("kết quả = %+v", report.Results[0])
	}
	if report.Results[0].CNAME != "" {
		t.Errorf("CNAME trỏ về chính nó không được hiện: %q", report.Results[0].CNAME)
	}
}

// Địa chỉ nội bộ không bao giờ là thứ một bản ghi DNS trỏ tới; giữ lại chúng
// thì mọi tên miền trỏ về 127.0.0.1 lúc thử nghiệm đều được báo là đúng.
func TestLocalAddressesSkipLoopback(t *testing.T) {
	for _, addr := range LocalAddresses() {
		ip := net.ParseIP(addr)
		if ip == nil {
			t.Errorf("địa chỉ không hợp lệ: %q", addr)
			continue
		}
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			t.Errorf("địa chỉ nội bộ lọt vào danh sách: %s", addr)
		}
	}
}
