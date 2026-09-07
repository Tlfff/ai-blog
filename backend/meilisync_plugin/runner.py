"""文章搜索同步入口：增量消费、暂停恢复、全量刷新与一致性检查。"""

from __future__ import annotations

import asyncio
from contextlib import suppress
from dataclasses import dataclass
from pathlib import Path
from typing import Any, AsyncIterator, List, Optional
from uuid import uuid4

import asyncmy
import typer
import yaml
from asyncmy.errors import OperationalError
from asyncmy.replication.row_events import (
    DeleteRowsEvent,
    UpdateRowsEvent,
    WriteRowsEvent,
)
from loguru import logger
from meilisync.discover import get_progress, get_source
from meilisync.enums import EventType
from meilisync.meili import Meili
from meilisync.progress.redis import Redis
from meilisync.schemas import Event, ProgressEvent
from meilisync.settings import Settings
from meilisync.source.mysql import MySQL
from meilisync.version import __VERSION__

from meilisync_plugin.control import RedisControl
from meilisync_plugin.events import RowEvent
from meilisync_plugin.incremental import IncrementalSynchronizer
from meilisync_plugin.operations import CommitStateUncertain, SearchOperations

app = typer.Typer(help="文章搜索 Meilisync 运维入口")


@dataclass
class Runtime:
    """单个 CLI 事件循环内共享的同步依赖。"""

    settings: Settings
    progress: Any
    source_factory: Any
    meili: Meili
    control: RedisControl

    async def close(self) -> None:
        meili_error: Exception | None = None
        try:
            close_meili = getattr(self.meili.client, "aclose", None)
            if close_meili is not None:
                await close_meili()
        except Exception as exc:
            meili_error = exc
        finally:
            close_redis = getattr(self.progress.redis, "aclose", None)
            if close_redis is None:
                close_redis = getattr(self.progress.redis, "close", None)
            if close_redis is not None:
                result = close_redis()
                if asyncio.iscoroutine(result):
                    await result
        if meili_error is not None:
            raise meili_error


async def _iter_all_rows(source: MySQL) -> AsyncIterator[ProgressEvent | Event]:
    """逐行发布每个 Binlog 包，并在暂停取消阻塞读取时关闭连接。"""
    source.conn = await asyncmy.connect(**source.kwargs)
    source.ctl_conn = await asyncmy.connect(**source.kwargs)
    if not source.progress:
        source.progress = await source.get_current_progress()
    yield ProgressEvent(progress=source.progress)
    await source._create_stream()
    try:
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
                    source.progress["master_log_position"] = (
                        source.stream._master_log_position
                    )
                    for position, row in enumerate(rows):
                        yield RowEvent(
                            type=event_type,
                            table=row_event.table,
                            data=row,
                            progress=dict(source.progress),
                            commit_progress=position == len(rows) - 1,
                        )
            except OperationalError as exc:
                logger.exception(f"Binlog stream error: {exc}, sleep 10s and retry...")
                await asyncio.sleep(10)
                try:
                    await source.stream.close()
                    source.ctl_conn.close()
                    await source._create_stream()
                except Exception as reconnect_error:
                    logger.exception(f"Recreate binlog stream error: {reconnect_error}")
    finally:
        stream = getattr(source, "stream", None)
        if stream is not None:
            with suppress(Exception):
                await stream.close()
        for connection_name in ("conn", "ctl_conn"):
            connection = getattr(source, connection_name, None)
            if connection is not None:
                with suppress(Exception):
                    connection.close()


MySQL.__aiter__ = _iter_all_rows


@app.callback()
def callback(
    context: typer.Context,
    config_file: str = typer.Option(
        "config.yml", "-c", "--config", help="Meilisync 配置文件"
    ),
) -> None:
    context.ensure_object(dict)
    context.obj["config_file"] = config_file


@app.command(help="显示 Meilisync 版本")
def version() -> None:
    typer.echo(__VERSION__)


@app.command(help="启动可暂停和恢复的增量同步")
def start(context: typer.Context) -> None:
    async def run() -> None:
        runtime = await _load_runtime(context.obj["config_file"])
        try:
            synchronizer = IncrementalSynchronizer(
                settings=runtime.settings,
                progress=runtime.progress,
                source_factory=runtime.source_factory,
                meili=runtime.meili,
                control=runtime.control,
            )
            logger.info(
                "启动 Meilisync 增量同步；成功写入 Meilisearch 后提交 Redis 进度"
            )
            await synchronizer.run()
        finally:
            await runtime.close()

    asyncio.run(run())


@app.command(help="请求增量进程暂停并等待已接收事件刷盘")
def pause(
    context: typer.Context, timeout: float = typer.Option(30, help="等待进程确认的秒数")
) -> None:
    async def run() -> None:
        runtime = await _load_runtime(context.obj["config_file"])
        try:
            request = await runtime.control.request_pause(timeout=timeout)
            state = "已由增量进程确认" if request.acknowledged else "当前无活跃增量进程"
            typer.echo(f"增量同步已暂停：{state}，token={request.token}")
        finally:
            await runtime.close()

    asyncio.run(run())


@app.command(help="恢复增量同步；进程从 Redis 保存的 Binlog 进度继续")
def resume(context: typer.Context) -> None:
    async def run() -> None:
        runtime = await _load_runtime(context.obj["config_file"])
        owner = uuid4().hex
        acquired = False
        try:
            acquired = await runtime.control.acquire_refresh(owner)
            if not acquired:
                raise RuntimeError("搜索全量刷新正在执行，不能提前恢复增量同步")
            await runtime.control.resume()
            typer.echo("增量同步已恢复；活跃或下次启动的进程将从 Redis 进度继续")
        finally:
            try:
                if acquired:
                    await runtime.control.release_refresh(owner)
            finally:
                await runtime.close()

    asyncio.run(run())


@app.command(help="使用临时索引全量导入并通过 Index Swap 原子刷新")
def refresh(
    context: typer.Context,
    table: Optional[List[str]] = typer.Option(
        None, "-t", "--table", help="表名；一次刷新仅支持一张配置表"
    ),
    size: int = typer.Option(10_000, "-s", "--size", help="每批导入文档数"),
) -> None:
    async def run() -> None:
        runtime = await _load_runtime(context.obj["config_file"])
        pause_request = None
        refresh_owner = uuid4().hex
        refresh_acquired = False
        try:
            selected = _selected_syncs(runtime.settings, table)
            if len(selected) != 1:
                raise typer.BadParameter(
                    "一次全量刷新仅支持一张配置表", param_hint="--table"
                )
            refresh_acquired = await runtime.control.acquire_refresh(refresh_owner)
            if not refresh_acquired:
                raise RuntimeError("已有搜索全量刷新持有 Redis refresh 租约")
            pause_request = await runtime.control.request_pause(timeout=30)
            results = await _refresh_selected(runtime, selected, size, refresh_owner)
            for result in results:
                typer.echo(
                    f"刷新完成 table={result.table} index={result.index} "
                    f"mysql={result.mysql_count} meilisearch={result.index_count}"
                )
        finally:
            try:
                try:
                    if pause_request is not None and not pause_request.already_paused:
                        await runtime.control.resume()
                finally:
                    if refresh_acquired:
                        await runtime.control.release_refresh(refresh_owner)
            finally:
                await runtime.close()

    asyncio.run(run())


@app.command(help="核对 MySQL 与 Meilisearch 文档数量并清理旧临时索引")
def check(
    context: typer.Context,
    table: Optional[List[str]] = typer.Option(
        None, "-t", "--table", help="表名；不传则检查全部配置表"
    ),
) -> None:
    async def run() -> None:
        runtime = await _load_runtime(context.obj["config_file"])
        owner = uuid4().hex
        acquired = False
        try:
            acquired = await runtime.control.acquire_refresh(owner)
            if not acquired:
                raise RuntimeError("搜索全量刷新正在执行，不能并发检查或清理临时索引")
            source = runtime.source_factory(await runtime.progress.get())
            operations = SearchOperations(
                source,
                runtime.meili,
                control=runtime.control,
                refresh_owner=owner,
            )
            for sync in _selected_syncs(runtime.settings, table):
                result = await operations.check(sync)
                typer.echo(
                    f"检查通过 table={result.table} index={result.index} "
                    f"mysql={result.mysql_count} meilisearch={result.index_count} "
                    f"cleaned={len(result.removed_indexes)}"
                )
        finally:
            try:
                if acquired:
                    await runtime.control.release_refresh(owner)
            finally:
                await runtime.close()

    asyncio.run(run())


async def _refresh_selected(
    runtime: Runtime,
    selected: list[Any],
    size: int,
    refresh_owner: str | None = None,
) -> list[Any]:
    """以候选位点刷新；Swap 后提交成功才允许清理旧索引。"""
    if len(selected) != 1:
        raise ValueError("一次全量刷新仅支持一个表")
    previous_progress = await runtime.progress.get()
    source = runtime.source_factory(previous_progress)
    current_progress = await source.get_current_progress()
    logger.info(f"全量刷新候选 Binlog 进度：{current_progress}")
    operations = SearchOperations(
        source,
        runtime.meili,
        control=runtime.control,
        refresh_owner=refresh_owner,
    )

    async def commit_progress() -> None:
        try:
            await runtime.progress.set(**current_progress)
        except Exception as commit_error:
            try:
                actual = await runtime.progress.get()
            except Exception as verify_error:
                raise CommitStateUncertain(
                    "无法确认 Redis Binlog 进度是否已提交"
                ) from verify_error
            expected = {name: str(value) for name, value in current_progress.items()}
            actual = {name: str(value) for name, value in actual.items()}
            if actual != expected:
                raise commit_error
            logger.warning("Redis 位点提交响应失败，但回读确认候选进度已生效")
        logger.info(f"Index Swap 成功，已提交 Redis Binlog 进度：{current_progress}")

    return [
        await operations.refresh(sync, size=size, after_swap=commit_progress)
        for sync in selected
    ]


async def _load_runtime(config_file: str) -> Runtime:
    """在当前事件循环中创建配置、Redis 进度、source 与 Meilisearch 客户端。"""
    raw = yaml.safe_load(Path(config_file).read_text(encoding="utf-8"))
    settings = Settings.model_validate(raw)
    progress_kwargs = settings.progress.model_dump(exclude={"type"})
    progress = get_progress(settings.progress.type)(**progress_kwargs)
    if not isinstance(progress, Redis):
        raise RuntimeError("暂停、恢复和续传要求 progress.type=redis")

    source_cls = get_source(settings.source.type)
    source_kwargs = settings.source.model_dump(exclude={"type"})

    def source_factory(current_progress: dict) -> Any:
        return source_cls(
            progress=dict(current_progress), tables=settings.tables, **source_kwargs
        )

    meili_settings = settings.meilisearch
    meili = Meili(
        meili_settings.api_url, meili_settings.api_key, settings.plugins_cls()
    )
    control = RedisControl(progress.redis, progress_kwargs["key"])
    return Runtime(
        settings=settings,
        progress=progress,
        source_factory=source_factory,
        meili=meili,
        control=control,
    )


def _selected_syncs(settings: Settings, tables: Optional[List[str]]) -> list[Any]:
    selected = [sync for sync in settings.sync if not tables or sync.table in tables]
    unknown = sorted(set(tables or []) - {sync.table for sync in settings.sync})
    if unknown:
        raise typer.BadParameter(
            f"配置中不存在表：{', '.join(unknown)}", param_hint="--table"
        )
    return selected


if __name__ == "__main__":
    app()
