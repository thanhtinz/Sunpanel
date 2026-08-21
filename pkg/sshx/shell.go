package sshx

import (
	"io"

	"golang.org/x/crypto/ssh"
)

// Kích thước mặc định của cửa sổ terminal khi giao diện chưa kịp báo.
const (
	defaultCols = 80
	defaultRows = 24
)

// Shell là một phiên dòng lệnh trên máy chủ từ xa.
type Shell struct {
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
}

// OpenShell mở một phiên dòng lệnh có cấp phát terminal.
func (c *Client) OpenShell(cols, rows int) (*Shell, error) {
	session, err := c.conn.NewSession()
	if err != nil {
		return nil, err
	}

	if cols <= 0 {
		cols = defaultCols
	}
	if rows <= 0 {
		rows = defaultRows
	}

	// ECHO bật và tốc độ khai báo cao: đây là các giá trị một terminal thật gửi
	// đi, và thiếu chúng thì trình soạn thảo trên máy đích vẽ sai màn hình.
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 38400,
		ssh.TTY_OP_OSPEED: 38400,
	}
	if err := session.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		_ = session.Close()
		return nil, err
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		_ = session.Close()
		return nil, err
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		_ = session.Close()
		return nil, err
	}
	// Không cần đọc riêng luồng lỗi: đã cấp phát terminal thì máy chủ trộn sẵn
	// nó vào cùng một luồng, đúng như khi ngồi trước một cửa sổ SSH thật.

	if err := session.Shell(); err != nil {
		_ = session.Close()
		return nil, err
	}
	return &Shell{session: session, stdin: stdin, stdout: stdout}, nil
}

// Read đọc dữ liệu máy chủ gửi về.
func (s *Shell) Read(p []byte) (int, error) { return s.stdout.Read(p) }

// Write gửi phím người dùng gõ sang máy chủ.
func (s *Shell) Write(p []byte) (int, error) { return s.stdin.Write(p) }

// Resize báo cho máy chủ biết cửa sổ đã đổi kích thước.
func (s *Shell) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return s.session.WindowChange(rows, cols)
}

// Close đóng phiên.
func (s *Shell) Close() error {
	_ = s.stdin.Close()
	return s.session.Close()
}
