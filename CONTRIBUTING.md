# Đóng góp cho SunPanel

Cảm ơn bạn đã quan tâm tới dự án. Tài liệu này ghi lại những quy ước cần biết trước khi
gửi thay đổi.

## Bắt đầu

```sh
git clone https://github.com/thanhtinz/Sunpanel.git && cd Sunpanel
make frontend && make build
make check          # lint + test, phải xanh trước khi gửi PR
```

Yêu cầu: Go 1.24+, Node 22+.

## Quy ước mã nguồn

**Bình luận viết bằng tiếng Việt.** Đây là dự án của cộng đồng Việt Nam; tên định danh
theo chuẩn Go (tiếng Anh), còn bình luận giải thích bằng tiếng Việt.

**Bình luận giải thích *tại sao*, không mô tả *cái gì*.** Mã nguồn đã nói rõ nó làm gì.
Bình luận tốt là bình luận trả lời được câu hỏi "sao lại làm thế này?".

**Không gọi thẳng `os` hay `os/exec` trong `internal/service`.** Mọi thao tác hệ điều hành
phải đi qua `pkg/host.Host`. Đây là ràng buộc kiến trúc, không phải sở thích — nó là thứ
cho phép hỗ trợ đa node sau này mà không phải viết lại.

**Lỗi trả cho người dùng phải là mã dịch.** Dùng `apperr`, đừng bao giờ trả chuỗi tiếng
Anh hay tiếng Việt cứng từ backend.

**Thêm chuỗi giao diện thì phải thêm ở cả `vi.json` lẫn `en.json`.** CI sẽ chặn nếu thiếu.

## Bảo mật

Mọi thay đổi chạm tới các phần sau bắt buộc phải kèm test:

- `pkg/host/safejoin.go` — chống path traversal
- `internal/service/auth.go` — luồng xác thực
- `internal/middleware/` — phân quyền và kiểm soát truy cập

Nếu phát hiện lỗ hổng, **đừng mở issue công khai** — hãy gửi email trực tiếp cho người
bảo trì để có thời gian vá trước khi công bố.

## Gửi thay đổi

1. Tạo nhánh từ `main`
2. `make check` phải xanh
3. Mô tả rõ *vì sao* cần thay đổi, không chỉ *thay đổi gì*
4. Một PR giải quyết một việc

## Thông điệp commit

Viết ở thể mệnh lệnh, ngắn gọn, giải thích lý do trong phần thân nếu cần:

```
Thêm kiểm tra symlink cho SafeJoin

Trước đây symlink trỏ ra ngoài thư mục gốc vẫn được chấp nhận vì hàm chỉ
làm sạch đường dẫn ở mức văn bản, chưa giải liên kết thật trên đĩa.
```
