# Biểu trưng ứng dụng

Mỗi tệp ở đây là biểu trưng của một ứng dụng trong danh mục, đặt tên đúng bằng
định danh ứng dụng (`key` trong tệp YAML tương ứng ở `../catalog/`).

## Vì sao nhúng thẳng vào binary

Panel phải hiện đúng biểu trưng trên máy chủ không ra được Internet, và chính
sách nội dung của panel chặn ảnh từ tên miền ngoài (`img-src 'self' data:`). Một
đường dẫn tới CDN sẽ cho ra ô trống đúng lúc cần nhất.

Panel đọc tệp lúc khởi động rồi gửi xuống giao diện dưới dạng data URI. Giao
diện nhúng nó vào thẻ `<img>` chứ không chèn thẳng vào trang: danh mục tự thêm
của quản trị viên cũng là dữ liệu ngoài, mà một tệp SVG chèn thẳng vào trang thì
chạy được mã kịch bản bên trong.

## Quy ước tên tệp

| Tệp | Dùng khi |
|---|---|
| `<định-danh>.svg` (hoặc `.webp`, `.png`, `.jpg`) | mặc định, và khi panel ở chế độ sáng |
| `<định-danh>-dark.<đuôi>` | khi panel ở chế độ tối |

Bản `-dark` chỉ cần khi logo gốc là nét đen trên nền trong suốt — Ghost, Umami,
Vaultwarden, MinIO, IT Tools — vì những logo đó chìm hẳn vào nền tối. Không có
bản `-dark` thì panel dùng chung bản thường.

Ưu tiên ảnh véc-tơ: `.svg` nét gọn ở mọi cỡ và thường nhẹ hơn hẳn. Chỉ dùng ảnh
điểm khi dự án không phát hành bản véc-tơ, và khi đó hãy thu về 128×128 — ô hiển
thị chỉ rộng 44 điểm ảnh, một tệp 512×512 chỉ làm binary phình ra. Kiểm thử sẽ
báo lỗi nếu một biểu trưng vượt 64 KB.

## Nguồn

Logo lấy từ bộ sưu tập [dashboard-icons](https://github.com/homarr-labs/dashboard-icons),
riêng Memos và Uptime Kuma lấy thẳng từ kho mã của chính dự án.

Mỗi logo là **nhãn hiệu của chủ sở hữu tương ứng**, đưa vào đây để người dùng
nhận ra ứng dụng mình định cài. Chúng không thuộc giấy phép của SunPanel, và
việc SunPanel liệt kê một ứng dụng không có nghĩa dự án đó bảo trợ SunPanel.
