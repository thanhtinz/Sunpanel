package sshx

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// testServer là một máy chủ SSH tí hon dựng ngay trong bài kiểm thử.
//
// Có nó thì phần khách hàng được thử với giao thức thật — bắt tay, xác thực,
// mở phiên, mã thoát — thay vì chỉ thử vài hàm phân tích chuỗi.
type testServer struct {
	address     string
	fingerprint string
	// password và username là thông tin đăng nhập được chấp nhận.
	username string
	password string
}

// newTestServer dựng máy chủ và dừng nó khi bài kiểm thử kết thúc.
func newTestServer(t *testing.T) *testServer {
	t.Helper()

	hostKey, _ := generateKey(t)
	signer, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatalf("dựng khóa máy chủ: %v", err)
	}

	server := &testServer{
		fingerprint: ssh.FingerprintSHA256(signer.PublicKey()),
		username:    "quantri",
		password:    "MatKhau@2026",
	}

	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if conn.User() == server.username && string(password) == server.password {
				return nil, nil
			}
			return nil, errors.New("sai mật khẩu")
		},
		PublicKeyCallback: func(conn ssh.ConnMetadata, _ ssh.PublicKey) (*ssh.Permissions, error) {
			if conn.User() == server.username {
				return nil, nil
			}
			return nil, errors.New("không nhận khóa này")
		},
	}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("mở cổng: %v", err)
	}
	server.address = listener.Addr().String()
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serve(conn, config)
		}
	}()
	return server
}

// serve xử lý một kết nối: chỉ hiểu đúng những gì bài kiểm thử cần.
func serve(conn net.Conn, config *ssh.ServerConfig) {
	defer func() { _ = conn.Close() }()

	sshConn, channels, requests, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer func() { _ = sshConn.Close() }()
	go ssh.DiscardRequests(requests)

	for newChannel := range channels {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "chỉ hỗ trợ session")
			continue
		}
		channel, sessionRequests, err := newChannel.Accept()
		if err != nil {
			return
		}
		go handleSession(channel, sessionRequests)
	}
}

func handleSession(channel ssh.Channel, requests <-chan *ssh.Request) {
	defer func() { _ = channel.Close() }()

	for req := range requests {
		switch req.Type {
		case "exec":
			// Thân yêu cầu exec là chuỗi lệnh có tiền tố độ dài bốn byte.
			command := string(req.Payload[4:])
			_ = req.Reply(true, nil)

			status := uint32(0)
			switch {
			case strings.Contains(command, "SP_HOST"):
				_, _ = io.WriteString(channel, "SP_HOST=vps-thu-nghiem\nSP_KERNEL=6.1.0\n"+
					"SP_ARCH=x86_64\nSP_OS=Debian GNU/Linux 12 (bookworm)\nSP_CPU=4\n"+
					"SP_UPTIME=98765.43\nSP_LOAD=0.42\nSP_MEM=4096000 1024000\nSP_DISK=52428800 20971520\n")
			case strings.Contains(command, "hong"):
				_, _ = io.WriteString(channel.Stderr(), "khong tim thay lenh\n")
				status = 127
			default:
				_, _ = io.WriteString(channel, "xin chao\n")
			}

			_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
			return
		case "pty-req", "window-change":
			_ = req.Reply(true, nil)
		case "shell":
			_ = req.Reply(true, nil)
			go func() {
				// Vọng lại những gì nhận được, đủ để bài kiểm thử biết hai chiều
				// của phiên đều thông.
				_, _ = io.Copy(channel, channel)
				_ = channel.Close()
			}()
		default:
			_ = req.Reply(false, nil)
		}
	}
}

// generateKey sinh một cặp khóa ed25519 kèm bản PEM của khóa riêng.
func generateKey(t *testing.T) (ed25519.PrivateKey, string) {
	t.Helper()

	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("sinh khóa: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatalf("mã hóa khóa: %v", err)
	}
	block := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	return private, string(block)
}

// target dựng thông tin kết nối tới máy chủ thử nghiệm.
func (s *testServer) target() Target {
	host, port, _ := net.SplitHostPort(s.address)
	number := 0
	_, _ = fmtSscan(port, &number)
	return Target{
		Host: host, Port: number,
		Credential: Credential{User: s.username, Password: s.password},
	}
}

// fmtSscan tách riêng để phần import không phình vì một lần đọc số.
func fmtSscan(value string, out *int) (int, error) {
	number := 0
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, errors.New("không phải số")
		}
		number = number*10 + int(char-'0')
	}
	*out = number
	return 1, nil
}

func TestDialAndRun(t *testing.T) {
	server := newTestServer(t)

	client, err := Dial(context.Background(), server.target())
	if err != nil {
		t.Fatalf("kết nối: %v", err)
	}
	defer func() { _ = client.Close() }()

	if client.Fingerprint() != server.fingerprint {
		t.Errorf("dấu vân tay = %q, mong %q", client.Fingerprint(), server.fingerprint)
	}

	result, err := client.Run(context.Background(), "echo xin chao")
	if err != nil {
		t.Fatalf("chạy lệnh: %v", err)
	}
	if strings.TrimSpace(result.Stdout) != "xin chao" || result.ExitCode != 0 {
		t.Errorf("kết quả = %+v", result)
	}
}

// Lệnh trả mã khác 0 không phải lỗi kết nối: bên gọi phải đọc được cả mã lẫn
// phần đã in ra, nếu không thì mọi lệnh thất bại đều trông như mất mạng.
func TestRunKeepsExitCode(t *testing.T) {
	server := newTestServer(t)

	client, err := Dial(context.Background(), server.target())
	if err != nil {
		t.Fatalf("kết nối: %v", err)
	}
	defer func() { _ = client.Close() }()

	result, err := client.Run(context.Background(), "lenh-hong")
	if err != nil {
		t.Fatalf("chạy lệnh: %v", err)
	}
	if result.ExitCode != 127 || !strings.Contains(result.Stderr, "khong tim thay") {
		t.Errorf("kết quả = %+v", result)
	}
}

func TestSystemInfo(t *testing.T) {
	server := newTestServer(t)

	client, err := Dial(context.Background(), server.target())
	if err != nil {
		t.Fatalf("kết nối: %v", err)
	}
	defer func() { _ = client.Close() }()

	info, err := client.SystemInfo(context.Background())
	if err != nil {
		t.Fatalf("đọc thông tin: %v", err)
	}

	if info.Hostname != "vps-thu-nghiem" || info.Arch != "x86_64" || info.CPUCores != 4 {
		t.Errorf("thông tin = %+v", info)
	}
	if info.OS != "Debian GNU/Linux 12 (bookworm)" {
		t.Errorf("hệ điều hành = %q", info.OS)
	}
	// /proc/meminfo tính bằng kilobyte và phần đã dùng là tổng trừ khả dụng.
	if info.MemoryTotal != 4096000*1024 || info.MemoryUsed != (4096000-1024000)*1024 {
		t.Errorf("bộ nhớ = %d/%d", info.MemoryUsed, info.MemoryTotal)
	}
	if info.Uptime() != 98765*time.Second {
		t.Errorf("thời gian chạy = %v", info.Uptime())
	}
}

func TestAuthFailure(t *testing.T) {
	server := newTestServer(t)

	target := server.target()
	target.Password = "sai-mat-khau"

	if _, err := Dial(context.Background(), target); !errors.Is(err, ErrAuthFailed) {
		t.Errorf("lỗi = %v, mong ErrAuthFailed", err)
	}
}

// Khóa máy chủ đổi nghĩa là hoặc máy đã bị cài lại, hoặc có người đứng giữa
// đang giả làm nó. Cả hai đều phải dừng lại chứ không im lặng kết nối tiếp.
func TestHostKeyChangeIsRefused(t *testing.T) {
	server := newTestServer(t)

	target := server.target()
	target.Fingerprint = "SHA256:khoa-cua-mot-may-khac"

	_, err := Dial(context.Background(), target)
	if !errors.Is(err, ErrHostKeyChanged) {
		t.Errorf("lỗi = %v, mong ErrHostKeyChanged", err)
	}
}

func TestKnownFingerprintIsAccepted(t *testing.T) {
	server := newTestServer(t)

	target := server.target()
	target.Fingerprint = server.fingerprint

	client, err := Dial(context.Background(), target)
	if err != nil {
		t.Fatalf("kết nối với khóa đã biết: %v", err)
	}
	_ = client.Close()
}

func TestPrivateKeyAuth(t *testing.T) {
	server := newTestServer(t)
	_, pemKey := generateKey(t)

	target := server.target()
	target.Password = ""
	target.PrivateKey = pemKey

	client, err := Dial(context.Background(), target)
	if err != nil {
		t.Fatalf("đăng nhập bằng khóa: %v", err)
	}
	_ = client.Close()
}

func TestBadPrivateKeyIsReported(t *testing.T) {
	server := newTestServer(t)

	target := server.target()
	target.Password = ""
	target.PrivateKey = "không phải khóa"

	if _, err := Dial(context.Background(), target); err == nil {
		t.Error("khóa hỏng lại được chấp nhận")
	}
}

func TestOpenShellEchoes(t *testing.T) {
	server := newTestServer(t)

	client, err := Dial(context.Background(), server.target())
	if err != nil {
		t.Fatalf("kết nối: %v", err)
	}
	defer func() { _ = client.Close() }()

	shell, err := client.OpenShell(120, 40)
	if err != nil {
		t.Fatalf("mở phiên: %v", err)
	}
	defer func() { _ = shell.Close() }()

	if _, err := shell.Write([]byte("ls\n")); err != nil {
		t.Fatalf("gửi phím: %v", err)
	}
	if err := shell.Resize(100, 30); err != nil {
		t.Errorf("đổi kích thước: %v", err)
	}

	// Đọc trong luồng riêng kèm mốc chờ: máy chủ thử nghiệm vọng lại ngay, nên
	// im lặng quá vài giây nghĩa là phiên hỏng chứ không phải chậm.
	echoed := make(chan string, 1)
	go func() {
		buffer := make([]byte, 16)
		n, err := shell.Read(buffer)
		if err != nil {
			echoed <- ""
			return
		}
		echoed <- string(buffer[:n])
	}()

	select {
	case value := <-echoed:
		if !strings.Contains(value, "ls") {
			t.Errorf("phản hồi = %q", value)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("phiên không vọng lại gì trong năm giây")
	}
}

func TestUnreachableHost(t *testing.T) {
	target := Target{
		Host: "127.0.0.1", Port: 1,
		Credential: Credential{User: "ai-do", Password: "gi-do"},
	}

	if _, err := Dial(context.Background(), target); !errors.Is(err, ErrUnreachable) {
		t.Errorf("lỗi = %v, mong ErrUnreachable", err)
	}
}

// Mức dùng CPU là hiệu của hai lần đọc /proc/stat: đọc một lần chỉ cho ra mức
// trung bình kể từ ngày bật máy, và con số đó gần như không bao giờ đổi.
func TestParseMetricsUsesDelta(t *testing.T) {
	output := strings.Join([]string{
		"SP_CPU1=1000 9000",
		"SP_CPU2=1300 9700",
		"SP_MEM=4000000 1000000",
		"SP_DISK=1000000 250000",
		"SP_LOAD=1.25",
	}, "\n")

	metrics := parseMetrics(output)

	// Giữa hai lần đọc: bận thêm 300, rỗi thêm 700 — tức 30% của một nghìn nhịp.
	if metrics.CPUPercent < 29.9 || metrics.CPUPercent > 30.1 {
		t.Errorf("CPU = %.2f%%, mong 30%%", metrics.CPUPercent)
	}
	if metrics.MemoryPercent != 75 {
		t.Errorf("bộ nhớ = %.2f%%, mong 75%%", metrics.MemoryPercent)
	}
	if metrics.DiskPercent != 25 {
		t.Errorf("ổ đĩa = %.2f%%, mong 25%%", metrics.DiskPercent)
	}
	if metrics.Load1 != 1.25 {
		t.Errorf("tải = %.2f", metrics.Load1)
	}
}

// Số liệu đọc từ máy khác không phải lúc nào cũng nhất quán; một cột âm hay
// vượt 100 vẽ ra biểu đồ vô nghĩa.
func TestParseMetricsClampsRange(t *testing.T) {
	metrics := parseMetrics("SP_CPU1=5000 9000\nSP_CPU2=1000 9500\nSP_MEM=100 500\n")

	if metrics.CPUPercent != 0 {
		t.Errorf("CPU khi bộ đếm đặt lại = %.2f, mong 0", metrics.CPUPercent)
	}
	if metrics.MemoryPercent != 0 {
		t.Errorf("bộ nhớ khi khả dụng lớn hơn tổng = %.2f, mong 0", metrics.MemoryPercent)
	}
}

// Thiếu dữ liệu không được làm cả lần lấy mẫu hỏng: một bản Linux gọn nhẹ có
// thể không có df hoặc không có /proc/loadavg.
func TestParseMetricsToleratesMissingFields(t *testing.T) {
	metrics := parseMetrics("SP_MEM=1000 400\n")

	if metrics.MemoryPercent != 60 {
		t.Errorf("bộ nhớ = %.2f%%, mong 60%%", metrics.MemoryPercent)
	}
	if metrics.CPUPercent != 0 || metrics.DiskPercent != 0 {
		t.Errorf("trường thiếu phải là 0: %+v", metrics)
	}
}
