"""Meilisync Redis 进度兼容启动器测试。"""

from __future__ import annotations

import asyncio
import unittest
from unittest import mock

from meilisync_plugin import runner


class FakeRedisClient:
    """记录 Redis 进度读取和连接关闭。"""

    def __init__(self) -> None:
        self.closed = False

    async def hgetall(self, key: str) -> dict[str, str]:
        self.key = key
        return {"master_log_position": "4"}

    async def aclose(self) -> None:
        self.closed = True


class FakeProgress:
    """提供原始 Redis.get 所需的最小进度对象。"""

    def __init__(self) -> None:
        self.key = "progress"
        self.kwargs = {"dsn": "redis://127.0.0.1:6379/0"}
        self.redis = FakeRedisClient()




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
        self.progress = {"master_log_file": "mysql-bin.000001", "master_log_position": 4}

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
    """验证启动进度读取后不会复用旧事件循环连接。"""

    def test_get_resets_redis_client_after_reading_progress(self) -> None:
        progress = FakeProgress()
        old_client = progress.redis

        current = asyncio.run(runner._get_and_reset_connection(progress))

        self.assertEqual({"master_log_position": "4"}, current)
        self.assertTrue(old_client.closed)
        self.assertIsNot(old_client, progress.redis)

    def test_iter_all_rows_emits_every_row_in_one_binlog_event(self) -> None:
        source = FakeMySQLSource()

        async def connect(**_kwargs):
            return object()

        async def collect() -> list[int]:
            events = runner._iter_all_rows(source)
            await events.__anext__()
            rows = [(await events.__anext__()).data["id"], (await events.__anext__()).data["id"]]
            await events.aclose()
            return rows

        with mock.patch.object(runner.asyncmy, "connect", side_effect=connect), mock.patch.object(runner, "WriteRowsEvent", FakeWriteRowsEvent):
            rows = asyncio.run(collect())

        self.assertEqual([1, 2], rows)
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

        with mock.patch.object(runner.asyncmy, "connect", side_effect=connect), mock.patch.object(runner.asyncio, "sleep", side_effect=no_sleep), mock.patch.object(runner, "WriteRowsEvent", FakeWriteRowsEvent), mock.patch.object(runner.logger, "exception"):
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

        with mock.patch.object(runner.asyncmy, "connect", side_effect=connect), mock.patch.object(runner.asyncio, "sleep", side_effect=no_sleep), mock.patch.object(runner, "WriteRowsEvent", FakeWriteRowsEvent), mock.patch.object(runner.logger, "exception") as log_exception:
            row_id = asyncio.run(collect())

        self.assertEqual(1, row_id)
        self.assertEqual(2, source.create_count)
        self.assertGreaterEqual(log_exception.call_count, 2)
        self.assertIn("Recreate binlog stream error", log_exception.call_args_list[-1].args[0])


if __name__ == "__main__":
    unittest.main()
