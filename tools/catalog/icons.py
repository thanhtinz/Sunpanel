#!/usr/bin/env python3
"""Tải biểu trưng chính thức cho mọi ứng dụng trong danh mục.

    python3 tools/catalog/icons.py        # tải cái còn thiếu
    python3 tools/catalog/icons.py --all  # tải lại tất cả

Sau đó chạy tiếp bộ thu nhỏ nếu có tệp cần:

    node tools/catalog/resize.mjs

Bước này từng chỉ nằm trong đầu người làm, nên mỗi lần chạy lại nó khôi phục
đúng những tệp đã bị thay bằng tay — ví dụ một SVG vẽ quá chi tiết đã được dựng
sẵn thành ảnh điểm. Quy tắc kích thước vì vậy nằm ngay trong mã.
"""

import json
import os
import re
import sys
import urllib.error
import urllib.request

BASE = "https://raw.githubusercontent.com/homarr-labs/dashboard-icons/main"
HERE = os.path.dirname(os.path.abspath(__file__))
CATALOG = os.path.join(HERE, "..", "..", "pkg", "appstore", "catalog")
ICONS = os.path.join(HERE, "..", "..", "pkg", "appstore", "icons")
RESIZE_LIST = os.path.join(HERE, "resize.json")

# Kiểm thử chặn biểu trưng vượt 64 KB sau khi mã hóa base64, tức khoảng 48 KB
# tệp gốc. Ảnh véc-tơ vượt ngưỡng đó là vẽ quá chi tiết cho một ô 44 điểm ảnh,
# nên lấy bản ảnh điểm rồi thu nhỏ thay vì mang cả tệp vào binary.
MAX_VECTOR_BYTES = 48 * 1024

# Tên trong bộ sưu tập khác định danh trong danh mục.
ALIAS = {
    "actual": "actual-budget",
    "cloudflared": "cloudflare",
    "drawio": "draw-io",
    "gatus": "gatus",
    "gitea-runner": "gitea",
    "mongo": "mongodb",
    "nodered": "node-red",
    "homeassistant": "home-assistant",
    "openwebui": "open-webui",
    "pihole": "pi-hole",
    "postgres": "postgresql",
    "restic-rest": "restic",
    "rocketchat": "rocket-chat",
    "statping-ng": "statping-ng",
    "tandoor": "tandoor-recipes",
    "tubearchivist": "tube-archivist",
    "volume-backup": "docker-volume-backup",
    "wg-easy": "wireguard",
    "wikijs": "wikijs",
}


def fetch(name, ext):
    url = f"{BASE}/{ext}/{name}.{ext}"
    try:
        request = urllib.request.Request(url, headers={"User-Agent": "sunpanel-icons"})
        with urllib.request.urlopen(request, timeout=30) as response:
            return response.read()
    except (urllib.error.URLError, TimeoutError):
        return None


def existing(key):
    """Các tệp biểu trưng đã có của một định danh."""
    return [f for f in os.listdir(ICONS) if re.fullmatch(rf"{re.escape(key)}\.(svg|webp|png|jpe?g)", f)]


def save(key, suffix, data, ext, resize):
    """Ghi tệp biểu trưng, xếp hàng thu nhỏ nếu là ảnh điểm."""
    target = key + suffix
    if ext == "svg":
        path = os.path.join(ICONS, target + ".svg")
        with open(path, "wb") as handle:
            handle.write(data)
        return

    raw = os.path.join("/tmp", f"sunpanel-icon-{target}.{ext}")
    with open(raw, "wb") as handle:
        handle.write(data)
    resize.append([raw, os.path.join(ICONS, target + ".webp")])


def main():
    refetch = "--all" in sys.argv
    keys = sorted(f[:-5] for f in os.listdir(CATALOG) if f.endswith(".yaml"))

    resize = []
    missing = []
    for key in keys:
        if not refetch and existing(key):
            continue

        name = ALIAS.get(key, key)
        found = False

        # Bản thường, rồi bản dùng cho nền tối nếu dự án có phát hành.
        for variant, suffix in ((name, ""), (name + "-light", "-dark")):
            svg = fetch(variant, "svg")
            if svg is not None and len(svg) <= MAX_VECTOR_BYTES:
                save(key, suffix, svg, "svg", resize)
                found = found or suffix == ""
                continue

            for ext in ("webp", "png"):
                raster = fetch(variant, ext)
                if raster is None:
                    continue
                save(key, suffix, raster, ext, resize)
                found = found or suffix == ""
                break

        if not found:
            missing.append(key)

    with open(RESIZE_LIST, "w", encoding="utf-8") as handle:
        json.dump(resize, handle, indent=2)
        handle.write("\n")

    print(f"đã xét {len(keys)} ứng dụng, {len(resize)} ảnh điểm cần thu nhỏ")
    if resize:
        print("chạy tiếp: node tools/catalog/resize.mjs")
    if missing:
        print(f"\nkhông tìm được biểu trưng của {len(missing)} ứng dụng:")
        for key in missing:
            print("  " + key)


if __name__ == "__main__":
    main()
