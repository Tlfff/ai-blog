"""MySQL 全量刷新一致性快照测试。"""

from __future__ import annotations

import asyncio
import unittest
from types import SimpleNamespace
from unittest import mock

from meilisync.settings import Sync
from meilisync.source.mysql import MySQL

from meilisync_plugin.snapshot import MySQLFullSnapshot, source_count


class FakeCursor:
    """按主键游标返回两行文章。"""

    def __init__(self, connection: "FakeConnection") -> None:
        self.connection = connection
        self.result = None

    async def __aenter__(self):
        return self

    async def __aexit__(self, *_args):
        return None

    async def execute(self, query: str, params=()) -> None:
        self.connection.queries.append((query, params))
        if "COUNT(*)" in query:
            self.result = {"count": 2}
        elif query.startswith("SELECT"):
            last_id = params[0] if params else 0
            rows = [{"id": 1}, {"id": 2}]
            self.result = [row for row in rows if row["id"] > last_id][:1]

    async def fetchone(self):
        return self.result

    async def fetchall(self):
        return self.result


class FakeConnection:
    def __init__(self) -> None:
        self.queries: list[tuple[str, tuple]] = []
        self.rolled_back = False
        self.closed = False

    def cursor(self, **_kwargs):
        return FakeCursor(self)

    async def rollback(self) -> None:
        self.rolled_back = True

    def close(self) -> None:
        self.closed = True


class MySQLFullSnapshotTest(unittest.TestCase):
    def test_uses_consistent_transaction_and_primary_key_cursor(self) -> None:
        async def scenario():
            connection = FakeConnection()
            source = SimpleNamespace(kwargs={})
            snapshot = MySQLFullSnapshot(
                source,
                Sync(table="articles", index="articles"),
                1,
            )

            async def connect(**_kwargs):
                return connection

            with mock.patch(
                "meilisync_plugin.snapshot.asyncmy.connect", side_effect=connect
            ):
                async with snapshot:
                    rows = [row async for batch in snapshot.batches() for row in batch]
            return snapshot.count, rows, connection

        count, rows, connection = asyncio.run(scenario())

        self.assertEqual(2, count)
        self.assertEqual([{"id": 1}, {"id": 2}], rows)
        queries = [query for query, _params in connection.queries]
        self.assertIn("START TRANSACTION WITH CONSISTENT SNAPSHOT", queries)
        self.assertTrue(any("WHERE id > %s" in query for query in queries))
        self.assertFalse(any("OFFSET" in query for query in queries))
        self.assertTrue(connection.rolled_back)
        self.assertTrue(connection.closed)

    def test_current_count_closes_mysql_connection(self) -> None:
        async def scenario():
            connection = FakeConnection()
            source = object.__new__(MySQL)
            source.kwargs = {}

            async def connect(**_kwargs):
                return connection

            with mock.patch(
                "meilisync_plugin.snapshot.asyncmy.connect", side_effect=connect
            ):
                count = await source_count(
                    source, Sync(table="articles", index="articles")
                )
            return count, connection.closed

        count, closed = asyncio.run(scenario())

        self.assertEqual(2, count)
        self.assertTrue(closed)


if __name__ == "__main__":
    unittest.main()
