package notify

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testMessage() Message {
	return Message{
		Title:    "Đĩa sắp đầy",
		Body:     "Phân vùng / đã dùng 92% dung lượng.",
		Level:    LevelWarning,
		Hostname: "may-chu-1",
		At:       time.Date(2026, 8, 18, 5, 30, 0, 0, time.UTC),
		Fields:   map[string]string{"Ngưỡng": "90%", "Đã dùng": "92%"},
	}
}

func TestNewRejectsIncompleteConfig(t *testing.T) {
	cases := []struct {
		kind   Kind
		config map[string]string
	}{
		{Email, map[string]string{"host": "smtp.vi-du.vn"}},
		{Telegram, map[string]string{"token": "abc"}},
		{Webhook, map[string]string{}},
		{Webhook, map[string]string{"url": "khong-phai-dia-chi"}},
		{"khong-ton-tai", map[string]string{}},
	}
	for _, c := range cases {
		if _, err := New(c.kind, c.config); err == nil {
			t.Errorf("cấu hình thiếu của %s phải bị từ chối", c.kind)
		}
	}
}

// Tiêu đề tiếng Việt phải mã hóa theo RFC 2047, nếu không nhiều máy chủ thư cắt
// mất dấu hoặc hiện ra một dãy ký tự vô nghĩa.
func TestEmailSendsThroughRealSMTPServer(t *testing.T) {
	received := make(chan string, 1)
	address := startFakeSMTP(t, received)

	channel, err := New(Email, map[string]string{
		"host": "127.0.0.1",
		"port": fmt.Sprint(portOf(t, address)),
		"from": "panel@vi-du.vn",
		"to":   "admin@vi-du.vn, phu@vi-du.vn",
	})
	if err != nil {
		t.Fatalf("dựng kênh email: %v", err)
	}

	if err := channel.Send(context.Background(), testMessage()); err != nil {
		t.Fatalf("gửi email: %v", err)
	}

	select {
	case data := <-received:
		if !strings.Contains(data, "RCPT TO:<admin@vi-du.vn>") {
			t.Errorf("thiếu người nhận thứ nhất:\n%s", data)
		}
		if !strings.Contains(data, "RCPT TO:<phu@vi-du.vn>") {
			t.Errorf("danh sách nhiều người nhận chưa được tách:\n%s", data)
		}
		if !strings.Contains(data, "=?UTF-8?q?") {
			t.Errorf("tiêu đề tiếng Việt chưa được mã hóa RFC 2047:\n%s", data)
		}
		if !strings.Contains(data, "Phân vùng / đã dùng 92%") {
			t.Errorf("nội dung tiếng Việt không nguyên vẹn:\n%s", data)
		}
		if !strings.Contains(data, "charset=UTF-8") {
			t.Errorf("thiếu khai báo bộ mã:\n%s", data)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("máy chủ SMTP giả không nhận được thư")
	}
}

// startFakeSMTP dựng một máy chủ SMTP tối giản, đủ để nhận một lá thư.
func startFakeSMTP(t *testing.T, received chan<- string) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("mở cổng SMTP giả: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		var transcript strings.Builder
		reader := bufio.NewReader(conn)
		write := func(line string) { fmt.Fprintf(conn, "%s\r\n", line) }

		write("220 smtp-gia san sang")
		inData := false
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}
			transcript.WriteString(line)
			trimmed := strings.TrimRight(line, "\r\n")

			if inData {
				if trimmed == "." {
					inData = false
					write("250 da nhan")
				}
				continue
			}

			switch {
			case strings.HasPrefix(trimmed, "EHLO"), strings.HasPrefix(trimmed, "HELO"):
				// Không quảng cáo STARTTLS: máy chủ giả không có chứng chỉ, và bài
				// test này kiểm phần dựng thư chứ không kiểm TLS.
				write("250-smtp-gia")
				write("250 SIZE 10240000")
			case strings.HasPrefix(trimmed, "MAIL FROM"), strings.HasPrefix(trimmed, "RCPT TO"):
				write("250 ok")
			case trimmed == "DATA":
				inData = true
				write("354 moi gui noi dung")
			case trimmed == "QUIT":
				write("221 tam biet")
				received <- transcript.String()
				return
			default:
				write("250 ok")
			}
		}
		received <- transcript.String()
	}()

	return listener.Addr().String()
}

func portOf(t *testing.T, address string) int {
	t.Helper()
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatalf("tách cổng: %v", err)
	}
	var number int
	fmt.Sscanf(port, "%d", &number)
	return number
}

// Bỏ sót một ký tự đặc biệt khiến Telegram từ chối cả tin nhắn.
func TestTelegramEscapesMarkdown(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &payload)
		if !strings.HasPrefix(r.URL.Path, "/botTOKEN-THU/sendMessage") {
			t.Errorf("đường dẫn sai: %s", r.URL.Path)
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	channel, err := New(Telegram, map[string]string{
		"token": "TOKEN-THU", "chatId": "-100123", "apiBase": server.URL,
	})
	if err != nil {
		t.Fatalf("dựng kênh Telegram: %v", err)
	}

	message := testMessage()
	message.Title = "Cảnh báo [quan trọng] 92% (đĩa)"
	if err := channel.Send(context.Background(), message); err != nil {
		t.Fatalf("gửi Telegram: %v", err)
	}

	text, _ := payload["text"].(string)
	for _, char := range []string{`\[`, `\]`, `\(`, `\)`, `\.`, `\%`} {
		if char == `\%` {
			continue // % không nằm trong danh sách ký tự đặc biệt của MarkdownV2
		}
		if !strings.Contains(text, char) {
			t.Errorf("ký tự %s chưa được thoát trong: %s", char, text)
		}
	}
	if payload["chat_id"] != "-100123" {
		t.Errorf("chat_id sai: %v", payload["chat_id"])
	}
	if !strings.Contains(text, "Máy chủ") {
		t.Errorf("thiếu tên máy chủ trong tin nhắn: %s", text)
	}
}

// Telegram nói rõ vì sao trong thân phản hồi; nuốt nó đi khiến người dùng không
// biết mình sai token hay bot chưa được thêm vào nhóm.
func TestTelegramSurfacesServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"ok":false,"description":"Unauthorized"}`))
	}))
	defer server.Close()

	channel, _ := New(Telegram, map[string]string{
		"token": "sai", "chatId": "1", "apiBase": server.URL,
	})
	err := channel.Send(context.Background(), testMessage())
	if !errors.Is(err, ErrSendFailed) {
		t.Fatalf("phải báo lỗi gửi, nhận: %v", err)
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("lỗi chưa giữ nội dung máy chủ trả về: %v", err)
	}
}

func TestWebhookSendsJSONAndHeaders(t *testing.T) {
	var payload map[string]any
	var gotAuth, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &payload)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	channel, err := New(Webhook, map[string]string{
		"url":     server.URL + "/hook",
		"headers": "Authorization: Bearer khoa-bi-mat\nX-Nguon: sunpanel",
	})
	if err != nil {
		t.Fatalf("dựng kênh webhook: %v", err)
	}

	if err := channel.Send(context.Background(), testMessage()); err != nil {
		t.Fatalf("gửi webhook: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("phương thức mặc định phải là POST, nhận %s", gotMethod)
	}
	if gotAuth != "Bearer khoa-bi-mat" {
		t.Errorf("tiêu đề xác thực chưa được gửi: %q", gotAuth)
	}
	if payload["title"] != "Đĩa sắp đầy" {
		t.Errorf("tiêu đề sai: %v", payload["title"])
	}
	// Slack và Discord đều đọc trường text, nên webhook của chúng phải dùng được ngay.
	text, _ := payload["text"].(string)
	if !strings.Contains(text, "Phân vùng / đã dùng 92%") {
		t.Errorf("trường text thiếu nội dung: %q", text)
	}
	if !strings.Contains(text, "may-chu-1") {
		t.Errorf("trường text thiếu tên máy chủ: %q", text)
	}
}
