package v1

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// ptyReadBuffer là kích thước bộ đệm đọc đầu ra của shell.
const ptyReadBuffer = 4096

// TerminalHandler xử lý endpoint terminal qua WebSocket.
type TerminalHandler struct {
	terminal *service.TerminalService
	upgrader websocket.Upgrader
}

// NewTerminalHandler tạo handler terminal.
func NewTerminalHandler(terminal *service.TerminalService) *TerminalHandler {
	return &TerminalHandler{
		terminal: terminal,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  ptyReadBuffer,
			WriteBufferSize: ptyReadBuffer,
			CheckOrigin:     sameOrigin,
		},
	}
}

// Status xử lý GET /api/v1/terminal/status — cho giao diện biết có mở được terminal không.
func (h *TerminalHandler) Status(c *gin.Context) {
	response.OK(c, gin.H{"available": h.terminal.Available()})
}

// clientMessage là tin nhắn từ trình duyệt gửi lên.
type clientMessage struct {
	// Type nhận "input" hoặc "resize".
	Type string `json:"type"`
	// Data là phím người dùng gõ, chỉ dùng với type "input".
	Data string `json:"data"`
	// Cols và Rows chỉ dùng với type "resize".
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// Connect xử lý GET /api/v1/terminal/ws — mở phiên terminal qua WebSocket.
func (h *TerminalHandler) Connect(c *gin.Context) {
	if !h.terminal.Available() {
		response.Fail(c, apperr.TerminalNotSupported)
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Debug("không nâng cấp được kết nối terminal", "error", err)
		return
	}
	defer func() { _ = conn.Close() }()

	session, err := h.terminal.Open(
		c.Request.Context(),
		middleware.UserID(c), middleware.Username(c), c.ClientIP(),
		c.Query("path"),
		parseUint16(c.Query("cols")), parseUint16(c.Query("rows")),
	)
	if err != nil {
		// Kết nối đã nâng cấp rồi nên không trả JSON được nữa; gửi thông báo lỗi
		// dưới dạng văn bản để hiện thẳng trong cửa sổ terminal.
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Không mở được phiên terminal.\r\n"))
		return
	}
	defer func() { _ = session.Close() }()

	// Đầu ra của shell chảy về trình duyệt trong goroutine riêng; goroutine chính
	// đọc phím gõ. Bên nào kết thúc trước thì đóng bên còn lại.
	done := make(chan struct{})
	go h.pumpOutput(conn, session, done)

	h.pumpInput(conn, session)
	<-done
}

// pumpOutput chuyển đầu ra của shell về trình duyệt.
func (h *TerminalHandler) pumpOutput(conn *websocket.Conn, session *service.Session, done chan struct{}) {
	defer close(done)
	defer func() { _ = conn.Close() }()

	buf := make([]byte, ptyReadBuffer)
	for {
		n, err := session.Read(buf)
		if n > 0 {
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			// Gửi dạng nhị phân: đầu ra của shell có thể chứa byte không hợp lệ
			// UTF-8 (ví dụ khi cat một tệp nhị phân), và khung văn bản WebSocket
			// bắt buộc phải là UTF-8 hợp lệ.
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
		}
		if err != nil {
			// Shell đã thoát hoặc PTY đã đóng — kết thúc bình thường.
			return
		}
	}
}

// pumpInput chuyển phím người dùng gõ vào shell.
func (h *TerminalHandler) pumpInput(conn *websocket.Conn, session *service.Session) {
	idleTimeout := h.terminal.IdleTimeout()

	for {
		// Mỗi lần nhận tin lại gia hạn: phiên chỉ bị đóng khi thực sự bị bỏ quên.
		_ = conn.SetReadDeadline(time.Now().Add(idleTimeout))

		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var msg clientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "input":
			if _, err := session.Write([]byte(msg.Data)); err != nil {
				return
			}
		case "resize":
			if err := session.Resize(msg.Cols, msg.Rows); err != nil {
				slog.Debug("không đổi được kích thước terminal", "error", err)
			}
		}
	}
}

// parseUint16 đọc một số nguyên không dấu từ chuỗi, trả về 0 nếu không hợp lệ.
func parseUint16(s string) uint16 {
	var value uint16
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0
		}
		// Giá trị vượt quá uint16 bị coi là không hợp lệ; tầng service sẽ kẹp lại.
		if value > 6553 {
			return 0
		}
		value = value*10 + uint16(ch-'0')
	}
	return value
}
