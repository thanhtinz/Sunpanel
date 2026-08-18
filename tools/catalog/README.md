# Bộ sinh danh mục ứng dụng

Danh mục ở `pkg/appstore/catalog/` được **sinh ra**, không sửa tay. Muốn đổi một
ứng dụng thì sửa bảng mô tả ở đây rồi chạy lại bộ sinh.

```bash
python3 tools/catalog/generate.py              # sinh lại toàn bộ
python3 tools/catalog/generate.py gitea n8n    # chỉ vài ứng dụng
python3 tools/catalog/generate.py --refresh    # hỏi lại registry để lấy thẻ mới
```

## Vì sao sinh chứ không viết tay

Hơn một trăm ứng dụng mà tuyệt đại đa số giống hệt nhau về cấu trúc: một
container, một cổng, một volume, vài biến môi trường. Viết tay từng tệp là cách
chắc chắn nhất để chúng lệch nhau — chỗ này quên `restart: unless-stopped`, chỗ
kia đặt tên volume khác — và để sửa một thói quen chung phải mở cả trăm tệp.

## Các tệp

| Tệp | Việc |
|---|---|
| `model.py` | mô tả một ứng dụng và dựng YAML từ nó |
| `registry.py` | dò thẻ image thật từ Docker Hub và ghcr.io |
| `generate.py` | ghép hai phần trên lại và ghi ra tệp |
| `apps_*.py` | bảng mô tả ứng dụng, chia theo nhóm |
| `tags.json` | thẻ đã dò được lần trước |

## Phiên bản

Mỗi ứng dụng có nhiều phiên bản cài song song được. Một phiên bản chỉ là một bộ
giá trị biến — hầu như luôn là thẻ image — chứ không phải một khuôn compose
riêng: nhân bản khuôn compose cho từng phiên bản là cách chắc chắn nhất để sửa
lỗi ở một bản rồi quên mất các bản còn lại.

Thẻ được **dò từ chính registry**, không viết tay. Một thẻ bịa ra chỉ lộ ra khi
người dùng bấm cài và ngồi chờ tải, kèm thông báo lỗi của Docker chứ không phải
của panel. Bộ dò lấy bản mới nhất của mỗi dòng phiên bản lớn — người dùng chọn
giữa "bản 8" và "bản 7 mà ứng dụng cũ đang chạy", chứ không cần ba trăm bản vá.
Ứng dụng ở nguyên một dòng phiên bản lớn nhiều năm thì lấy thêm vài bản nhỏ gần
nhất, nếu không sẽ chỉ có đúng một lựa chọn.

`tags.json` nhớ kết quả lần trước để lần sinh sau không phải hỏi lại registry,
và để bản khác biệt giữa hai lần sinh đọc được. Dùng `--refresh` khi muốn cập
nhật lên phiên bản mới.

## Kiểm tra sau khi sinh

```bash
go test ./pkg/appstore/       # lược đồ, biểu trưng, phân loại, khuôn compose
```

Kiểm thử bắt buộc mọi phiên bản dựng ra được tệp compose hợp lệ và chỉ dùng
image đã khai báo. Nếu máy có Docker, nên kiểm thêm bằng `docker compose config`
trên từng tổ hợp ứng dụng × phiên bản — đó là thứ đã bắt được lỗi khuôn compose
lấy nhầm ô mật khẩu làm số hiệu cổng.

## Thêm một ứng dụng

Thêm một `App(...)` vào tệp `apps_*.py` hợp nhóm nhất. Phần lớn ứng dụng chỉ cần
khai báo image, cổng và volume; chỉ khi ứng dụng cần thêm cơ sở dữ liệu hoặc
nhiều dịch vụ mới phải viết `compose=` bằng tay.

Nhớ đặt biểu trưng vào `pkg/appstore/icons/` — xem README ở đó. Kiểm thử sẽ báo
lỗi nếu một ứng dụng không có biểu trưng.
