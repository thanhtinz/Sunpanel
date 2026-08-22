// Package certs quản lý chứng chỉ TLS của các website.
//
// Panel dùng golang.org/x/crypto/acme thay vì một thư viện ACME riêng: gói này
// đã nằm sẵn trong phụ thuộc của dự án, do chính đội ngũ Go bảo trì, và không
// kéo thêm thứ gì vào binary.
package certs

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Các lỗi dùng chung.
var (
	// ErrNotFound là không tìm thấy chứng chỉ.
	ErrNotFound = errors.New("certs: không tìm thấy chứng chỉ")
	// ErrInvalidName là tên chứng chỉ không hợp lệ.
	ErrInvalidName = errors.New("certs: tên chứng chỉ không hợp lệ")
	// ErrInvalidCertificate là dữ liệu chứng chỉ không đọc được.
	ErrInvalidCertificate = errors.New("certs: dữ liệu chứng chỉ không hợp lệ")
)

// validName khớp tên chứng chỉ, vốn được dùng làm tên thư mục.
var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

// Info là thông tin tóm tắt của một chứng chỉ.
type Info struct {
	// Name là định danh trong kho.
	Name string `json:"name"`
	// Domains là các tên miền chứng chỉ bảo vệ.
	Domains []string `json:"domains"`
	// IPs là các địa chỉ IP chứng chỉ bảo vệ.
	//
	// Tách riêng khỏi Domains vì đây là hai trường khác nhau trong chứng chỉ:
	// gộp chung sẽ khiến việc đối chiếu "chứng chỉ này có phủ tên miền kia không"
	// cho kết quả sai.
	IPs []string `json:"ips"`
	// Issuer là tổ chức cấp chứng chỉ.
	Issuer string `json:"issuer"`
	// NotBefore và NotAfter là khoảng thời gian chứng chỉ có hiệu lực.
	NotBefore time.Time `json:"notBefore"`
	NotAfter  time.Time `json:"notAfter"`
	// SelfSigned cho biết chứng chỉ do chính panel tự ký.
	SelfSigned bool `json:"selfSigned"`
	// CertFile và KeyFile là đường dẫn tệp trên đĩa.
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

// DaysRemaining trả về số ngày còn lại trước khi chứng chỉ hết hạn.
//
// Giá trị âm nghĩa là đã hết hạn.
func (i Info) DaysRemaining() int {
	return int(time.Until(i.NotAfter).Hours() / 24)
}

// NeedsRenewal cho biết đã tới lúc gia hạn chưa.
//
// Ngưỡng 30 ngày theo khuyến nghị của Let's Encrypt: đủ sớm để còn nhiều lần thử
// lại nếu việc gia hạn thất bại vì DNS hoặc mạng.
func (i Info) NeedsRenewal() bool {
	return i.DaysRemaining() < 30
}

// Store lưu chứng chỉ trên đĩa.
//
// Mỗi chứng chỉ nằm trong một thư mục riêng gồm fullchain.pem và privkey.pem,
// theo đúng cách Let's Encrypt sắp xếp, để người quen với certbot không phải học lại.
type Store struct {
	root string
}

// NewStore tạo kho chứng chỉ tại thư mục root.
func NewStore(root string) *Store { return &Store{root: root} }

// Dir trả về thư mục chứa một chứng chỉ.
func (s *Store) Dir(name string) (string, error) {
	if !validName.MatchString(name) {
		return "", fmt.Errorf("%w: %q", ErrInvalidName, name)
	}
	return filepath.Join(s.root, name), nil
}

// Paths trả về đường dẫn tệp chứng chỉ và khóa.
func (s *Store) Paths(name string) (certFile, keyFile string, err error) {
	dir, err := s.Dir(name)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, "fullchain.pem"), filepath.Join(dir, "privkey.pem"), nil
}

// Save ghi chứng chỉ và khóa xuống đĩa.
func (s *Store) Save(name string, certPEM, keyPEM []byte) error {
	dir, err := s.Dir(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("tạo thư mục chứng chỉ: %w", err)
	}

	certFile, keyFile, err := s.Paths(name)
	if err != nil {
		return err
	}

	// Chứng chỉ công khai nên máy chủ web đọc được là đủ; khóa riêng thì chỉ chủ
	// sở hữu — lộ khóa là mất trắng lớp bảo vệ TLS.
	if err := os.WriteFile(certFile, certPEM, 0o644); err != nil {
		return fmt.Errorf("ghi chứng chỉ: %w", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		return fmt.Errorf("ghi khóa riêng: %w", err)
	}
	return nil
}

// Load đọc thông tin một chứng chỉ đã lưu.
func (s *Store) Load(name string) (Info, error) {
	certFile, keyFile, err := s.Paths(name)
	if err != nil {
		return Info{}, err
	}

	content, err := os.ReadFile(certFile)
	if err != nil {
		if os.IsNotExist(err) {
			return Info{}, fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		return Info{}, fmt.Errorf("đọc chứng chỉ: %w", err)
	}

	info, err := Parse(content)
	if err != nil {
		return Info{}, err
	}
	info.Name = name
	info.CertFile = certFile
	info.KeyFile = keyFile
	return info, nil
}

// List liệt kê mọi chứng chỉ trong kho.
func (s *Store) List() ([]Info, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("đọc thư mục chứng chỉ: %w", err)
	}

	var out []Info
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Thư mục hỏng không nên làm hỏng cả danh sách; bỏ qua và đi tiếp.
		info, err := s.Load(entry.Name())
		if err != nil {
			continue
		}
		out = append(out, info)
	}
	return out, nil
}

// Delete xóa một chứng chỉ khỏi kho.
func (s *Store) Delete(name string) error {
	dir, err := s.Dir(name)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("xóa chứng chỉ: %w", err)
	}
	return nil
}

// Parse đọc thông tin từ dữ liệu PEM của một chứng chỉ.
func Parse(certPEM []byte) (Info, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return Info{}, fmt.Errorf("%w: không đọc được khối PEM", ErrInvalidCertificate)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return Info{}, fmt.Errorf("%w: %w", ErrInvalidCertificate, err)
	}

	ips := make([]string, 0, len(cert.IPAddresses))
	for _, ip := range cert.IPAddresses {
		ips = append(ips, ip.String())
	}

	domains := cert.DNSNames
	// Chứng chỉ cũ có thể chỉ ghi tên miền ở Common Name. Chỉ suy ra từ đó khi
	// chứng chỉ hoàn toàn không có SAN — nếu đã có SAN dạng IP thì Common Name
	// chính là địa chỉ IP đó, và chép nó sang Domains sẽ tạo ra một tên miền không
	// hề tồn tại.
	if len(domains) == 0 && len(ips) == 0 && cert.Subject.CommonName != "" {
		domains = []string{cert.Subject.CommonName}
	}

	return Info{
		Domains:   domains,
		IPs:       ips,
		Issuer:    issuerName(cert),
		NotBefore: cert.NotBefore,
		NotAfter:  cert.NotAfter,
		// Chứng chỉ tự ký có tổ chức cấp trùng với chính chủ thể.
		SelfSigned: cert.Issuer.String() == cert.Subject.String(),
	}, nil
}

func issuerName(cert *x509.Certificate) string {
	if len(cert.Issuer.Organization) > 0 {
		return strings.Join(cert.Issuer.Organization, ", ")
	}
	if cert.Issuer.CommonName != "" {
		return cert.Issuer.CommonName
	}
	return cert.Issuer.String()
}
