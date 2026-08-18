package certs

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/acme"
)

// Các máy chủ ACME của Let's Encrypt.
const (
	// LetsEncryptProduction cấp chứng chỉ thật, nhưng giới hạn số lần thử rất chặt.
	LetsEncryptProduction = "https://acme-v02.api.letsencrypt.org/directory"
	// LetsEncryptStaging dùng để thử nghiệm: chứng chỉ không được trình duyệt tin
	// nhưng giới hạn thoáng hơn nhiều.
	LetsEncryptStaging = "https://acme-staging-v02.api.letsencrypt.org/directory"
)

// orderTimeout là thời gian tối đa cho trọn một lần xin chứng chỉ.
const orderTimeout = 5 * time.Minute

// ChallengeSolver ghi và xóa tệp trả lời thử thách HTTP-01.
//
// Tách thành interface vì nơi đặt tệp phụ thuộc cấu hình website: có website
// phục vụ từ thư mục riêng, có website là reverse proxy nên panel phải tự phục vụ.
type ChallengeSolver interface {
	// Present đặt nội dung trả lời tại đường dẫn /.well-known/acme-challenge/{token}.
	Present(ctx context.Context, token, keyAuth string) error
	// CleanUp xóa nội dung đã đặt.
	CleanUp(ctx context.Context, token string) error
}

// ACMEClient xin chứng chỉ từ một máy chủ ACME.
type ACMEClient struct {
	directoryURL string
	email        string
	// accountKeyPath là nơi lưu khóa tài khoản ACME.
	accountKeyPath string
}

// NewACMEClient tạo client ACME.
//
// staging quyết định dùng máy chủ thử nghiệm hay máy chủ thật. Nên thử ở máy chủ
// thử nghiệm trước: Let's Encrypt giới hạn 5 lần thất bại mỗi tài khoản mỗi giờ,
// và chạm giới hạn nghĩa là phải chờ.
func NewACMEClient(email, accountKeyPath string, staging bool) *ACMEClient {
	directory := LetsEncryptProduction
	if staging {
		directory = LetsEncryptStaging
	}
	return &ACMEClient{directoryURL: directory, email: email, accountKeyPath: accountKeyPath}
}

// Obtain xin một chứng chỉ mới cho các tên miền cho trước.
func (c *ACMEClient) Obtain(
	ctx context.Context, domains []string, solver ChallengeSolver,
) (certPEM, keyPEM []byte, err error) {
	if len(domains) == 0 {
		return nil, nil, fmt.Errorf("%w: cần ít nhất một tên miền", ErrInvalidCertificate)
	}
	for _, domain := range domains {
		if err := validateACMEDomain(domain); err != nil {
			return nil, nil, err
		}
	}

	ctx, cancel := context.WithTimeout(ctx, orderTimeout)
	defer cancel()

	accountKey, err := c.loadOrCreateAccountKey()
	if err != nil {
		return nil, nil, err
	}

	client := &acme.Client{Key: accountKey, DirectoryURL: c.directoryURL}

	// Đăng ký tài khoản. Tài khoản đã tồn tại không phải lỗi — ACME trả về chính
	// tài khoản cũ, và đó là điều ta muốn ở mọi lần chạy sau lần đầu.
	account := &acme.Account{Contact: []string{"mailto:" + c.email}}
	if _, err := client.Register(ctx, account, acme.AcceptTOS); err != nil {
		if !isAccountExists(err) {
			return nil, nil, fmt.Errorf("đăng ký tài khoản ACME: %w", err)
		}
	}

	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(domains...))
	if err != nil {
		return nil, nil, fmt.Errorf("tạo đơn xin chứng chỉ: %w", err)
	}

	if err := c.solveAuthorizations(ctx, client, order, solver); err != nil {
		return nil, nil, err
	}

	order, err = client.WaitOrder(ctx, order.URI)
	if err != nil {
		return nil, nil, fmt.Errorf("chờ đơn được duyệt: %w", err)
	}

	return c.finalize(ctx, client, order, domains)
}

// solveAuthorizations giải mọi thử thách của đơn.
func (c *ACMEClient) solveAuthorizations(
	ctx context.Context, client *acme.Client, order *acme.Order, solver ChallengeSolver,
) error {
	for _, authzURL := range order.AuthzURLs {
		authz, err := client.GetAuthorization(ctx, authzURL)
		if err != nil {
			return fmt.Errorf("đọc yêu cầu xác thực: %w", err)
		}
		// Tên miền đã được xác thực gần đây thì bỏ qua, không cần làm lại.
		if authz.Status == acme.StatusValid {
			continue
		}

		challenge := findHTTPChallenge(authz)
		if challenge == nil {
			return fmt.Errorf("%w: máy chủ ACME không cung cấp thử thách HTTP-01 cho %s",
				ErrInvalidCertificate, authz.Identifier.Value)
		}

		keyAuth, err := client.HTTP01ChallengeResponse(challenge.Token)
		if err != nil {
			return fmt.Errorf("dựng nội dung trả lời thử thách: %w", err)
		}

		if err := solver.Present(ctx, challenge.Token, keyAuth); err != nil {
			return fmt.Errorf("đặt tệp trả lời thử thách: %w", err)
		}

		err = c.completeChallenge(ctx, client, challenge, authz)
		// Luôn dọn tệp trả lời, kể cả khi thất bại: để lại là để hở một tệp lạ
		// trong thư mục web của người dùng.
		if cleanupErr := solver.CleanUp(ctx, challenge.Token); cleanupErr != nil && err == nil {
			err = fmt.Errorf("dọn tệp trả lời thử thách: %w", cleanupErr)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *ACMEClient) completeChallenge(
	ctx context.Context, client *acme.Client, challenge *acme.Challenge, authz *acme.Authorization,
) error {
	if _, err := client.Accept(ctx, challenge); err != nil {
		return fmt.Errorf("báo sẵn sàng cho thử thách: %w", err)
	}
	if _, err := client.WaitAuthorization(ctx, authz.URI); err != nil {
		return fmt.Errorf("xác thực tên miền %s thất bại: %w", authz.Identifier.Value, err)
	}
	return nil
}

// finalize sinh khóa, gửi CSR và nhận chứng chỉ.
func (c *ACMEClient) finalize(
	ctx context.Context, client *acme.Client, order *acme.Order, domains []string,
) (certPEM, keyPEM []byte, err error) {
	// Khóa của chứng chỉ luôn sinh mới ở mỗi lần xin: dùng lại khóa cũ nghĩa là
	// một khóa bị lộ vẫn dùng được với chứng chỉ mới.
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("sinh khóa chứng chỉ: %w", err)
	}

	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: domains[0]},
		DNSNames: domains,
	}, certKey)
	if err != nil {
		return nil, nil, fmt.Errorf("tạo yêu cầu ký chứng chỉ: %w", err)
	}

	chain, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil {
		return nil, nil, fmt.Errorf("nhận chứng chỉ: %w", err)
	}

	// Ghép cả chuỗi thành fullchain: thiếu chứng chỉ trung gian thì nhiều thiết
	// bị di động sẽ báo chứng chỉ không tin cậy.
	var certBuf []byte
	for _, der := range chain {
		certBuf = append(certBuf, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	}

	keyDER, err := x509.MarshalECPrivateKey(certKey)
	if err != nil {
		return nil, nil, fmt.Errorf("mã hóa khóa chứng chỉ: %w", err)
	}

	return certBuf, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), nil
}

// loadOrCreateAccountKey đọc khóa tài khoản ACME, tạo mới nếu chưa có.
//
// Khóa tài khoản phải được giữ nguyên giữa các lần chạy: mất nó nghĩa là mất
// luôn lịch sử tài khoản và các giới hạn tần suất đã tích lũy.
func (c *ACMEClient) loadOrCreateAccountKey() (*ecdsa.PrivateKey, error) {
	content, err := os.ReadFile(c.accountKeyPath)
	if err == nil {
		block, _ := pem.Decode(content)
		if block == nil {
			return nil, fmt.Errorf("%w: khóa tài khoản ACME hỏng", ErrInvalidCertificate)
		}
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("đọc khóa tài khoản ACME: %w", err)
		}
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("đọc khóa tài khoản ACME: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("sinh khóa tài khoản ACME: %w", err)
	}

	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("mã hóa khóa tài khoản ACME: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(c.accountKeyPath), 0o700); err != nil {
		return nil, fmt.Errorf("tạo thư mục khóa tài khoản: %w", err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	if err := os.WriteFile(c.accountKeyPath, encoded, 0o600); err != nil {
		return nil, fmt.Errorf("ghi khóa tài khoản ACME: %w", err)
	}

	return key, nil
}

// findHTTPChallenge tìm thử thách HTTP-01 trong một yêu cầu xác thực.
func findHTTPChallenge(authz *acme.Authorization) *acme.Challenge {
	for _, challenge := range authz.Challenges {
		if challenge.Type == "http-01" {
			return challenge
		}
	}
	return nil
}

// isAccountExists cho biết lỗi có phải do tài khoản đã tồn tại hay không.
func isAccountExists(err error) bool {
	var acmeErr *acme.Error
	if !errorsAs(err, &acmeErr) {
		return false
	}
	return acmeErr.ProblemType == "urn:ietf:params:acme:error:accountDoesNotExist" ||
		acmeErr.StatusCode == 200 || acmeErr.StatusCode == 409
}
