package agent

import (
	"bytes"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/thanhtinz/sunpanel/pkg/host"
)

// maxRequestSize giới hạn kích thước một yêu cầu.
//
// Agent chạy quyền root; một yêu cầu khổng lồ là cách rẻ nhất để làm nó hết bộ nhớ.
const maxRequestSize = 64 << 20

// Server là agent chạy trên máy chủ được quản lý.
type Server struct {
	host  host.Host
	token string
	// version là phiên bản binary, hiện trong phản hồi ping.
	version string
	started time.Time
}

// NewServer tạo agent.
func NewServer(h host.Host, token, version string) *Server {
	return &Server{host: h, token: token, version: version, started: time.Now()}
}

// Handler trả về bộ định tuyến HTTP của agent.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(PathPing, s.wrap(s.handlePing))
	mux.HandleFunc(PathInfo, s.wrap(s.handleInfo))
	mux.HandleFunc(PathExec, s.wrap(s.handleExec))
	mux.HandleFunc(PathFS, s.wrap(s.handleFS))

	// Mọi đường dẫn khác trả 404 không kèm thông tin gì: agent không nên tự giới
	// thiệu mình với các công cụ quét cổng.
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	})
	return mux
}

// wrap kiểm tra xác thực và phiên bản trước khi gọi handler.
func (s *Server) wrap(next func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// So sánh token bằng hàm chống đo thời gian: so sánh chuỗi thường cho phép
		// đoán dần từng ký tự qua thời gian phản hồi.
		provided := r.Header.Get(HeaderToken)
		if subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) != 1 {
			slog.Warn("agent từ chối yêu cầu sai token", "remote", r.RemoteAddr)
			writeError(w, http.StatusUnauthorized, "token không hợp lệ")
			return
		}

		if version := r.Header.Get(HeaderVersion); version != "" && version != APIVersion {
			writeError(w, http.StatusBadRequest, fmt.Sprintf(
				"phiên bản giao thức không khớp: panel %s, agent %s", version, APIVersion))
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)
		if err := next(w, r); err != nil {
			slog.Error("agent xử lý yêu cầu thất bại", "path", r.URL.Path, "error", err)
			writeError(w, http.StatusInternalServerError, err.Error())
		}
	}
}

func (s *Server) handlePing(w http.ResponseWriter, _ *http.Request) error {
	hostname, _ := os.Hostname()
	return writeJSON(w, PingResponse{
		Version: APIVersion, Hostname: hostname,
		AgentVersion: s.version, Uptime: time.Since(s.started),
	})
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) error {
	info, err := s.host.Info(r.Context())
	if err != nil {
		return fmt.Errorf("đọc thông tin hệ thống: %w", err)
	}
	return writeJSON(w, info)
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) error {
	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "yêu cầu không đọc được")
		return nil
	}

	cmd := host.Command{
		Path: req.Path, Args: req.Args, Dir: req.Dir,
		Env: req.Env, Timeout: req.Timeout,
	}
	if req.Stdin != "" {
		cmd.Stdin = []byte(req.Stdin)
	}

	result, err := s.host.Exec(r.Context(), cmd)
	if err != nil {
		// Lệnh không nằm trong allowlist hay không chạy được là lỗi của yêu cầu,
		// không phải lỗi của agent.
		writeError(w, http.StatusBadRequest, err.Error())
		return nil
	}

	return writeJSON(w, ExecResponse{
		ExitCode: result.ExitCode, Stdout: result.Stdout,
		Stderr: result.Stderr, Duration: result.Duration,
	})
}

func (s *Server) handleFS(w http.ResponseWriter, r *http.Request) error {
	var req FSRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "yêu cầu không đọc được")
		return nil
	}

	fs := s.host.FS()
	ctx := r.Context()

	switch req.Action {
	case FSList:
		entries, err := fs.List(ctx, req.Path)
		if err != nil {
			return fsError(w, err)
		}
		return writeJSON(w, FSResponse{Entries: entries})

	case FSStat:
		info, err := fs.Stat(ctx, req.Path)
		if err != nil {
			return fsError(w, err)
		}
		return writeJSON(w, FSResponse{Info: &info})

	case FSRead:
		reader, err := fs.Open(ctx, req.Path)
		if err != nil {
			return fsError(w, err)
		}
		defer reader.Close()

		content, err := io.ReadAll(io.LimitReader(reader, maxRequestSize))
		if err != nil {
			return fmt.Errorf("đọc tệp: %w", err)
		}
		// base64 để dữ liệu nhị phân đi qua JSON nguyên vẹn.
		return writeJSON(w, FSResponse{Content: base64.StdEncoding.EncodeToString(content)})

	case FSWrite:
		content, err := base64.StdEncoding.DecodeString(req.Content)
		if err != nil {
			writeError(w, http.StatusBadRequest, "nội dung không phải base64 hợp lệ")
			return nil
		}
		if err := fs.Write(ctx, req.Path, bytes.NewReader(content), os.FileMode(req.Mode)); err != nil {
			return fsError(w, err)
		}
		return writeJSON(w, FSResponse{})

	case FSMkdir:
		if err := fs.Mkdir(ctx, req.Path, os.FileMode(req.Mode)); err != nil {
			return fsError(w, err)
		}
		return writeJSON(w, FSResponse{})

	case FSRemove:
		if err := fs.Remove(ctx, req.Path, req.Recursive); err != nil {
			return fsError(w, err)
		}
		return writeJSON(w, FSResponse{})

	case FSMove:
		if err := fs.Move(ctx, req.Path, req.NewPath); err != nil {
			return fsError(w, err)
		}
		return writeJSON(w, FSResponse{})

	case FSChmod:
		if err := fs.Chmod(ctx, req.Path, os.FileMode(req.Mode)); err != nil {
			return fsError(w, err)
		}
		return writeJSON(w, FSResponse{})
	}

	writeError(w, http.StatusBadRequest, "thao tác không được hỗ trợ: "+string(req.Action))
	return nil
}

// fsError trả lỗi hệ thống tệp với mã HTTP phù hợp.
//
// Đường dẫn thoát khỏi phạm vi cho phép là lỗi của bên gọi chứ không phải sự cố
// của agent, nên phải trả 400 để panel phân biệt được.
func fsError(w http.ResponseWriter, err error) error {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, host.ErrPathEscapesRoot):
		status = http.StatusForbidden
	case os.IsNotExist(err):
		status = http.StatusNotFound
	case os.IsPermission(err):
		status = http.StatusForbidden
	}
	writeError(w, status, err.Error())
	return nil
}

func writeJSON(w http.ResponseWriter, payload any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}
