package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// smtpTimeout giới hạn thời gian chờ máy chủ SMTP.
const smtpTimeout = 30 * time.Second

// emailChannel gửi thông báo qua SMTP.
type emailChannel struct {
	host     string
	port     int
	username string
	password string
	from     string
	to       []string
	// useTLS bật TLS ngay từ đầu kết nối (cổng 465); ngược lại dùng STARTTLS.
	useTLS bool
	// skipVerify bỏ qua việc kiểm chứng chỉ máy chủ SMTP.
	skipVerify bool
}

func newEmail(config map[string]string) (Channel, error) {
	host, err := require(config, "host")
	if err != nil {
		return nil, err
	}
	from, err := require(config, "from")
	if err != nil {
		return nil, err
	}
	to, err := require(config, "to")
	if err != nil {
		return nil, err
	}

	port, _ := strconv.Atoi(config["port"])
	if port <= 0 || port > 65535 {
		port = 587
	}

	recipients := make([]string, 0, 4)
	for _, address := range strings.Split(to, ",") {
		if trimmed := strings.TrimSpace(address); trimmed != "" {
			recipients = append(recipients, trimmed)
		}
	}
	if len(recipients) == 0 {
		return nil, fmt.Errorf("%w: không có người nhận", ErrInvalidConfig)
	}

	return &emailChannel{
		host: host, port: port,
		username: strings.TrimSpace(config["username"]),
		password: config["password"],
		from:     from, to: recipients,
		// Cổng 465 dùng TLS ngay từ đầu, các cổng khác dùng STARTTLS.
		useTLS:     config["tls"] == "true" || port == 465,
		skipVerify: config["skipVerify"] == "true",
	}, nil
}

// Kind trả về loại kênh.
func (c *emailChannel) Kind() Kind { return Email }

// Send gửi thông báo qua SMTP.
func (c *emailChannel) Send(ctx context.Context, message Message) error {
	address := net.JoinHostPort(c.host, strconv.Itoa(c.port))
	dialer := &net.Dialer{Timeout: smtpTimeout}

	var conn net.Conn
	var err error
	if c.useTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", address, c.tlsConfig())
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("%w: kết nối SMTP: %s", ErrSendFailed, err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, c.host)
	if err != nil {
		return fmt.Errorf("%w: bắt tay SMTP: %s", ErrSendFailed, err)
	}
	defer client.Quit()

	// STARTTLS nâng cấp kết nối đang mở lên TLS. Không có nó thì mật khẩu SMTP
	// đi qua mạng ở dạng rõ.
	if !c.useTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(c.tlsConfig()); err != nil {
				return fmt.Errorf("%w: bật STARTTLS: %s", ErrSendFailed, err)
			}
		}
	}

	if c.username != "" {
		auth := smtp.PlainAuth("", c.username, c.password, c.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("%w: đăng nhập SMTP: %s", ErrSendFailed, err)
		}
	}

	if err := client.Mail(c.from); err != nil {
		return fmt.Errorf("%w: đặt người gửi: %s", ErrSendFailed, err)
	}
	for _, recipient := range c.to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("%w: đặt người nhận %s: %s", ErrSendFailed, recipient, err)
		}
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("%w: mở luồng dữ liệu: %s", ErrSendFailed, err)
	}
	if _, err := writer.Write([]byte(c.compose(message))); err != nil {
		writer.Close()
		return fmt.Errorf("%w: ghi nội dung: %s", ErrSendFailed, err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("%w: kết thúc nội dung: %s", ErrSendFailed, err)
	}
	return nil
}

func (c *emailChannel) tlsConfig() *tls.Config {
	return &tls.Config{ServerName: c.host, InsecureSkipVerify: c.skipVerify}
}

// compose dựng nội dung thư.
func (c *emailChannel) compose(message Message) string {
	subject := fmt.Sprintf("%s [SunPanel] %s", message.Level.Icon(), message.Title)

	var builder strings.Builder
	builder.WriteString("From: " + c.from + "\r\n")
	builder.WriteString("To: " + strings.Join(c.to, ", ") + "\r\n")
	// Tiêu đề tiếng Việt phải được mã hóa theo RFC 2047, nếu không nhiều máy chủ
	// thư sẽ cắt mất dấu hoặc hiện ra một dãy ký tự vô nghĩa.
	builder.WriteString("Subject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\n")
	builder.WriteString("MIME-Version: 1.0\r\n")
	builder.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	builder.WriteString("Content-Transfer-Encoding: 8bit\r\n")
	builder.WriteString("\r\n")
	builder.WriteString(strings.ReplaceAll(message.PlainText(), "\n", "\r\n"))
	return builder.String()
}
