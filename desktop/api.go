package main

import (
	"context"
	"encoding/json"
	"net/http"
	"path"
	"time"
)

// apiTimeout là thời gian chờ tối đa của một lần đọc thông tin từ máy chủ.
const apiTimeout = 30 * time.Second

// decodeJSON đọc JSON, tách riêng để phần xử lý terminal không phải nhập gói.
func decodeJSON(data []byte, target any) error { return json.Unmarshal(data, target) }

// writeJSON trả về một giá trị dạng JSON.
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(value)
}

// writeAPIError trả về lỗi dạng JSON để giao diện hiện nguyên câu.
func writeAPIError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

// sessionOf tìm phiên theo tham số truy vấn, trả về false nếu chưa kết nối.
func sessionOf(sessions *Sessions, w http.ResponseWriter, r *http.Request) (*Session, bool) {
	session, ok := sessions.Get(r.URL.Query().Get("id"))
	if !ok {
		http.Error(w, "chưa có kết nối", http.StatusNotFound)
		return nil, false
	}
	return session, true
}

// infoHandler trả về thông tin máy chủ: tên máy, nhân, CPU, bộ nhớ, ổ đĩa.
func infoHandler(sessions *Sessions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := sessionOf(sessions, w, r)
		if !ok {
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), apiTimeout)
		defer cancel()

		info, err := session.Client().SystemInfo(ctx)
		if err != nil {
			writeAPIError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, map[string]any{
			"name": session.Server.Name,
			"addr": session.Server.Label(),
			"info": info,
		})
	}
}

// metricsHandler trả về mức dùng CPU, bộ nhớ và ổ đĩa ngay lúc gọi.
func metricsHandler(sessions *Sessions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := sessionOf(sessions, w, r)
		if !ok {
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), apiTimeout)
		defer cancel()

		metrics, err := session.Client().Metrics(ctx)
		if err != nil {
			writeAPIError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, metrics)
	}
}

// filesHandler duyệt thư mục trên máy chủ qua SFTP.
func filesHandler(sessions *Sessions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := sessionOf(sessions, w, r)
		if !ok {
			return
		}

		files, err := session.Files()
		if err != nil {
			writeAPIError(w, http.StatusBadGateway, err)
			return
		}

		dir := r.URL.Query().Get("path")
		if dir == "" || dir == "." {
			// Hiện đường dẫn thật chứ không phải một dấu chấm: mọi bước đi tiếp
			// đều dựng từ đây, và người dùng cần biết mình đang đứng ở đâu.
			home, err := files.Getwd()
			if err != nil {
				writeAPIError(w, http.StatusBadGateway, err)
				return
			}
			dir = home
		}
		dir = path.Clean(dir)

		list, err := files.List(dir)
		if err != nil {
			writeAPIError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, map[string]any{"path": dir, "entries": list})
	}
}
