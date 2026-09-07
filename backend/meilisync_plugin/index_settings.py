"""配置 articles 索引字段优先级、公开过滤与中文分词。"""

from __future__ import annotations

import json
import os
import time
from typing import Any
from urllib import error, request

INDEX_NAME = "articles"
DEFAULT_URL = "http://127.0.0.1:7700"
TASK_TIMEOUT_SECONDS = 30


def article_settings() -> dict[str, Any]:
    """返回文章索引的稳定设置契约。"""
    return {
        "searchableAttributes": [
            "title",
            "title_pinyin",
            "tags",
            "title_initials",
            "content_plain",
        ],
        "displayedAttributes": [
            "id",
            "title",
            "tags",
            "status",
            "updated_time",
            "content_plain",
        ],
        "filterableAttributes": ["status", "tags"],
        "typoTolerance": {"enabled": False},
        "prefixSearch": "disabled",
        "localizedAttributes": [
            {
                "attributePatterns": ["title", "tags", "content_plain"],
                "locales": ["zho"],
            }
        ],
    }


def meili_request(method: str, path: str, payload: dict[str, Any] | None = None) -> dict[str, Any]:
    """向 Meilisearch 发送 JSON 请求并返回响应对象。"""
    base_url = os.getenv("MEILISEARCH_URL", DEFAULT_URL).rstrip("/")
    body = None if payload is None else json.dumps(payload).encode("utf-8")
    req = request.Request(base_url + path, data=body, method=method)
    req.add_header("Content-Type", "application/json")
    api_key = os.getenv("MEILISEARCH_API_KEY", "")
    if api_key:
        req.add_header("Authorization", "Bearer " + api_key)
    try:
        with request.urlopen(req, timeout=5) as response:
            return json.load(response)
    except (error.HTTPError, error.URLError, TimeoutError, json.JSONDecodeError) as exc:
        raise RuntimeError(f"Meilisearch 请求失败: {method} {path}") from exc


def wait_task(task_uid: int) -> None:
    """等待异步索引设置任务成功，失败或超时立即终止。"""
    deadline = time.monotonic() + TASK_TIMEOUT_SECONDS
    while time.monotonic() < deadline:
        task = meili_request("GET", f"/tasks/{task_uid}")
        status = task.get("status")
        if status == "succeeded":
            return
        if status in {"failed", "canceled"}:
            raise RuntimeError(f"Meilisearch 索引设置失败: {task.get('error', status)}")
        time.sleep(0.1)
    raise TimeoutError("等待 Meilisearch 索引设置超时")


def configure() -> None:
    """提交并确认 articles 索引设置。"""
    result = meili_request("PATCH", f"/indexes/{INDEX_NAME}/settings", article_settings())
    task_uid = result.get("taskUid")
    if not isinstance(task_uid, int):
        raise RuntimeError("Meilisearch 未返回设置任务标识")
    wait_task(task_uid)


if __name__ == "__main__":
    configure()
