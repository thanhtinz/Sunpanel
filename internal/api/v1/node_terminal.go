package v1

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
	"github.com/thanhtinz/sunpanel/pkg/sshx"
)

// remoteIdleTimeout đóng phiên bị bỏ quên.
//
// Mỗi phiên giữ một kết nối SSH mở trên máy đích; quên đóng nghĩa là để lại rác
// trên một máy chủ mà panel không dọn hộ được.
const remoteIdleTimeout = 30 * time.Minute

// NodeTerminalHandler mở phiên dòng lệnh tới máy chủ từ xa qua WebSocket.
type NodeTerminalHandler struct {
	nodes    *service.NodeService
	upgrader websocket.Upgrader
}

// NewNodeTerminalHandler tạo handler terminal từ xa.
func NewNodeTerminalHandler(nodes *service.NodeService) *NodeTerminalHandler {
	return &NodeTerminalHandler{
		nodes: nodes,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  ptyReadBuffer,
			WriteBufferSize: ptyReadBuffer,
			CheckOrigin:     sameOrigin,
		},
	}
}

// Connect xử lý GET /api/v1/nodes/:id/terminal.
func (h *NodeTerminalHandler) Connect(c *gin.Context) {
	id, err := uintParam(c, "id")
	if err != nil {
		response.Fail(c, err)
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Debug("không nâng cấp được kết nối terminal từ xa", "error", err)
		return
	}
	defer func() { _ = conn.Close() }()

	shell, closeAll, err := h.nodes.OpenShell(
		c.Request.Context(), id,
		int(parseUint16(c.Query("cols"))), int(parseUint16(c.Query("rows"))),
		service.AuditEntry{
			UserID:   middleware.UserID(c),
			Username: middleware.Username(c),
			IP:       c.ClientIP(),
		},
	)
	if err != nil {
		// Kết nối đã nâng cấp rồi nên không trả JSON được nữa; gửi thẳng câu lỗi
		// vào cửa sổ terminal, đó là chỗ duy nhất người dùng còn nhìn thấy.
		_ = conn.WriteMessage(websocket.TextMessage, []byte("Không mở được phiên: "+err.Error()+"\r\n"))
		return
	}
	defer closeAll()

	done := make(chan struct{})
	go pumpRemoteOutput(conn, shell, done)

	pumpRemoteInput(conn, shell)
	<-done
}

// pumpRemoteOutput chuyển đầu ra của máy chủ từ xa về trình duyệt.
func pumpRemoteOutput(conn *websocket.Conn, shell *sshx.Shell, done chan struct{}) {
	defer close(done)
	defer func() { _ = conn.Close() }()

	buf := make([]byte, ptyReadBuffer)
	for {
		n, err := shell.Read(buf)
		if n > 0 {
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			// Gửi dạng nhị phân vì đầu ra của shell có thể chứa byte không hợp lệ
			// UTF-8, còn khung văn bản WebSocket bắt buộc phải là UTF-8 hợp lệ.
			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// pumpRemoteInput chuyển phím người dùng gõ sang máy chủ từ xa.
func pumpRemoteInput(conn *websocket.Conn, shell *sshx.Shell) {
	for {
		_ = conn.SetReadDeadline(time.Now().Add(remoteIdleTimeout))

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
			if _, err := shell.Write([]byte(msg.Data)); err != nil {
				return
			}
		case "resize":
			if err := shell.Resize(int(msg.Cols), int(msg.Rows)); err != nil {
				slog.Debug("không đổi được kích thước terminal từ xa", "error", err)
			}
		}
	}
}
