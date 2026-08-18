#!/usr/bin/env python3
"""Sinh danh mục ứng dụng từ bảng mô tả.

    python3 tools/catalog/generate.py            # sinh lại toàn bộ
    python3 tools/catalog/generate.py gitea n8n  # chỉ sinh vài ứng dụng

Thẻ image được dò từ chính registry chứ không viết tay: một thẻ bịa ra chỉ lộ ra
khi người dùng bấm cài và ngồi chờ tải, kèm thông báo lỗi của Docker chứ không
phải của panel. Kết quả dò được nhớ lại trong tags.json để lần sinh sau không
phải hỏi lại registry, và để bản khác biệt giữa hai lần sinh đọc được.
"""

import json
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import registry  # noqa: E402
from model import render  # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(HERE, "..", "..", "pkg", "appstore", "catalog")
CACHE = os.path.join(HERE, "tags.json")


def load_apps():
    apps = []
    for module in ("apps_core", "apps_media", "apps_dev", "apps_tools", "apps_home",
                   "apps_data", "apps_office", "apps_net",
                   "apps_social", "apps_ai", "apps_biz", "apps_desk", "apps_cms", "apps_iot"):
        try:
            apps.extend(__import__(module).APPS)
        except ModuleNotFoundError:
            continue
    return apps


def resolve_tags(app, cache, refresh):
    """Lấy danh sách thẻ phiên bản của một ứng dụng."""
    if app.fixed_tags:
        return list(app.fixed_tags)

    if not refresh and app.key in cache:
        return cache[app.key]

    if app.registry == "ghcr":
        repo = app.image.split("ghcr.io/", 1)[1]
        tags = registry.ghcr_tags(repo, pages=app.tag_pages)
    else:
        tags = registry.docker_hub_tags(app.image, pages=app.tag_pages)

    picked = registry.pick_versions(
        tags, suffix=app.tag_suffix, prefix=app.tag_prefix,
        count=app.tag_count, min_major=app.min_major, any_suffix=app.tag_any_suffix
    )
    if not picked:
        raise ValueError(f"không thẻ nào khớp trong {len(tags)} thẻ")

    cache[app.key] = picked
    return picked


def main():
    wanted = set(sys.argv[1:])
    refresh = "--refresh" in wanted
    wanted.discard("--refresh")

    cache = {}
    if os.path.exists(CACHE):
        with open(CACHE, encoding="utf-8") as handle:
            cache = json.load(handle)

    apps = load_apps()
    keys = set()
    written = 0
    failed = []

    for app in apps:
        if app.key in keys:
            raise SystemExit(f"định danh {app.key} xuất hiện hai lần")
        keys.add(app.key)

        if wanted and app.key not in wanted:
            continue

        try:
            tags = resolve_tags(app, cache, refresh)
        except Exception as err:  # noqa: BLE001 — gom mọi lỗi để báo một lượt
            failed.append(f"{app.key}: {err}")
            continue

        path = os.path.join(OUT, app.key + ".yaml")
        with open(path, "w", encoding="utf-8") as handle:
            handle.write(render(app, tags))
        written += 1
        print(f"{app.key:22} {', '.join(tags)}")

    with open(CACHE, "w", encoding="utf-8") as handle:
        json.dump(dict(sorted(cache.items())), handle, ensure_ascii=False, indent=2)
        handle.write("\n")

    print(f"\nđã sinh {written}/{len(apps)} ứng dụng")
    if failed:
        print(f"\nkhông dò được thẻ của {len(failed)} ứng dụng:")
        for line in failed:
            print("  " + line)
        raise SystemExit(1)


if __name__ == "__main__":
    main()
