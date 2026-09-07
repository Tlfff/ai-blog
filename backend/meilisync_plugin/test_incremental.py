"""增量同步暂停、恢复与 Redis 进度续传测试。"""

from __future__ import annotations

import asyncio
import unittest
from types import SimpleNamespace

from meilisync.enums import EventType
from meilisync.schemas import Event, ProgressEvent
from meilisync.settings import Sync

from meilisync_plugin.control import RedisControl
from meilisync_plugin.events import RowEvent
from meilisync_plugin.incremental import IncrementalSynchronizer


class FakeRedis:
    """实现 RedisControl 所需命令。"""

    def __init__(self) -> None:
        self.hashes: dict[str, dict[str, str]] = {}
        self.values: dict[str, str] = {}

    async def hgetall(self, key: str) -> dict[str, str]:
        return dict(self.hashes.get(key, {}))

    async def hget(self, key: str, field: str) -> str | None:
        return self.hashes.get(key, {}).get(field)

    async def hset(self, key: str, mapping: dict[str, str]) -> int:
        self.hashes.setdefault(key, {}).update(
            {name: str(value) for name, value in mapping.items()}
        )
        return len(mapping)

    async def hdel(self, key: str, *fields: str) -> int:
        values = self.hashes.setdefault(key, {})
        for field in fields:
            values.pop(field, None)
        return len(fields)

    async def set(self, key: str, value: str, **kwargs) -> bool | None:
        if kwargs.get("nx") and key in self.values:
            return None
        self.values[key] = value
        return True

    async def eval(self, script: str, numkeys: int, *args) -> int:
        keys = args[:numkeys]
        values = args[numkeys:]
        owner_key = keys[0]
        owner = values[0]
        if "string.sub" in script:
            current = self.values.get(owner_key, "")
            if not current.startswith(owner + "|"):
                return 0
            self.values.pop(owner_key, None)
            return 1
        if self.values.get(owner_key) != owner:
            return 0
        if "KEYS[2], ARGV[1]" in script:
            self.values[keys[1]] = owner + "|" + values[1]
            return 1
        if "HSET" in script:
            progress_key = keys[1]
            mapping = dict(zip(values[1::2], values[2::2], strict=True))
            self.hashes.setdefault(progress_key, {}).update(mapping)
            return 1
        if "EXPIRE" in script:
            return 1
        self.values.pop(owner_key, None)
        return 1

    async def get(self, key: str) -> str | None:
        return self.values.get(key)

    async def exists(self, key: str) -> int:
        return int(key in self.values)

    async def delete(self, key: str) -> int:
        self.values.pop(key, None)
        return 1


class FakeProgress:
    """记录最后一次持久化 Binlog 位点。"""

    def __init__(self, redis: FakeRedis) -> None:
        self.redis = redis
        self.value = {"master_log_file": "mysql-bin.000001", "master_log_position": "4"}
        self.redis.hashes["progress"] = dict(self.value)
        self.set_values: list[dict] = []

    async def get(self) -> dict:
        self.value = await self.redis.hgetall("progress")
        return dict(self.value)

    async def set(self, **value) -> None:
        self.value = {name: str(item) for name, item in value.items()}
        self.redis.hashes["progress"] = dict(self.value)
        self.set_values.append(dict(self.value))


class BlockingSource:
    """发送预设事件后阻塞，直到暂停取消读取。"""

    def __init__(self, progress: dict, events: list[Event]) -> None:
        self.progress = progress
        self.events = events
        self.closed = False

    def __aiter__(self):
        async def iterate():
            try:
                yield ProgressEvent(progress=self.progress)
                for event in self.events:
                    yield event
                await asyncio.Future()
            finally:
                self.closed = True

        return iterate()


class FakeClient:
    async def wait_for_task(self, **_kwargs):
        return SimpleNamespace(status="succeeded")


class FakeMeili:
    """记录已提交的文档事件。"""

    def __init__(self) -> None:
        self.client = FakeClient()
        self.ids: list[int] = []

    async def handle_events_by_type(self, _sync, events, _event_type):
        self.ids.extend(event.data["id"] for event in events)
        return SimpleNamespace(task_uid=len(self.ids))


class FakeSettings:
    def __init__(self, sync: Sync) -> None:
        self.sync = [sync]
        self.meilisearch = SimpleNamespace(insert_size=500, insert_interval=60)

    def get_sync(self, table: str):
        return self.sync[0] if table == self.sync[0].table else None


class IncrementalSynchronizerTest(unittest.TestCase):
    """验证暂停前刷盘，恢复时从持久化位点重建 Binlog source。"""

    def test_pause_flushes_progress_and_resume_recreates_source_from_redis(
        self,
    ) -> None:
        async def scenario() -> tuple[list[int], list[dict], list[dict]]:
            sync = Sync(table="articles", index="articles")
            settings = FakeSettings(sync)
            redis = FakeRedis()
            progress = FakeProgress(redis)
            control = RedisControl(redis, "progress", poll_interval=0.001)
            meili = FakeMeili()
            source_progresses: list[dict] = []

            def source_factory(current: dict):
                source_progresses.append(dict(current))
                position = int(current["master_log_position"])
                event_id, next_position = (1, 100) if position < 100 else (2, 200)
                return BlockingSource(
                    current,
                    [
                        Event(
                            type=EventType.create,
                            table="articles",
                            data={"id": event_id},
                            progress={
                                "master_log_file": "mysql-bin.000001",
                                "master_log_position": next_position,
                            },
                        )
                    ],
                )

            synchronizer = IncrementalSynchronizer(
                settings=settings,
                progress=progress,
                source_factory=source_factory,
                meili=meili,
                control=control,
                poll_interval=0.001,
            )
            task = asyncio.create_task(synchronizer.run())
            while source_progresses == []:
                await asyncio.sleep(0)
            await asyncio.sleep(0.01)

            pause_task = asyncio.create_task(control.request_pause(timeout=1))
            request = await pause_task
            self.assertTrue(request.acknowledged)
            first_saved = await progress.get()
            self.assertEqual("100", first_saved["master_log_position"])

            await control.resume()
            while len(source_progresses) < 2:
                await asyncio.sleep(0.001)
            await asyncio.sleep(0.01)
            second_pause = await control.request_pause(timeout=1)
            self.assertTrue(second_pause.acknowledged)
            second_saved = await progress.get()
            self.assertEqual("200", second_saved["master_log_position"])
            task.cancel()
            with self.assertRaises(asyncio.CancelledError):
                await task
            return meili.ids, source_progresses, [first_saved, second_saved]

        ids, source_progresses, saved = asyncio.run(scenario())

        self.assertEqual([1, 2], ids)
        self.assertEqual("100", source_progresses[1]["master_log_position"])
        self.assertTrue(any(value["master_log_position"] == "100" for value in saved))

    def test_partial_multi_row_packet_never_advances_redis_progress(self) -> None:
        async def scenario() -> tuple[list[int], str]:
            sync = Sync(table="articles", index="articles")
            settings = FakeSettings(sync)
            settings.meilisearch.insert_size = 1
            redis = FakeRedis()
            progress = FakeProgress(redis)
            control = RedisControl(redis, "progress", poll_interval=0.001)
            meili = FakeMeili()

            def source_factory(current: dict):
                return BlockingSource(
                    current,
                    [
                        RowEvent(
                            type=EventType.create,
                            table="articles",
                            data={"id": 1},
                            progress={
                                "master_log_file": "mysql-bin.000001",
                                "master_log_position": 100,
                            },
                            commit_progress=False,
                        )
                    ],
                )

            synchronizer = IncrementalSynchronizer(
                settings=settings,
                progress=progress,
                source_factory=source_factory,
                meili=meili,
                control=control,
                poll_interval=0.001,
            )
            task = asyncio.create_task(synchronizer.run())
            while meili.ids != [1]:
                await asyncio.sleep(0.001)
            paused = await control.request_pause(timeout=1)
            self.assertTrue(paused.acknowledged)
            task.cancel()
            with self.assertRaises(asyncio.CancelledError):
                await task
            current = await progress.get()
            return meili.ids, current["master_log_position"]

        ids, position = asyncio.run(scenario())

        self.assertEqual([1], ids)
        self.assertEqual("4", position)

    def test_start_honors_existing_pause_before_opening_source(self) -> None:
        async def scenario() -> int:
            sync = Sync(table="articles", index="articles")
            redis = FakeRedis()
            progress = FakeProgress(redis)
            control = RedisControl(redis, "progress", poll_interval=0.001)
            await control.request_pause(timeout=0.1)
            calls = 0

            def source_factory(_current: dict):
                nonlocal calls
                calls += 1
                return BlockingSource(_current, [])

            synchronizer = IncrementalSynchronizer(
                settings=FakeSettings(sync),
                progress=progress,
                source_factory=source_factory,
                meili=FakeMeili(),
                control=control,
                poll_interval=0.001,
            )
            task = asyncio.create_task(synchronizer.run())
            await asyncio.sleep(0.01)
            self.assertEqual(0, calls)
            await control.resume()
            while calls == 0:
                await asyncio.sleep(0.001)
            task.cancel()
            with self.assertRaises(asyncio.CancelledError):
                await task
            return calls

        self.assertEqual(1, asyncio.run(scenario()))


if __name__ == "__main__":
    unittest.main()
