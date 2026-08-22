module github.com/thanhtinz/sunpanel/desktop

go 1.25.0

require (
	github.com/gorilla/websocket v1.5.3
	github.com/thanhtinz/sunpanel v0.0.0-00010101000000-000000000000
	github.com/webview/webview_go v0.0.0-20240831120633-6173450d4dd6
)

require (
	github.com/kr/fs v0.1.0 // indirect
	github.com/pkg/sftp v1.13.11 // indirect
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
)

// Ứng dụng dùng chung lõi SSH với panel thay vì viết lại: cùng cách nhận diện
// khóa máy chủ, cùng cách đọc thông số, nên hai bên không bao giờ lệch nhau.
replace github.com/thanhtinz/sunpanel => ../
