"""MySQL 全量刷新一致性快照。"""

from __future__ import annotations

from typing import Any

import asyncmy
from asyncmy.cursors import DictCursor
from meilisync.settings import Sync
from meilisync.source.mysql import MySQL


class MySQLFullSnapshot:
    """在同一 REPEATABLE READ 快照内按主键游标读取全表和数量。"""

    def __init__(self, source: MySQL, sync: Sync, size: int) -> None:
        self.source = source
        self.sync = sync
        self.size = size
        self.connection = None
        self.count = 0

    async def __aenter__(self) -> "MySQLFullSnapshot":
        self.connection = await asyncmy.connect(**self.source.kwargs)
        async with self.connection.cursor(cursor=DictCursor) as cursor:
            await cursor.execute("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ")
            await cursor.execute("START TRANSACTION WITH CONSISTENT SNAPSHOT")
            await cursor.execute(f"SELECT COUNT(*) AS count FROM {self.sync.table}")
            self.count = int((await cursor.fetchone())["count"])
        return self

    async def __aexit__(self, _exc_type, _exc_value, _traceback) -> None:
        if self.connection is None:
            return
        try:
            await self.connection.rollback()
        finally:
            self.connection.close()

    async def batches(self):
        """使用主键游标分页，避免 OFFSET 在并发变化下移位漏行。"""
        if self.connection is None:
            raise RuntimeError("MySQL 快照尚未打开")
        fields = self._fields_sql()
        primary_key = self.sync.pk
        mapped_primary_key = (self.sync.fields or {}).get(primary_key) or primary_key
        last_primary_key: Any | None = None
        while True:
            where = "" if last_primary_key is None else f"WHERE {primary_key} > %s "
            params = () if last_primary_key is None else (last_primary_key,)
            query = (
                f"SELECT {fields} FROM {self.sync.table} {where}"
                f"ORDER BY {primary_key} LIMIT {self.size}"
            )
            async with self.connection.cursor(cursor=DictCursor) as cursor:
                await cursor.execute(query, params)
                rows = await cursor.fetchall()
            if not rows:
                return
            last_primary_key = rows[-1][mapped_primary_key]
            yield rows

    def _fields_sql(self) -> str:
        if not self.sync.fields:
            return "*"
        return ", ".join(
            f"{field} AS {alias or field}" for field, alias in self.sync.fields.items()
        )


class SourceFullSnapshot:
    """供非 MySQL 测试替身复用原 source 接口。"""

    def __init__(self, source: Any, sync: Sync, size: int) -> None:
        self.source = source
        self.sync = sync
        self.size = size
        self.count = 0

    async def __aenter__(self) -> "SourceFullSnapshot":
        self.count = await self.source.get_count(self.sync)
        return self

    async def __aexit__(self, _exc_type, _exc_value, _traceback) -> None:
        return None

    def batches(self):
        return self.source.get_full_data(self.sync, self.size)


def full_snapshot(source: Any, sync: Sync, size: int):
    """生产 MySQL 使用一致性快照，测试替身保留最小 source 契约。"""
    if isinstance(source, MySQL):
        return MySQLFullSnapshot(source, sync, size)
    return SourceFullSnapshot(source, sync, size)


async def source_count(source: Any, sync: Sync) -> int:
    """读取当前表数量，并确保生产 MySQL 连接显式关闭。"""
    if not isinstance(source, MySQL):
        return int(await source.get_count(sync))
    connection = await asyncmy.connect(**source.kwargs)
    try:
        async with connection.cursor(cursor=DictCursor) as cursor:
            await cursor.execute(f"SELECT COUNT(*) AS count FROM {sync.table}")
            return int((await cursor.fetchone())["count"])
    finally:
        connection.close()
