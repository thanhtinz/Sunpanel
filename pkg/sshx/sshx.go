// Package sshx kết nối tới một máy chủ khác qua SSH.
//
// Panel đã điều khiển được máy chủ khác qua agent, nhưng cách đó đòi hỏi cài
// thêm phần mềm lên máy đích. SSH thì máy chủ nào cũng có sẵn từ lúc nhà cung
// cấp giao máy, nên đây là con đường duy nhất dùng được ngay với một VPS vừa
// mua về.
package sshx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// Các lỗi của gói.
var (
	// ErrHostKeyChanged là khóa máy chủ khác với lần kết nối trước.
	ErrHostKeyChanged = errors.New("sshx: khóa máy chủ đã thay đổi")
	// ErrAuthFailed là sai tài khoản, mật khẩu hoặc khóa.
	ErrAuthFailed = errors.New("sshx: xác thực thất bại")
	// ErrUnreachable là không mở được kết nối tới máy chủ.
	ErrUnreachable = errors.New("sshx: không kết nối được tới máy chủ")
)

// dialTimeout là thời gian chờ tối đa khi mở kết nối.
//
// Một địa chỉ gõ nhầm hoặc một cổng bị tường lửa chặn sẽ không trả lời gì cả;
// không có mốc dừng thì trang thêm máy chủ treo tới khi trình duyệt bỏ cuộc.
const dialTimeout = 10 * time.Second

// runTimeout là thời gian chờ tối đa của một lệnh đọc thông tin.
const runTimeout = 20 * time.Second

// Credential là cách đăng nhập vào máy chủ.
type Credential struct {
	User string
	// Password dùng khi đăng nhập bằng mật khẩu.
	Password string
	// PrivateKey là nội dung khóa riêng dạng PEM.
	PrivateKey string
	// Passphrase mở khóa riêng nếu khóa có đặt mật khẩu.
	Passphrase string
}

// Target là máy chủ cần kết nối.
type Target struct {
	Host string
	Port int
	Credential
	// Fingerprint là dấu vân tay khóa máy chủ đã ghi nhận ở lần kết nối trước.
	//
	// Để trống nghĩa là lần đầu: khóa nhận được sẽ được chấp nhận và trả về để
	// bên gọi lưu lại. Từ lần sau khóa phải khớp, nếu không thì hoặc máy chủ đã
	// được cài lại, hoặc có người đứng giữa đang giả làm nó.
	Fingerprint string
}

// Address là địa chỉ đầy đủ để mở kết nối.
func (t Target) Address() string {
	port := t.Port
	if port == 0 {
		port = 22
	}
	return net.JoinHostPort(t.Host, strconv.Itoa(port))
}

// Client là một kết nối SSH đang mở.
type Client struct {
	conn        *ssh.Client
	fingerprint string
}

// Dial mở kết nối tới máy chủ.
func Dial(ctx context.Context, target Target) (*Client, error) {
	auth, err := authMethods(target.Credential)
	if err != nil {
		return nil, err
	}

	var seen string
	config := &ssh.ClientConfig{
		User:    target.User,
		Auth:    auth,
		Timeout: dialTimeout,
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			seen = ssh.FingerprintSHA256(key)
			if target.Fingerprint == "" || target.Fingerprint == seen {
				return nil
			}
			return fmt.Errorf("%w: %s", ErrHostKeyChanged, seen)
		},
	}

	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", target.Address())
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrUnreachable, describeDialError(err))
	}

	// Bắt tay SSH cũng phải có mốc dừng riêng: một cổng đang mở nhưng không nói
	// giao thức SSH sẽ giữ kết nối im lặng mãi mãi.
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(dialTimeout))
	}

	sshConn, channels, requests, err := ssh.NewClientConn(conn, target.Address(), config)
	if err != nil {
		_ = conn.Close()
		if errors.Is(err, ErrHostKeyChanged) || strings.Contains(err.Error(), ErrHostKeyChanged.Error()) {
			return nil, fmt.Errorf("%w: %s", ErrHostKeyChanged, seen)
		}
		if strings.Contains(err.Error(), "unable to authenticate") {
			return nil, ErrAuthFailed
		}
		return nil, fmt.Errorf("%w: %s", ErrUnreachable, err.Error())
	}
	_ = conn.SetDeadline(time.Time{})

	return &Client{conn: ssh.NewClient(sshConn, channels, requests), fingerprint: seen}, nil
}

// Fingerprint là dấu vân tay khóa của máy chủ vừa kết nối.
func (c *Client) Fingerprint() string { return c.fingerprint }

// Close đóng kết nối.
func (c *Client) Close() error { return c.conn.Close() }

// Result là kết quả chạy một lệnh.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Run chạy một lệnh và chờ kết quả.
//
// Lệnh chạy qua shell đăng nhập của người dùng trên máy đích, đúng như khi gõ
// tay: những gì panel gửi sang đây đều do quản trị viên tự soạn, cùng mức tin
// cậy với terminal.
func (c *Client) Run(ctx context.Context, command string) (Result, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = session.Close() }()

	var stdout, stderr strings.Builder
	session.Stdout = &stdout
	session.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- session.Run(command) }()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		return Result{}, ctx.Err()
	case err := <-done:
		result := Result{Stdout: stdout.String(), Stderr: stderr.String()}
		var exitErr *ssh.ExitError
		switch {
		case err == nil:
		case errors.As(err, &exitErr):
			// Lệnh chạy xong nhưng trả mã khác 0 không phải lỗi kết nối; bên gọi
			// cần đọc được cả mã lẫn phần đã in ra.
			result.ExitCode = exitErr.ExitStatus()
		default:
			return result, err
		}
		return result, nil
	}
}

// authMethods dựng danh sách cách xác thực từ thông tin đăng nhập.
func authMethods(cred Credential) ([]ssh.AuthMethod, error) {
	methods := make([]ssh.AuthMethod, 0, 2)

	if key := strings.TrimSpace(cred.PrivateKey); key != "" {
		var signer ssh.Signer
		var err error
		if cred.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(key), []byte(cred.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(key))
		}
		if err != nil {
			return nil, fmt.Errorf("sshx: khóa riêng không đọc được: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	if cred.Password != "" {
		methods = append(methods, ssh.Password(cred.Password))
		// Nhiều máy chủ hỏi mật khẩu theo kiểu tương tác thay vì kiểu "password";
		// thiếu nhánh này thì đăng nhập bằng mật khẩu hỏng trên đúng những máy
		// cấu hình chặt nhất.
		methods = append(methods, ssh.KeyboardInteractive(
			func(_, _ string, questions []string, _ []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = cred.Password
				}
				return answers, nil
			},
		))
	}

	if len(methods) == 0 {
		return nil, errors.New("sshx: phải có mật khẩu hoặc khóa riêng")
	}
	return methods, nil
}

// describeDialError rút gọn lỗi mạng dài dòng của Go thành một câu đọc được.
func describeDialError(err error) string {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "máy chủ không trả lời"
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Err != nil {
		return opErr.Err.Error()
	}
	return err.Error()
}
