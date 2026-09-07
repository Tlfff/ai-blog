"""搜索全量刷新、原子交换和一致性检查测试。"""

from __future__ import annotations

import asyncio
import unittest
from copy import deepcopy
from dataclasses import dataclass
from types import SimpleNamespace

from meilisync.settings import Sync

from meilisync_plugin.index_settings import article_settings
from meilisync_plugin.operations import (
    CommitStateUncertain,
    ConsistencyError,
    SearchOperations,
)


@dataclass
class FakeTask:
    """模拟 Meilisearch 异步任务。"""

    task_uid: int


class FakeSettings:
    """提供 SDK settings 的最小序列化接口。"""

    def __init__(self, values: dict):
        self.values = deepcopy(values)

    def model_dump(self, **_kwargs) -> dict:
        return deepcopy(self.values)

    @classmethod
    def model_validate(cls, values: dict) -> "FakeSettings":
        return cls(values)


class FakeIndex:
    """模拟单个 Meilisearch 索引。"""

    def __init__(self, client: "FakeClient", uid: str):
        self.client = client
        self.uid = uid

    async def get_settings(self) -> FakeSettings:
        return FakeSettings(self.client.indexes[self.uid]["settings"])

    async def get_stats(self) -> SimpleNamespace:
        return SimpleNamespace(
            number_of_documents=len(self.client.indexes[self.uid]["documents"])
        )

    async def delete(self) -> FakeTask:
        if self.uid in self.client.fail_delete:
            raise RuntimeError("delete failed")
        del self.client.indexes[self.uid]
        return self.client.task()


class FakeClient:
    """在内存中模拟索引创建、任务等待与 Index Swap。"""

    def __init__(self) -> None:
        self.indexes = {
            "articles": {
                "settings": article_settings(),
                "documents": {99: {"id": 99, "title": "旧数据"}},
            },
            "articles__refresh__stale": {
                "settings": article_settings(),
                "documents": {98: {"id": 98}},
            },
        }
        self.next_task = 1
        self.fail_swap = False
        self.fail_delete: set[str] = set()
        self.swaps: list[tuple[str, str]] = []

    def task(self) -> FakeTask:
        task = FakeTask(self.next_task)
        self.next_task += 1
        return task

    def index(self, uid: str) -> FakeIndex:
        return FakeIndex(self, uid)

    async def create_index(
        self, uid: str, primary_key: str, *, settings: FakeSettings, **_kwargs
    ) -> FakeIndex:
        self.indexes[uid] = {
            "settings": settings.model_dump(by_alias=True, exclude_none=True),
            "documents": {},
            "primary_key": primary_key,
        }
        return self.index(uid)

    async def get_indexes(self, **_kwargs) -> list[FakeIndex]:
        return [self.index(uid) for uid in list(self.indexes)]

    async def swap_indexes(self, pairs: list[tuple[str, str]]) -> FakeTask:
        if self.fail_swap:
            raise RuntimeError("swap failed")
        for stable, temporary in pairs:
            self.indexes[stable], self.indexes[temporary] = (
                self.indexes[temporary],
                self.indexes[stable],
            )
            self.swaps.append((stable, temporary))
        return self.task()

    async def wait_for_task(self, **_kwargs) -> SimpleNamespace:
        return SimpleNamespace(status="succeeded")


class FakeMeili:
    """复用实际转换插件语义后写入内存索引。"""

    def __init__(self, client: FakeClient, fail_on_batch: int | None = None) -> None:
        self.client = client
        self.fail_on_batch = fail_on_batch
        self.batch = 0

    async def add_data(self, sync: Sync, items: list[dict]) -> FakeTask:
        from meilisync_plugin.articles import transform

        self.batch += 1
        if self.batch == self.fail_on_batch:
            raise RuntimeError("insert failed")
        documents = self.client.indexes[sync.index_name]["documents"]
        for item in items:
            document = transform(item)
            documents[document["id"]] = document
        return self.client.task()


class FakeSource:
    """模拟 MySQL 全表分页与数量查询。"""

    def __init__(self, rows: list[dict]) -> None:
        self.rows = rows

    async def get_full_data(self, _sync: Sync, size: int):
        for offset in range(0, len(self.rows), size):
            yield deepcopy(self.rows[offset : offset + size])

    async def get_count(self, _sync: Sync) -> int:
        return len(self.rows)


class FakeControl:
    """记录刷新期间正在使用的临时索引。"""

    def __init__(self) -> None:
        self.active_temp: str | None = None

    async def set_active_temp(self, index: str, _owner: str) -> bool:
        self.active_temp = index
        return True

    async def clear_active_temp(self, _owner: str) -> bool:
        self.active_temp = None
        return True

    async def get_active_temp(self) -> str | None:
        return self.active_temp


class SearchOperationsTest(unittest.TestCase):
    """验证刷新只在完整校验后交换并清理旧索引。"""

    def setUp(self) -> None:
        self.rows = [
            {
                "id": 1,
                "title": "搜索 入门",
                "content": "# 正文",
                "tags": "Go,go",
                "status": 3,
            },
            {
                "id": 2,
                "title": "恢复",
                "content": "image://1 可见",
                "tags": "运维",
                "status": 2,
            },
        ]
        self.sync = Sync(
            table="articles",
            index="articles",
            plugins=["meilisync_plugin.articles.ArticlePlugin"],
        )

    def test_refresh_uses_plugin_settings_swaps_and_cleans_old_indexes(self) -> None:
        client = FakeClient()
        control = FakeControl()
        operations = SearchOperations(
            FakeSource(self.rows),
            FakeMeili(client),
            control=control,
            temp_token=lambda: "new",
        )

        result = asyncio.run(operations.refresh(self.sync, size=1))

        self.assertEqual(2, result.mysql_count)
        self.assertEqual(2, result.index_count)
        self.assertEqual({1, 2}, set(client.indexes["articles"]["documents"]))
        self.assertEqual(
            "sousuorumen", client.indexes["articles"]["documents"][1]["title_pinyin"]
        )
        self.assertEqual(article_settings(), client.indexes["articles"]["settings"])
        self.assertNotIn("articles__refresh__stale", client.indexes)
        self.assertFalse(
            any(uid.startswith("articles__refresh__") for uid in client.indexes)
        )
        self.assertEqual([("articles", "articles__refresh__new")], client.swaps)
        self.assertIsNone(control.active_temp)

    def test_refresh_failure_before_swap_keeps_live_index_unchanged(self) -> None:
        client = FakeClient()
        original = deepcopy(client.indexes["articles"])
        operations = SearchOperations(
            FakeSource(self.rows),
            FakeMeili(client, fail_on_batch=2),
            control=FakeControl(),
            temp_token=lambda: "failed",
        )

        with self.assertRaisesRegex(RuntimeError, "insert failed"):
            asyncio.run(operations.refresh(self.sync, size=1))

        self.assertEqual(original, client.indexes["articles"])
        self.assertEqual([], client.swaps)
        self.assertNotIn("articles__refresh__failed", client.indexes)

    def test_old_index_cleanup_failure_keeps_new_index_and_committed_progress(
        self,
    ) -> None:
        client = FakeClient()
        client.fail_delete.add("articles__refresh__keep-old")
        committed = False
        operations = SearchOperations(
            FakeSource(self.rows),
            FakeMeili(client),
            control=FakeControl(),
            temp_token=lambda: "keep-old",
        )

        async def commit() -> None:
            nonlocal committed
            committed = True

        result = asyncio.run(operations.refresh(self.sync, size=2, after_swap=commit))

        self.assertTrue(committed)
        self.assertEqual(2, result.index_count)
        self.assertEqual({1, 2}, set(client.indexes["articles"]["documents"]))
        self.assertIn("articles__refresh__keep-old", client.indexes)
        self.assertEqual(
            {99}, set(client.indexes["articles__refresh__keep-old"]["documents"])
        )

    def test_uncertain_progress_commit_preserves_both_indexes_for_recovery(
        self,
    ) -> None:
        client = FakeClient()
        control = FakeControl()
        operations = SearchOperations(
            FakeSource(self.rows),
            FakeMeili(client),
            control=control,
            temp_token=lambda: "uncertain",
        )

        async def uncertain_commit() -> None:
            raise CommitStateUncertain("unknown")

        with self.assertRaises(CommitStateUncertain):
            asyncio.run(
                operations.refresh(self.sync, size=2, after_swap=uncertain_commit)
            )

        self.assertEqual({1, 2}, set(client.indexes["articles"]["documents"]))
        self.assertEqual(
            {99}, set(client.indexes["articles__refresh__uncertain"]["documents"])
        )
        self.assertEqual("articles__refresh__uncertain", control.active_temp)

    def test_progress_commit_failure_rolls_back_live_index(self) -> None:
        client = FakeClient()
        original = deepcopy(client.indexes["articles"])
        operations = SearchOperations(
            FakeSource(self.rows),
            FakeMeili(client),
            control=FakeControl(),
            temp_token=lambda: "rollback",
        )

        async def fail_commit() -> None:
            raise RuntimeError("redis commit failed")

        with self.assertRaisesRegex(RuntimeError, "redis commit failed"):
            asyncio.run(operations.refresh(self.sync, size=2, after_swap=fail_commit))

        self.assertEqual(original, client.indexes["articles"])
        self.assertEqual(
            [
                ("articles", "articles__refresh__rollback"),
                ("articles", "articles__refresh__rollback"),
            ],
            client.swaps,
        )

    def test_check_reports_mismatch_without_cleaning_active_temp(self) -> None:
        client = FakeClient()
        client.indexes["articles"]["documents"] = {}
        control = FakeControl()
        control.active_temp = "articles__refresh__stale"
        operations = SearchOperations(
            FakeSource(self.rows), FakeMeili(client), control=control
        )

        with self.assertRaises(ConsistencyError) as raised:
            asyncio.run(operations.check(self.sync))

        self.assertEqual(2, raised.exception.mysql_count)
        self.assertEqual(0, raised.exception.index_count)
        self.assertIn("articles__refresh__stale", client.indexes)

    def test_settings_mismatch_fails_before_creating_temp_index(self) -> None:
        client = FakeClient()
        client.indexes["articles"]["settings"]["filterableAttributes"] = []
        original = deepcopy(client.indexes)
        operations = SearchOperations(
            FakeSource(self.rows), FakeMeili(client), control=FakeControl()
        )

        with self.assertRaisesRegex(ConsistencyError, "索引设置"):
            asyncio.run(operations.refresh(self.sync, size=2))

        self.assertEqual(original, client.indexes)


if __name__ == "__main__":
    unittest.main()
