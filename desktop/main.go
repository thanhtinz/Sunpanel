// Ứng dụng SunPanel cho máy tính.
//
// Cùng một giao diện panel, nhưng chạy trong cửa sổ riêng của hệ điều hành thay
// vì một tab trình duyệt: có biểu tượng trên thanh tác vụ, không có thanh địa
// chỉ, và nhớ sẵn danh sách máy chủ nên mở lên là vào thẳng.
//
// Ứng dụng không mang theo bản sao nào của giao diện: nó nạp giao diện từ chính
// panel đang chạy. Nhờ vậy nâng cấp panel là ứng dụng có ngay bản mới, không có
// chuyện app và máy chủ lệch phiên bản.
package main

import (
	_ "embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/webview/webview_go"
)

//go:embed picker.html
var pickerHTML []byte

// Kích thước cửa sổ lúc mở.
//
// Panel có menu bên trái và bảng nhiều cột, nên cửa sổ hẹp hơn mức này sẽ rơi
// vào bố cục điện thoại ngay trên máy tính.
const (
	windowWidth  = 1280
	windowHeight = 820
	minWidth     = 900
	minHeight    = 600
)

// backToPickerJS bắt phím quay về danh sách máy chủ.
//
// Chạy trên mọi trang, kể cả giao diện panel từ xa: sau khi cửa sổ đã đi tới
// một máy chủ thì không còn nút nào của ứng dụng để bấm nữa, và đây là đường
// duy nhất quay lại mà không phải tắt cửa sổ.
const backToPickerJS = `
window.addEventListener('keydown', function (event) {
  if (event.ctrlKey && event.shiftKey && (event.key === 'H' || event.key === 'h')) {
    event.preventDefault()
    if (window.showPicker) window.showPicker()
  }
})
`

func main() {
	// WebKit trong máy ảo và container thường không có tăng tốc phần cứng; thiếu
	// hai biến này thì cửa sổ mở ra trắng trơn mà không báo lỗi gì.
	setDefaultEnv("WEBKIT_DISABLE_COMPOSITING_MODE", "1")
	setDefaultEnv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")

	store, err := NewStore()
	if err != nil {
		log.Fatalf("không mở được danh sách máy chủ: %v", err)
	}

	// Trang chọn máy chủ được phục vụ từ một cổng cục bộ ngẫu nhiên thay vì nhúng
	// thẳng bằng data: — trang data: không có gốc riêng, và trình duyệt nhúng từ
	// chối phần lớn thứ chạy trên đó.
	pickerURL, err := servePicker()
	if err != nil {
		log.Fatalf("không mở được trang chọn máy chủ: %v", err)
	}

	view := webview.New(false)
	defer view.Destroy()

	view.SetTitle("SunPanel")
	view.SetSize(windowWidth, windowHeight, webview.HintNone)
	view.SetSize(minWidth, minHeight, webview.HintMin)

	bind(view, store, pickerURL)
	view.Init(backToPickerJS)

	// Mở thẳng máy chủ dùng lần trước: người quản trị mở ứng dụng này để xem một
	// máy cụ thể, không phải để chọn lại từ đầu mỗi sáng.
	if last, ok := store.Last(); ok {
		view.SetTitle("SunPanel — " + last.Name)
		view.Navigate(last.URL)
	} else {
		view.Navigate(pickerURL)
	}

	view.Run()
}

// bind gắn các hàm Go mà trang chọn máy chủ gọi được.
func bind(view webview.WebView, store *Store, pickerURL string) {
	must := func(err error) {
		if err != nil {
			log.Printf("không gắn được hàm cho giao diện: %v", err)
		}
	}

	must(view.Bind("listServers", func() []Server { return store.List() }))

	// Trả về chuỗi lỗi thay vì error: bên gọi là JavaScript, và một lời hứa bị
	// từ chối ở đó khó hiển thị hơn hẳn một câu ngắn hiện ngay dưới ô nhập.
	must(view.Bind("saveServer", func(id, name, address string) string {
		if _, err := store.Save(Server{ID: id, Name: name, URL: address}); err != nil {
			return err.Error()
		}
		return ""
	}))

	must(view.Bind("removeServer", func(id string) string {
		if err := store.Remove(id); err != nil {
			return err.Error()
		}
		return ""
	}))

	must(view.Bind("openServer", func(id string) string {
		for _, server := range store.List() {
			if server.ID != id {
				continue
			}
			if err := store.MarkLast(id); err != nil {
				log.Printf("không ghi nhớ được máy chủ vừa mở: %v", err)
			}
			view.SetTitle("SunPanel — " + server.Name)
			view.Navigate(server.URL)
			return ""
		}
		return "không tìm thấy máy chủ"
	}))

	must(view.Bind("showPicker", func() {
		view.SetTitle("SunPanel")
		view.Navigate(pickerURL)
	}))
}

// servePicker phục vụ trang chọn máy chủ trên một cổng cục bộ.
func servePicker() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(pickerHTML)
	})

	server := &http.Server{Handler: mux}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("máy chủ cục bộ dừng: %v", err)
		}
	}()

	return fmt.Sprintf("http://%s/", listener.Addr().String()), nil
}

// setDefaultEnv đặt biến môi trường nếu người dùng chưa tự đặt.
func setDefaultEnv(key, value string) {
	if os.Getenv(key) == "" {
		_ = os.Setenv(key, value)
	}
}
