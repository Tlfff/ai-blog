"""将 articles ROW Binlog 事件转换为稳定的 Meilisearch 文档。"""

from __future__ import annotations

import html
import re
import unicodedata
from datetime import date, datetime
from typing import Any

from pypinyin import Style, lazy_pinyin

_FENCED_CODE = re.compile(r"(?:^|\n)\s*(?:```|~~~)[^\n]*\n[\s\S]*?(?:\n\s*(?:```|~~~)(?=\n|$)|$)")
_INLINE_CODE = re.compile(r"`[^`\n]*`")
_MARKDOWN_IMAGE = re.compile(r"!\[[^\]]*\]\([^)]*\)")
_HTML_IMAGE = re.compile(r"<img\b[^>]*>", re.IGNORECASE)
_STABLE_IMAGE = re.compile(r"image://\d+")
_MARKDOWN_LINK = re.compile(r"\[([^\]]+)\]\([^)]*\)")
_HTML_TAG = re.compile(r"<[^>]+>")
_MARKDOWN_PREFIX = re.compile(r"(?m)^\s{0,3}(?:#{1,6}\s+|>\s*|[-+*]\s+|\d+[.)]\s+)")
_MARKDOWN_MARKER = re.compile(r"[*_~|{}\[\]()>#\\]")
_SPACE = re.compile(r"\s+")
_TAG_SEPARATOR = re.compile(r"[,，、;；\s]+")


def plain_markdown(value: Any) -> str:
    """删除图片、代码和 Markdown 语法，仅保留可搜索正文。"""
    text = unicodedata.normalize("NFKC", str(value or ""))
    text = _FENCED_CODE.sub(" ", text)
    text = _INLINE_CODE.sub(" ", text)
    text = _MARKDOWN_IMAGE.sub(" ", text)
    text = _HTML_IMAGE.sub(" ", text)
    text = _STABLE_IMAGE.sub(" ", text)
    text = _MARKDOWN_LINK.sub(r"\1", text)
    text = _HTML_TAG.sub(" ", text)
    text = _MARKDOWN_PREFIX.sub("", text)
    text = _MARKDOWN_MARKER.sub(" ", text)
    return _SPACE.sub(" ", html.unescape(text)).strip()


def normalize_tags(value: Any) -> list[str]:
    """规范化、去重并保留文章标签边界。"""
    normalized = unicodedata.normalize("NFKC", str(value or ""))
    result: list[str] = []
    seen: set[str] = set()
    for raw in _TAG_SEPARATOR.split(normalized):
        tag = raw.strip()
        key = tag.casefold()
        if not tag or key in seen:
            continue
        seen.add(key)
        result.append(tag)
    return result


def compact_pinyin(value: str, style: Style = Style.NORMAL) -> str:
    """生成无空格的小写标题拼音或首字母。"""
    parts = lazy_pinyin(unicodedata.normalize("NFKC", value), style=style)
    return "".join(parts).replace(" ", "").lower()


def normalized_time(value: Any) -> str | None:
    """将 MySQL 日期时间转换为 Meilisearch 可序列化文本。"""
    if value is None or value == "":
        return None
    if isinstance(value, datetime):
        return value.isoformat(timespec="microseconds")
    if isinstance(value, date):
        return value.isoformat()
    return str(value)


def transform(row: dict[str, Any]) -> dict[str, Any]:
    """将 articles 数据行转换为搜索上下文发布文档。"""
    title = unicodedata.normalize("NFKC", str(row.get("title", ""))).strip()
    return {
        "id": int(row["id"]),
        "title": title,
        "title_pinyin": compact_pinyin(title),
        "title_initials": compact_pinyin(title, Style.FIRST_LETTER),
        "content_plain": plain_markdown(row.get("content")),
        "tags": normalize_tags(row.get("tags")),
        "status": int(row.get("status", 0)),
        "updated_time": normalized_time(row.get("updated_time")),
    }


class ArticlePlugin:
    """ArticlePlugin 在 Meilisync 写入前转换文章 ROW 事件。"""

    is_global = False

    async def pre_event(self, event: Any) -> Any:
        """使用同一转换处理 INSERT、UPDATE 和 DELETE 事件数据。"""
        event.data = transform(dict(event.data))
        return event

    async def post_event(self, event: Any) -> Any:
        """保留已写入事件，供后续插件继续处理。"""
        return event
