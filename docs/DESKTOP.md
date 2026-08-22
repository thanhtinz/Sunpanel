# Ứng dụng máy tính

`desktop/` là ứng dụng gốc của SunPanel: cùng một giao diện panel, nhưng chạy
trong cửa sổ riêng của hệ điều hành thay vì một tab trình duyệt — có biểu tượng
trên thanh tác vụ, không có thanh địa chỉ, và nhớ sẵn danh sách máy chủ nên mở
lên là vào thẳng.

Ứng dụng **không mang theo bản sao nào của giao diện**: nó nạp giao diện từ
chính panel đang chạy. Nâng cấp panel là ứng dụng có ngay bản mới, không có
chuyện app và máy chủ lệch phiên bản.

## Cách dùng

Lần đầu mở lên, ứng dụng hiện trang chọn máy chủ. Thêm một máy chủ cần hai thứ:

| Ô | Ví dụ |
|---|---|
| Tên | `VPS Sài Gòn` |
| Địa chỉ | `https://203.0.113.10:9527/qvzQfJuo56JQ/` |

Địa chỉ là URL đầy đủ kèm **đường dẫn bí mật** mà panel in ra lúc cài. Thiếu
giao thức thì ứng dụng tự hiểu là `http://`, và tự thêm dấu gạch chéo cuối
đường dẫn — nếu không, thẻ `base` của panel phân giải sai và giao diện nạp tài
nguyên từ thư mục cha.

Bấm **Kết nối** để mở máy chủ. Máy chủ vừa mở được ghi nhớ, lần chạy sau ứng
dụng vào thẳng máy đó. Đang ở trong panel, bấm **Ctrl+Shift+H** để quay về danh
sách máy chủ.

Danh sách nằm ở `servers.json` trong thư mục cấu hình của người dùng
(`~/.config/sunpanel/` trên Linux, `%AppData%\sunpanel\` trên Windows,
`~/Library/Application Support/sunpanel/` trên macOS), quyền `0600` vì địa chỉ
panel có kèm đường dẫn bí mật — thứ đứng giữa người lạ và trang đăng nhập.

## Build

Ứng dụng là một **mô-đun Go riêng** chứ không nằm trong `./...` của panel. Lý do:
nó cần cgo để gọi trình duyệt nhúng của hệ điều hành, còn binary panel phải giữ
`CGO_ENABLED=0` để biên dịch chéo cho sáu nền tảng từ một máy. Tách mô-đun giữ
hai ràng buộc đó không đụng nhau.

```bash
make desktop        # ra bin/sunpanel-desktop
make desktop-test   # test của riêng ứng dụng
```

Mỗi hệ điều hành build trên chính nó — không biên dịch chéo được, vì phần
trình duyệt nhúng là thư viện hệ thống:

| Hệ điều hành | Trình duyệt nhúng | Cần cài trước khi build |
|---|---|---|
| Linux | WebKitGTK | `libgtk-3-dev` và `libwebkit2gtk-4.1-dev` (Debian/Ubuntu), `gtk3-devel` và `webkit2gtk4.1-devel` (Fedora/Rocky) |
| Windows | WebView2 | Chỉ cần Go kèm trình biên dịch C (MinGW-w64); WebView2 Runtime có sẵn trên Windows 11 và Windows 10 đã cập nhật |
| macOS | WKWebView | Xcode Command Line Tools |

Trên Linux, thư viện `webview` khai báo phụ thuộc `webkit2gtk-4.0` trong khi
Ubuntu 24.04 trở lên chỉ còn gói `4.1`. `desktop/pkgconfig/webkit2gtk-4.0.pc`
là tệp chuyển tiếp cho trường hợp đó, và `make desktop` đã trỏ `PKG_CONFIG_PATH`
vào sẵn. Máy nào còn `4.0` thật thì tệp này không được dùng tới.

## Điện thoại

Chưa có bản gốc cho Android và iOS. Trong lúc chờ, panel là ứng dụng web cài
được: mở panel bằng trình duyệt trên điện thoại rồi chọn **Cài ứng dụng** trong
menu tài khoản — biểu tượng nằm ngoài màn hình chính và mở lên không có thanh
địa chỉ.
