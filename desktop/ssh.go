package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thanhtinz/sunpanel/pkg/sshx"
)

// dialTimeout là thời gian chờ tối đa khi mở kết nối tới máy chủ.
const dialTimeout = 20 * time.Second

// Session là một kết nối SSH đang mở tới một máy chủ.
//
// Kết nối được giữ lại giữa các trang: mở terminal rồi bấm sang tab tệp không
// nên phải đăng nhập lại, và mỗi lần bắt tay SSH là một vòng đi về qua mạng.
type Session struct {
	ID     string
	Server Server

	mu     sync.Mutex
	client *sshx.Client
	files  *sshx.Files
}

// Sessions giữ các kết nối SSH đang mở theo định danh máy chủ.
type Sessions struct {
	mu    sync.Mutex
	items map[string]*Session
}

// NewSessions tạo kho phiên rỗng.
func NewSessions() *Sessions {
	return &Sessions{items: map[string]*Session{}}
}

// describeError đổi lỗi của lõi SSH thành câu người dùng đọc được.
//
// Lỗi gốc mang theo tên gói và viết cho người đọc log; ở đây là dòng chữ đỏ ngay
// dưới nút Kết nối, nên phải nói rõ chuyện gì xảy ra và làm gì tiếp.
func describeError(server Server, err error) error {
	switch {
	case errors.Is(err, sshx.ErrHostKeyChanged):
		return fmt.Errorf(
			"khóa của %s khác lần kết nối trước. Máy chủ có thể vừa được cài lại — "+
				"cũng có thể ai đó đang giả làm nó. Chỉ sửa lại máy chủ này để xóa khóa cũ "+
				"khi bạn chắc chắn vì sao khóa đổi",
			server.Host)
	case errors.Is(err, sshx.ErrAuthFailed):
		return fmt.Errorf("máy chủ từ chối đăng nhập của %s: sai mật khẩu, sai khóa, hoặc tài khoản không được phép", server.User)
	case errors.Is(err, sshx.ErrUnreachable):
		return fmt.Errorf("không nối được tới %s: máy chủ không trả lời, sai cổng, hoặc tường lửa chặn", server.Label())
	}
	return err
}

// Open mở kết nối tới một máy chủ và giữ lại phiên.
//
// password là mật khẩu người dùng vừa nhập; để trống thì dùng thứ đã lưu.
func (s *Sessions) Open(ctx context.Context, server Server, password string) (*Session, string, error) {
	if server.Kind != KindSSH {
		return nil, "", errors.New("máy chủ này không phải kết nối SSH")
	}

	credential := sshx.Credential{User: server.User, Password: password}
	if credential.Password == "" {
		credential.Password = server.Password
	}
	if server.KeyPath != "" {
		key, err := os.ReadFile(server.KeyPath)
		if err != nil {
			return nil, "", errors.New("không đọc được tệp khóa: " + err.Error())
		}
		credential.PrivateKey = string(key)
		// Mật khẩu đã nhập được hiểu là để mở khóa riêng chứ không phải để đăng
		// nhập, vì khóa có đặt mật khẩu là chuyện thường.
		credential.Passphrase = credential.Password
		credential.Password = ""
	}

	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	client, err := sshx.Dial(dialCtx, sshx.Target{
		Host:        server.Host,
		Port:        server.sshPort(),
		Credential:  credential,
		Fingerprint: server.Fingerprint,
	})
	if err != nil {
		return nil, "", describeError(server, err)
	}

	session := &Session{ID: server.ID, Server: server, client: client}

	s.mu.Lock()
	if previous := s.items[server.ID]; previous != nil {
		previous.Close()
	}
	s.items[server.ID] = session
	s.mu.Unlock()

	return session, client.Fingerprint(), nil
}

// Get trả về phiên đang mở của một máy chủ.
func (s *Sessions) Get(id string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.items[id]
	return session, ok
}

// Close đóng mọi phiên đang mở.
func (s *Sessions) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, session := range s.items {
		session.Close()
		delete(s.items, id)
	}
}

// Client trả về kết nối SSH của phiên.
func (s *Session) Client() *sshx.Client {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.client
}

// Files mở kênh SFTP, dùng lại kênh cũ nếu đã mở.
func (s *Session) Files() (*sshx.Files, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.files != nil {
		return s.files, nil
	}
	if s.client == nil {
		return nil, errors.New("kết nối đã đóng")
	}

	files, err := s.client.Files()
	if err != nil {
		return nil, err
	}
	s.files = files
	return files, nil
}

// Close đóng phiên.
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.files != nil {
		_ = s.files.Close()
		s.files = nil
	}
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
}

// upgrader nâng cấp yêu cầu HTTP cục bộ thành WebSocket cho terminal.
//
// Không kiểm Origin: máy chủ này chỉ nghe trên vòng lặp nội bộ và chỉ cửa sổ
// của chính ứng dụng biết cổng ngẫu nhiên nó đang dùng.
var upgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

// terminalHandler nối một phiên SSH với xterm.js trong cửa sổ ứng dụng.
func terminalHandler(sessions *Sessions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := sessions.Get(r.URL.Query().Get("id"))
		if !ok {
			http.Error(w, "chưa có kết nối", http.StatusNotFound)
			return
		}
		client := session.Client()
		if client == nil {
			http.Error(w, "kết nối đã đóng", http.StatusGone)
			return
		}

		cols := queryInt(r, "cols", 80)
		rows := queryInt(r, "rows", 24)

		shell, err := client.OpenShell(cols, rows)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			_ = shell.Close()
			return
		}
		defer func() {
			_ = shell.Close()
			_ = conn.Close()
		}()

		// Máy chủ nói gì thì đẩy thẳng ra màn hình. Đọc theo khối chứ không theo
		// dòng: người dùng gõ ký tự nào phải thấy ngay ký tự đó.
		go func() {
			buffer := make([]byte, 8192)
			for {
				n, err := shell.Read(buffer)
				if n > 0 {
					if err := conn.WriteMessage(websocket.BinaryMessage, buffer[:n]); err != nil {
						return
					}
				}
				if err != nil {
					// Phiên kết thúc: đóng cửa sổ WebSocket để giao diện biết mà
					// báo, thay vì treo một terminal đã chết.
					_ = conn.WriteMessage(websocket.CloseMessage,
						websocket.FormatCloseMessage(websocket.CloseNormalClosure, "phiên đã kết thúc"))
					return
				}
			}
		}()

		for {
			kind, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			// Thông điệp dạng chữ là lệnh điều khiển của giao diện, hiện chỉ có
			// đổi kích thước; phần người dùng gõ luôn đi ở dạng nhị phân.
			if kind == websocket.TextMessage {
				handleControl(shell, data)
				continue
			}
			if _, err := shell.Write(data); err != nil {
				return
			}
		}
	}
}

// handleControl xử lý lệnh điều khiển gửi kèm phiên terminal.
func handleControl(shell *sshx.Shell, data []byte) {
	var control struct {
		Type string `json:"type"`
		Cols int    `json:"cols"`
		Rows int    `json:"rows"`
	}
	if err := decodeJSON(data, &control); err != nil {
		return
	}
	if control.Type == "resize" && control.Cols > 0 && control.Rows > 0 {
		_ = shell.Resize(control.Cols, control.Rows)
	}
}

// queryInt đọc một số nguyên từ tham số truy vấn.
func queryInt(r *http.Request, name string, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
