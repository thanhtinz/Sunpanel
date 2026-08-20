# Biểu trưng SunPanel

`sunpanel-logo.png` là bản gốc của biểu trưng ngang (biểu tượng + chữ).

Biểu tượng được vẽ lại thành SVG để dùng trong sản phẩm, ở hai nơi:

- `frontend/src/components/BrandLogo.vue` — dùng trong giao diện (menu, trang đăng nhập)
- `frontend/public/favicon.svg` — biểu tượng trên tab trình duyệt

Sửa hình thì phải sửa cả hai: chúng cố ý không dùng chung một tệp, vì biểu tượng
trên tab phải là tệp tĩnh còn biểu tượng trong giao diện phải đổi màu theo nền
sáng/tối.

## Màu

| Vai trò | Sáng | Tối |
|---|---|---|
| Khung cửa sổ | `#011c3e` | `#2c4f80` |
| Nền màn hình | `#ffffff` | `#f4f7fb` |
| Mặt trời | `#f0a500` | `#f0a500` |
| Ba chấm | `#42f2a3` · `#00d2f6` · `#33aba1` | như bên trái |

Khung cửa sổ đổi màu ở nền tối vì màu xanh than nguyên bản gần trùng màu nền:
để nguyên thì biểu tượng trông như một ô trắng trôi lơ lửng.
