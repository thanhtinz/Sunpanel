package v1

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/thanhtinz/sunpanel/internal/apperr"
	"github.com/thanhtinz/sunpanel/internal/middleware"
	"github.com/thanhtinz/sunpanel/internal/response"
	"github.com/thanhtinz/sunpanel/internal/service"
)

// Các mốc thời gian của giao thức WebSocket giám sát.
const (
	// writeWait là thời gian tối đa chờ ghi xong một khung tin.
	writeWait = 10 * time.Second
	// pongWait là thời gian tối đa chờ phản hồi ping từ trình duyệt.
	pongWait = 60 * time.Second
	// pingPeriod phải nhỏ hơn pongWait để kịp phát hiện kết nối chết.
	pingPeriod = (pongWait * 9) / 10
)

// MonitorHandler xử lý các endpoint giám sát tài nguyên.
type MonitorHandler struct {
	monitor  *service.MonitorService
	upgrader websocket.Upgrader
}

// NewMonitorHandler tạo handler giám sát.
func NewMonitorHandler(monitor *service.MonitorService) *MonitorHandler {
	return &MonitorHandler{
		monitor: monitor,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 4096,
			// Giao diện được phục vụ từ chính máy chủ này, nên chỉ chấp nhận
			// WebSocket cùng nguồn — đây là hàng rào chống tấn công cross-site
			// WebSocket hijacking.
			CheckOrigin: sameOrigin,
		},
	}
}

// Overview xử lý GET /api/v1/monitor/overview.
func (h *MonitorHandler) Overview(c *gin.Context) {
	info, err := h.monitor.SystemInfo(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}

	response.OK(c, gin.H{
		"system":  info,
		"current": h.monitor.Latest(),
	})
}

// Current xử lý GET /api/v1/monitor/current.
func (h *MonitorHandler) Current(c *gin.Context) {
	response.OK(c, h.monitor.Latest())
}

// History xử lý GET /api/v1/monitor/history?window=1h.
func (h *MonitorHandler) History(c *gin.Context) {
	window := time.Hour
	if raw := c.Query("window"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			response.Fail(c, apperr.BadRequest.Wrap(err))
			return
		}
		window = parsed
	}

	samples, err := h.monitor.History(c.Request.Context(), window)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, samples)
}

// Stream xử lý GET /api/v1/monitor/stream — đẩy số liệu realtime qua WebSocket.
func (h *MonitorHandler) Stream(c *gin.Context) {
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrader đã tự ghi phản hồi lỗi vào kết nối.
		slog.Debug("không nâng cấp được kết nối WebSocket", "error", err)
		return
	}
	defer func() { _ = conn.Close() }()

	snapshots, unsubscribe := h.monitor.Subscribe()
	defer unsubscribe()

	// Gửi ngay trạng thái hiện tại để biểu đồ có dữ liệu mà không phải chờ chu kỳ sau.
	if err := writeJSON(conn, h.monitor.Latest()); err != nil {
		return
	}

	go readUntilClose(conn)

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case snap, ok := <-snapshots:
			if !ok {
				return
			}
			if err := writeJSON(conn, snap); err != nil {
				slog.Debug("kết thúc luồng giám sát", "user", middleware.Username(c), "error", err)
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.Request.Context().Done():
			return
		}
	}
}

func writeJSON(conn *websocket.Conn, v any) error {
	if err := conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return err
	}
	return conn.WriteJSON(v)
}

// readUntilClose đọc và bỏ qua mọi tin từ trình duyệt.
//
// Luồng này chỉ có một chiều, nhưng vẫn phải đọc: đó là cách duy nhất để nhận
// khung pong và phát hiện kết nối đã đóng.
func readUntilClose(conn *websocket.Conn) {
	defer func() { _ = conn.Close() }()

	conn.SetReadLimit(512)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// sameOrigin chỉ chấp nhận WebSocket đến từ chính máy chủ đang phục vụ giao diện.
func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Không phải trình duyệt (ví dụ công cụ dòng lệnh); token vẫn được kiểm tra
		// ở lớp xác thực nên không có rủi ro cross-site ở đây.
		return true
	}

	parsed, err := http.NewRequest(http.MethodGet, origin, nil)
	if err != nil {
		return false
	}
	return parsed.URL.Host == r.Host
}
