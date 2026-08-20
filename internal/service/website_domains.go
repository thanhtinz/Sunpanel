package service

import (
	"context"

	"github.com/thanhtinz/sunpanel/pkg/dnscheck"
)

// CheckDomains tra cứu các tên miền của một website và so với địa chỉ máy chủ.
//
// Kiểm tra này trả lời trước câu hỏi mà ACME chỉ trả lời sau khi đã thất bại:
// tên miền có thật sự trỏ về máy này chưa. Không có nó, người dùng đọc dòng
// "xác thực thất bại" rồi đi sửa cấu hình nginx cho một chuyện nằm ở bảng DNS.
func (s *WebsiteService) CheckDomains(ctx context.Context, id uint) (dnscheck.Report, error) {
	site, err := s.Get(ctx, id)
	if err != nil {
		return dnscheck.Report{}, err
	}
	return s.dns.Check(ctx, splitDomains(site.Domains)), nil
}
