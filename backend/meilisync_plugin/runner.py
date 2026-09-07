"""修复 Meilisync 0.1.3 Redis 跨循环连接与多行 ROW 事件丢失问题。"""

from __future__ import annotations

import asyncio
from typing import Any, AsyncIterator

import asyncmy
import redis.asyncio as redis
from asyncmy.errors import OperationalError
from asyncmy.replication.row_events import DeleteRowsEvent, UpdateRowsEvent, WriteRowsEvent
from loguru import logger
from meilisync.enums import EventType
from meilisync.main import app
from meilisync.progress.redis import Redis
from meilisync.schemas import Event, ProgressEvent
from meilisync.source.mysql import MySQL

_original_get = Redis.get


async def _get_and_reset_connection(progress: Redis) -> dict[str, Any]:
    """读取启动进度后重建惰性客户端，避免连接跨 Typer 的两个事件循环复用。"""
    current = await _original_get(progress)
    close = getattr(progress.redis, "aclose", None)
    if close is None:
        close = progress.redis.close
    await close()
    progress.redis = redis.from_url(progress.kwargs["dsn"], decode_responses=True)
    return current


async def _iter_all_rows(source: MySQL) -> AsyncIterator[ProgressEvent | Event]:
    """逐行发布每个 Binlog 包，避免 Meilisync 0.1.3 只同步 rows[0]。"""
    source.conn = await asyncmy.connect(**source.kwargs)
    source.ctl_conn = await asyncmy.connect(**source.kwargs)
    if not source.progress:
        source.progress = await source.get_current_progress()
    yield ProgressEvent(progress=source.progress)
    await source._create_stream()
    while True:
        try:
            async for row_event in source.stream:
                if isinstance(row_event, WriteRowsEvent):
                    event_type = EventType.create
                    rows = [row["values"] for row in row_event.rows]
                elif isinstance(row_event, UpdateRowsEvent):
                    event_type = EventType.update
                    rows = [row["after_values"] for row in row_event.rows]
                elif isinstance(row_event, DeleteRowsEvent):
                    event_type = EventType.delete
                    rows = [row["values"] for row in row_event.rows]
                else:
                    continue
                source.progress["master_log_file"] = source.stream._master_log_file
                source.progress["master_log_position"] = source.stream._master_log_position
                for row in rows:
                    yield Event(type=event_type, table=row_event.table, data=row, progress=source.progress)
        except OperationalError as exc:
            logger.exception(f"Binlog stream error: {exc}, sleep 10s and retry...")
            await asyncio.sleep(10)
            try:
                await source.stream.close()
                source.ctl_conn.close()
                await source._create_stream()
            except Exception as reconnect_error:
                logger.exception(f"Recreate binlog stream error: {reconnect_error}")


Redis.get = _get_and_reset_connection
MySQL.__aiter__ = _iter_all_rows


if __name__ == "__main__":
    app()
