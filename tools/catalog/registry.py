"""Dò thẻ image thật từ registry.

Phiên bản trong danh mục phải là thẻ có thật: một thẻ bịa ra chỉ lộ ra khi
người dùng bấm cài và chờ tải xong, kèm thông báo lỗi của Docker chứ không phải
của panel.
"""

import json
import re
import time
import urllib.error
import urllib.parse
import urllib.request

_UA = {"User-Agent": "sunpanel-catalog-generator"}

# Thẻ phiên bản: v tuỳ chọn, 1–3 nhóm số, kèm hậu tố biến thể (alpine, fpm…).
_SEMVER = re.compile(r"^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:\.(\d+))?(-[A-Za-z0-9.-]+)?$")


def _get(url, tries=3):
    for attempt in range(tries):
        try:
            request = urllib.request.Request(url, headers=_UA)
            with urllib.request.urlopen(request, timeout=30) as response:
                return json.load(response)
        except (urllib.error.URLError, TimeoutError, json.JSONDecodeError):
            if attempt == tries - 1:
                raise
            time.sleep(2 * (attempt + 1))
    return None


def docker_hub_tags(repo, pages=3):
    """Liệt kê thẻ của một repo trên Docker Hub, mới nhất trước."""
    if "/" not in repo:
        repo = "library/" + repo

    tags = []
    url = f"https://hub.docker.com/v2/repositories/{repo}/tags?page_size=100&ordering=last_updated"
    for _ in range(pages):
        data = _get(url)
        tags.extend(item["name"] for item in data.get("results", []))
        url = data.get("next")
        if not url:
            break
    return tags


def ghcr_tags(repo, pages=6):
    """Liệt kê thẻ của một repo trên ghcr.io qua vé ẩn danh.

    Kho lớn có hàng nghìn thẻ và registry chỉ trả về tối đa 1000 mỗi lần, nên
    phải đi tiếp theo tiêu đề Link; dừng lại ở trang đầu sẽ chỉ thấy các thẻ
    dựng theo commit và bỏ sót toàn bộ phiên bản phát hành.
    """
    token = _get(f"https://ghcr.io/token?scope=repository:{repo}:pull&service=ghcr.io")["token"]
    url = f"https://ghcr.io/v2/{repo}/tags/list?n=1000"
    tags = []

    for _ in range(pages):
        request = urllib.request.Request(
            url, headers={**_UA, "Authorization": "Bearer " + token}
        )
        with urllib.request.urlopen(request, timeout=30) as response:
            tags.extend(json.load(response).get("tags", []))
            link = response.headers.get("Link", "")

        match = re.search(r"<([^>]+)>;\s*rel=\"?next\"?", link)
        if not match:
            break
        url = urllib.parse.urljoin("https://ghcr.io", match.group(1))

    return tags


def pick_versions(tags, suffix="", prefix="", count=3, min_major=0, any_suffix=False):
    """Chọn các phiên bản đáng đưa vào danh mục.

    Lấy bản mới nhất của mỗi dòng phiên bản lớn, tối đa `count` dòng. Người dùng
    cần chọn giữa "bản 6 mới" và "bản 5 mà website cũ đang chạy", chứ không cần
    một danh sách ba trăm bản vá.
    """
    best = {}
    for tag in tags:
        if prefix:
            if not tag.startswith(prefix):
                continue
            candidate = tag[len(prefix):]
        else:
            candidate = tag
        match = _SEMVER.match(candidate)
        if not match:
            continue
        if not any_suffix and (match.group(5) or "") != suffix:
            continue
        # Bỏ thẻ chỉ có số lớn ("6"): nó trôi theo thời gian nên hai máy cài cùng
        # một "phiên bản" có thể ra hai bản khác nhau.
        if match.group(2) is None:
            continue

        parts = tuple(int(x) if x else 0 for x in match.group(1, 2, 3, 4))
        if parts[0] < min_major:
            continue
        for group in (parts[0], parts[:2]):
            current = best.get(group)
            if current is None or parts > current[0]:
                best[group] = (parts, tag)

    # Ưu tiên mỗi dòng phiên bản lớn một bản: người dùng chọn giữa "bản 8" và
    # "bản 7 mà ứng dụng cũ đang chạy".
    majors = sorted((k for k in best if isinstance(k, int)), reverse=True)
    picked = [best[major][1] for major in majors[:count]]

    # Ứng dụng ở nguyên một dòng phiên bản lớn suốt nhiều năm — Immich, Gitea —
    # thì lấy thêm vài bản nhỏ gần nhất, nếu không sẽ chỉ có đúng một lựa chọn.
    if len(picked) < count and majors:
        minors = sorted(
            (k for k in best if isinstance(k, tuple) and k[0] == majors[0]), reverse=True
        )
        for minor in minors:
            tag = best[minor][1]
            if tag not in picked:
                picked.append(tag)
            if len(picked) == count:
                break

    # Xếp lại theo số phiên bản: bước bù ở trên nối các bản nhỏ vào cuối, làm
    # danh sách hết thứ tự, mà giao diện thì chọn sẵn phần tử đầu tiên.
    order = {}
    for group, (parts, tag) in best.items():
        order[tag] = parts
    return sorted(picked, key=lambda tag: order[tag], reverse=True)
