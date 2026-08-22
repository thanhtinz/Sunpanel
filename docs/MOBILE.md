# Ứng dụng điện thoại

`mobile/android/` là ứng dụng Android gốc của SunPanel: biểu tượng riêng trên
màn hình chính, mở lên là vào thẳng panel, không thanh địa chỉ, không tab.

Giống bản máy tính, ứng dụng **không mang theo bản sao nào của giao diện** mà
nạp thẳng từ panel đang chạy — nâng cấp panel là có ngay bản mới.

Ứng dụng nối tới máy chủ theo hai cách, chọn ngay trong biểu mẫu thêm máy chủ:

| Kiểu | Dùng khi |
|---|---|
| **Panel đã cài** | Máy chủ đã có SunPanel; ứng dụng mở thẳng giao diện panel. |
| **SSH thẳng vào VPS** | Máy chủ chưa cài gì. Ứng dụng tự nối SSH và cho terminal kèm thông số máy. |

## Panel đã cài

Bấm nút cộng và điền:

| Ô | Ví dụ |
|---|---|
| Tên gợi nhớ | `VPS Sài Gòn` |
| Địa chỉ panel | `https://203.0.113.10:9527/qvzQfJuo56JQ/` |

Địa chỉ là URL đầy đủ kèm **đường dẫn bí mật** panel in ra lúc cài. Thiếu giao
thức thì hiểu là `http://`, và dấu gạch chéo cuối được thêm tự động.

Bấm **Kết nối** để mở. Panel vừa mở được ghi nhớ nên lần sau vào thẳng. Nút quay
lại của hệ thống đi lùi trong panel, hết lịch sử thì về danh sách máy chủ.

## SSH thẳng vào VPS

Điền địa chỉ máy, cổng (mặc định 22), tên đăng nhập, rồi dán khóa riêng hoặc điền
mật khẩu. Bấm **Kết nối** là vào thẳng terminal, kèm thanh CPU / RAM / đĩa của
máy chủ đo lại mỗi năm giây.

Terminal là xterm.js chạy trong WebView chứ không phải một bộ giả lập viết tay,
nên `vim`, `htop`, `docker logs -f` đều vẽ đúng. Bàn phím ảo của điện thoại không
có Ctrl, Tab, Esc hay phím mũi tên — thiếu chúng thì không thoát nổi `vim` hay
dừng nổi một lệnh đang chạy — nên có thêm một hàng phím phụ ngay trên bàn phím,
gồm cả Ctrl bấm giữ để gõ tổ hợp.

Mật khẩu **không được lưu** trừ khi bạn tự đánh dấu *Nhớ mật khẩu trên máy này*;
mặc định ứng dụng hỏi lại mỗi lần kết nối.

Lần kết nối đầu, khóa của máy chủ được ghi nhận theo đúng vân tay SHA-256 mà
`ssh-keygen` và panel in ra. Từ lần sau khóa phải khớp: khóa đổi thì ứng dụng từ
chối và nói rõ vì sao. Đổi địa chỉ máy chủ sang máy khác thì khóa cũ bị bỏ.

## Chứng chỉ tự ký

Panel mới cài dùng chứng chỉ tự ký, mà WebView của Android thì chặn thẳng. Ứng
dụng hỏi một lần và hiện **vân tay SHA-256** của chứng chỉ; đối chiếu với vân
tay panel hiện ra rồi mới bấm Tin. Vân tay được nhớ theo từng máy chủ, nên lần
chứng chỉ thật sự đổi ứng dụng vẫn hỏi lại — đúng cách panel đối xử với khóa
máy chủ SSH. Đổi địa chỉ máy chủ sang máy khác thì vân tay cũ bị bỏ.

HTTP không mã hóa vẫn mở được, vì phần lớn panel chỉ chạy trong mạng nội bộ và
chưa gắn chứng chỉ. Ra ngoài Internet thì nên gắn chứng chỉ thật rồi đổi địa chỉ
sang `https://`.

Danh sách máy chủ nằm trong vùng riêng của ứng dụng, ứng dụng khác không đọc
được, và không đi vào bản sao lưu của hệ thống — địa chỉ có kèm đường dẫn bí mật,
thứ đứng giữa người lạ và trang đăng nhập.

## Build

```bash
cd mobile/android
./gradlew :app:testDebugUnitTest   # kiểm phần xử lý địa chỉ và danh sách
./gradlew :app:assembleDebug       # ra app/build/outputs/apk/debug/
```

Phần xử lý danh sách máy chủ, địa chỉ và số liệu tài nguyên viết thuần Kotlin,
tách khỏi Android, nên 28 bài kiểm thử chạy thẳng trên JVM.

Cần JDK 17 và Android SDK (compileSdk 35). Chạy được từ Android 7.0 trở lên —
đủ mới để WebView có sẵn WebSocket mà bảng giám sát cần, đủ cũ để chạy trên máy
đời thấp. Mỗi lần đẩy mã, CI dựng sẵn một bản `debug` và đính kèm vào lần chạy.

Bản phát hành lên cửa hàng phải tự ký bằng khóa của người dựng bản; kho mã không
giữ khóa ký nào.

## iPhone

Chưa có bản gốc cho iOS: dựng và ký được bản iOS thì phải có máy macOS kèm Xcode
và tài khoản nhà phát triển của Apple. Trong lúc chờ, mở panel bằng Safari rồi
chọn **Thêm vào màn hình chính** — biểu tượng nằm ngoài màn hình chính và mở lên
không có thanh địa chỉ.
