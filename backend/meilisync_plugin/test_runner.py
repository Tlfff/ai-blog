"""Meilisync Binlog 兼容与刷新进度测试。"""

from __future__ import annotations

import asyncio
import unittest
from types import SimpleNamespace
from unittest import mock

from meilisync_plugin import runner


class FakeWriteRowsEvent:
    """模拟包含多行的 MySQL WRITE_ROWS_EVENT。"""

    table = "articles"
    rows = [{"values": {"id": 1}}, {"values": {"id": 2}}]


class FakeStream:
    """提供单个多行 Binlog 包及其位点。"""

    _master_log_file = "mysql-bin.000001"
    _master_log_position = 128

    def __aiter__(self):
        async def events():
            yield FakeWriteRowsEvent()

        return events()


class FakeMySQLSource:
    """提供多行迭代补丁所需的最小 MySQL source。"""

    def __init__(self) -> None:
        self.kwargs = {}
        self.progress = {
            "master_log_file": "mysql-bin.000001",
            "master_log_position": 4,
        }

    async def _create_stream(self) -> None:
        self.stream = FakeStream()


class FakeConnection:
    """记录 Binlog 控制连接关闭。"""

    def __init__(self) -> None:
        self.closed = False

    def close(self) -> None:
        self.closed = True


class FailingStream:
    """模拟一次 MySQL Binlog 断链。"""

    def __init__(self) -> None:
        self.closed = False

    def __aiter__(self):
        async def events():
            raise runner.OperationalError(2013, "lost connection")
            yield

        return events()

    async def close(self) -> None:
        self.closed = True


class ReconnectingSource(FakeMySQLSource):
    """在首次断链后切换为正常流，可选模拟重建报错。"""

    def __init__(self, fail_reconnect: bool = False) -> None:
        super().__init__()
        self.create_count = 0
        self.fail_reconnect = fail_reconnect
        self.first_stream = FailingStream()

    async def _create_stream(self) -> None:
        self.create_count += 1
        if self.create_count == 1:
            self.stream = self.first_stream
            return
        self.stream = FakeStream()
        if self.fail_reconnect and self.create_count == 2:
            raise RuntimeError("reconnect failed")


class RunnerTest(unittest.TestCase):
    """验证多行 Binlog、断线重连和刷新失败进度恢复。"""

    def test_iter_all_rows_emits_every_row_in_one_binlog_event(self) -> None:
        source = FakeMySQLSource()

        async def connect(**_kwargs):
            return object()

        async def collect() -> list[int]:
            events = runner._iter_all_rows(source)
            await events.__anext__()
            first = await events.__anext__()
            second = await events.__anext__()
            rows = [first.data["id"], second.data["id"]]
            commit_flags = [first.commit_progress, second.commit_progress]
            await events.aclose()
            return rows, commit_flags

        with (
            mock.patch.object(runner.asyncmy, "connect", side_effect=connect),
            mock.patch.object(runner, "WriteRowsEvent", FakeWriteRowsEvent),
        ):
            rows, commit_flags = asyncio.run(collect())

        self.assertEqual([1, 2], rows)
        self.assertEqual([False, True], commit_flags)
        self.assertEqual(128, source.progress["master_log_position"])

    def test_iter_all_rows_reconnects_after_operational_error(self) -> None:
        source = ReconnectingSource()
        connections = [FakeConnection(), FakeConnection()]

        async def connect(**_kwargs):
            return connections.pop(0)

        async def no_sleep(_seconds: int) -> None:
            return None

        async def collect() -> int:
            events = runner._iter_all_rows(source)
            await events.__anext__()
            row_id = (await events.__anext__()).data["id"]
            await events.aclose()
            return row_id

        with (
            mock.patch.object(runner.asyncmy, "connect", side_effect=connect),
            mock.patch.object(runner.asyncio, "sleep", side_effect=no_sleep),
            mock.patch.object(runner, "WriteRowsEvent", FakeWriteRowsEvent),
            mock.patch.object(runner.logger, "exception"),
        ):
            row_id = asyncio.run(collect())

        self.assertEqual(1, row_id)
        self.assertEqual(2, source.create_count)
        self.assertTrue(source.first_stream.closed)
        self.assertTrue(source.ctl_conn.closed)

    def test_iter_all_rows_logs_reconnect_failure_and_continues(self) -> None:
        source = ReconnectingSource(fail_reconnect=True)
        connections = [FakeConnection(), FakeConnection()]

        async def connect(**_kwargs):
            return connections.pop(0)

        async def no_sleep(_seconds: int) -> None:
            return None

        async def collect() -> int:
            events = runner._iter_all_rows(source)
            await events.__anext__()
            row_id = (await events.__anext__()).data["id"]
            await events.aclose()
            return row_id

        with (
            mock.patch.object(runner.asyncmy, "connect", side_effect=connect),
            mock.patch.object(runner.asyncio, "sleep", side_effect=no_sleep),
            mock.patch.object(runner, "WriteRowsEvent", FakeWriteRowsEvent),
            mock.patch.object(runner.logger, "exception") as log_exception,
        ):
            row_id = asyncio.run(collect())

        self.assertEqual(1, row_id)
        self.assertEqual(2, source.create_count)
        self.assertGreaterEqual(log_exception.call_count, 2)
        self.assertIn(
            "Recreate binlog stream error", log_exception.call_args_list[-1].args[0]
        )

    def test_runtime_closes_redis_even_when_meilisearch_close_fails(self) -> None:
        class MeiliClient:
            async def aclose(self):
                raise RuntimeError("meili close failed")

        class RedisClient:
            def __init__(self) -> None:
                self.closed = False

            async def aclose(self):
                self.closed = True

        async def scenario() -> bool:
            redis_client = RedisClient()
            runtime = runner.Runtime(
                settings=object(),
                progress=SimpleNamespace(redis=redis_client),
                source_factory=object(),
                meili=SimpleNamespace(client=MeiliClient()),
                control=object(),
            )
            with self.assertRaisesRegex(RuntimeError, "meili close failed"):
                await runtime.close()
            return redis_client.closed

        self.assertTrue(asyncio.run(scenario()))

    def test_refresh_success_commits_candidate_progress_after_swap(self) -> None:
        class Progress:
            key = "progress"

            def __init__(self) -> None:
                self.value = {
                    "master_log_file": "mysql-bin.000001",
                    "master_log_position": "100",
                }
                self.redis = self

            async def get(self):
                return dict(self.value)

            async def set(self, **value):
                self.value = {name: str(item) for name, item in value.items()}

        class Source:
            async def get_current_progress(self):
                return {
                    "master_log_file": "mysql-bin.000001",
                    "master_log_position": "200",
                }

        class SuccessfulOperations:
            def __init__(self, *_args, **_kwargs):
                pass

            async def refresh(self, _sync, *, size, after_swap):
                await after_swap()
                return SimpleNamespace(
                    table="articles",
                    index="articles",
                    mysql_count=size,
                    index_count=size,
                )

        async def scenario():
            progress = Progress()
            runtime = SimpleNamespace(
                progress=progress,
                source_factory=lambda _current: Source(),
                meili=object(),
                control=object(),
            )
            with mock.patch.object(runner, "SearchOperations", SuccessfulOperations):
                await runner._refresh_selected(runtime, [object()], 10)
            return progress.value

        progress = asyncio.run(scenario())

        self.assertEqual("200", progress["master_log_position"])

    def test_refresh_accepts_lost_redis_response_when_candidate_is_committed(
        self,
    ) -> None:
        class Progress:
            def __init__(self) -> None:
                self.value = {
                    "master_log_file": "mysql-bin.000001",
                    "master_log_position": "100",
                }

            async def get(self):
                return dict(self.value)

            async def set(self, **value):
                self.value = {name: str(item) for name, item in value.items()}
                raise ConnectionError("response lost")

        class Source:
            async def get_current_progress(self):
                return {
                    "master_log_file": "mysql-bin.000001",
                    "master_log_position": "200",
                }

        class SuccessfulOperations:
            def __init__(self, *_args, **_kwargs):
                pass

            async def refresh(self, _sync, *, size, after_swap):
                await after_swap()
                return SimpleNamespace(
                    table="articles",
                    index="articles",
                    mysql_count=size,
                    index_count=size,
                )

        async def scenario():
            progress = Progress()
            runtime = SimpleNamespace(
                progress=progress,
                source_factory=lambda _current: Source(),
                meili=object(),
                control=object(),
            )
            with mock.patch.object(runner, "SearchOperations", SuccessfulOperations):
                await runner._refresh_selected(runtime, [object()], 10)
            return progress.value

        progress = asyncio.run(scenario())

        self.assertEqual("200", progress["master_log_position"])

    def test_refresh_failure_keeps_previous_redis_progress(self) -> None:
        class Progress:
            key = "progress"

            def __init__(self) -> None:
                self.value = {
                    "master_log_file": "mysql-bin.000001",
                    "master_log_position": "100",
                }
                self.redis = self

            async def get(self):
                return dict(self.value)

            async def set(self, **value):
                self.value = {name: str(item) for name, item in value.items()}

            async def delete(self, _key):
                self.value = {}

        class Source:
            async def get_current_progress(self):
                return {
                    "master_log_file": "mysql-bin.000001",
                    "master_log_position": "200",
                }

        class FailingOperations:
            def __init__(self, *_args, **_kwargs):
                pass

            async def refresh(self, _sync, *, size, after_swap):
                self.size = size
                self.after_swap = after_swap
                raise RuntimeError("refresh failed")

        async def scenario():
            progress = Progress()
            seen = []
            runtime = SimpleNamespace(
                progress=progress,
                source_factory=lambda current: seen.append(dict(current)) or Source(),
                meili=object(),
                control=object(),
            )
            with mock.patch.object(runner, "SearchOperations", FailingOperations):
                with self.assertRaisesRegex(RuntimeError, "refresh failed"):
                    await runner._refresh_selected(runtime, [object()], 10)
            return progress.value, seen

        progress, seen = asyncio.run(scenario())

        self.assertEqual("100", progress["master_log_position"])
        self.assertEqual("100", seen[0]["master_log_position"])


if __name__ == "__main__":
    unittest.main()
