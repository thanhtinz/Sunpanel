package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// selfSignedValidity là thời hạn của chứng chỉ tự ký.
//
// Một năm: đủ dài để không phiền người dùng thử nghiệm, đủ ngắn để không ai quên
// mất rằng đây chỉ là chứng chỉ tạm.
const selfSignedValidity = 365 * 24 * time.Hour

// GenerateSelfSigned tạo một chứng chỉ tự ký cho các tên miền cho trước.
//
// Chứng chỉ tự ký khiến trình duyệt cảnh báo, nhưng vẫn cần thiết: nó cho phép
// bật HTTPS ngay khi website chưa trỏ tên miền về máy chủ, và là chứng chỉ tạm
// để nginx khởi động được trong lúc chờ Let's Encrypt cấp chứng chỉ thật.
func GenerateSelfSigned(domains []string) (certPEM, keyPEM []byte, err error) {
	if len(domains) == 0 {
		return nil, nil, fmt.Errorf("%w: cần ít nhất một tên miền", ErrInvalidCertificate)
	}

	// ECDSA P-256 thay vì RSA: khóa nhỏ hơn nhiều, bắt tay TLS nhanh hơn, và mọi
	// trình duyệt còn được dùng đều hỗ trợ.
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("sinh khóa: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("sinh số sê-ri: %w", err)
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   domains[0],
			Organization: []string{"SunPanel (chứng chỉ tự ký)"},
		},
		// Lùi lại một giờ để chênh lệch đồng hồ giữa máy chủ và máy khách không
		// khiến chứng chỉ bị coi là chưa có hiệu lực.
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(selfSignedValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	for _, domain := range domains {
		if ip := net.ParseIP(domain); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
			continue
		}
		template.DNSNames = append(template.DNSNames, domain)
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, fmt.Errorf("tạo chứng chỉ: %w", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("mã hóa khóa riêng: %w", err)
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}
