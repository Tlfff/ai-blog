"""文章 Meilisync 转换与索引设置测试。"""

from __future__ import annotations

import asyncio
import json
import sys
import types
import unittest
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from unittest import mock


def _fake_lazy_pinyin(value, style=None):
    mapping = {"搜": "sou", "索": "suo", "入": "ru", "门": "men"}
    letters = [mapping.get(char, char) for char in value]
    if style == "first_letter":
        return [item[:1] for item in letters]
    return letters


fake_pypinyin = types.ModuleType("pypinyin")
fake_pypinyin.Style = types.SimpleNamespace(NORMAL="normal", FIRST_LETTER="first_letter")
fake_pypinyin.lazy_pinyin = _fake_lazy_pinyin
sys.modules.setdefault("pypinyin", fake_pypinyin)

from meilisync_plugin import articles, index_settings  # noqa: E402


@dataclass
class FakeEvent:
    data: dict


class ArticleTransformTest(unittest.TestCase):
    """验证文章 ROW 事件转换契约。"""

    def test_transform_removes_markdown_images_and_code(self):
        row = {
            "id": 9,
            "title": " 搜索入门 ",
            "content": "# 标题\n正文 [链接](https://example.test) ![图](image://12) image://13\n`inline`\n```go\nsecret()\n```\n<img src='x'>结尾",
            "tags": "Go, 搜索，go； Meilisearch",
            "status": 3,
            "updated_time": datetime(2026, 9, 7, 10, 0, 0, 123456),
        }
        document = articles.transform(row)
        self.assertEqual("sousuorumen", document["title_pinyin"])
        self.assertEqual("ssrm", document["title_initials"])
        self.assertEqual(["Go", "搜索", "Meilisearch"], document["tags"])
        self.assertEqual("标题 正文 链接 结尾", document["content_plain"])
        self.assertNotIn("content", document)
        self.assertEqual("2026-09-07T10:00:00.123456", document["updated_time"])

    def test_plugin_mutates_incremental_event(self):
        self.assertFalse(articles.ArticlePlugin.is_global)
        event = FakeEvent({"id": 1, "title": "搜索", "content": "正文", "tags": "", "status": 2})
        returned = asyncio.run(articles.ArticlePlugin().pre_event(event))
        self.assertIs(event, returned)
        self.assertEqual(1, event.data["id"])
        self.assertEqual(2, event.data["status"])
        self.assertEqual("sousuo", event.data["title_pinyin"])


class MeilisyncConfigTest(unittest.TestCase):
    """验证 ROW Binlog、Redis 进度和插件接线配置。"""

    def test_config_declares_mysql_redis_and_articles_sync(self):
        config = (Path(__file__).parents[1] / "configs" / "meilisync.yml").read_text(encoding="utf-8")
        self.assertIn("type: redis", config)
        self.assertIn("type: mysql", config)
        self.assertIn("server_id:", config)
        self.assertIn("charset: utf8mb4", config)
        self.assertIn("table: articles", config)
        self.assertIn("index: articles", config)
        self.assertIn("meilisync_plugin.articles.ArticlePlugin", config)
        self.assertNotIn("    fields:", config)

    def test_plugin_fields_are_preserved_for_meilisync(self):
        event = FakeEvent({"id": 1, "title": "搜索", "content": "正文", "tags": "中文", "status": 3})
        asyncio.run(articles.ArticlePlugin().pre_event(event))
        self.assertEqual(
            {"id", "title", "title_pinyin", "title_initials", "content_plain", "tags", "status", "updated_time"},
            set(event.data),
        )


class IndexSettingsTest(unittest.TestCase):
    """验证公开过滤、字段优先级和设置任务确认。"""

    def test_settings_contract(self):
        settings = index_settings.article_settings()
        self.assertEqual(
            ["title", "title_pinyin", "tags", "title_initials", "content_plain"],
            settings["searchableAttributes"],
        )
        self.assertEqual(["status", "tags"], settings["filterableAttributes"])
        self.assertEqual({"enabled": False}, settings["typoTolerance"])
        self.assertEqual("disabled", settings["prefixSearch"])
        self.assertEqual(["title", "tags", "content_plain"], settings["localizedAttributes"][0]["attributePatterns"])
        self.assertEqual(["zho"], settings["localizedAttributes"][0]["locales"])

    def test_configure_waits_for_successful_task(self):
        with mock.patch.object(
            index_settings,
            "meili_request",
            side_effect=[{"taskUid": 17}, {"status": "processing"}, {"status": "succeeded"}],
        ) as request_mock, mock.patch.object(index_settings.time, "sleep"):
            index_settings.configure()
        self.assertEqual("PATCH", request_mock.call_args_list[0].args[0])
        self.assertEqual("/tasks/17", request_mock.call_args_list[-1].args[1])
        json.dumps(request_mock.call_args_list[0].args[2])


if __name__ == "__main__":
    unittest.main()
