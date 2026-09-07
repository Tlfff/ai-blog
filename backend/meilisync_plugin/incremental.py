"""支持 Redis 协调暂停和可靠进度提交的 Meilisync 增量循环。"""

from __future__ import annotations

import asyncio
from contextlib import suppress
from time import monotonic
from typing import Any, Callable
from uuid import uuid4

from loguru import logger
from meilisync.enums import EventType
from meilisync.event import EventCollection
from meilisync.schemas import Event

from meilisync_plugin.events import RowEvent
from meilisync_plugin.tasks import wait_task


class IncrementalSynchronizer:
    """仅在 Meilisearch 任务成功后提交 Redis Binlog 进度。"""

    def __init__(
        self,
        *,
        settings: Any,
        progress: Any,
        source_factory: Callable[[dict], Any],
        meili: Any,
        control: Any,
        poll_interval: float = 0.2,
    ) -> None:
        self.settings = settings
        self.progress = progress
        self.source_factory = source_factory
        self.meili = meili
        self.control = control
        self.poll_interval = poll_interval
        self.worker_id = uuid4().hex
        self.lease_lost = asyncio.Event()

    async def run(self) -> None:
        """持续消费；暂停时取消阻塞读取、刷盘、确认，恢复后按 Redis 位点重连。"""
        if not await self.control.acquire_worker(self.worker_id):
            raise RuntimeError("已有 Meilisync 增量进程持有 Redis worker 租约")
        heartbeat = asyncio.create_task(self._heartbeat_loop())
        try:
            while True:
                self._raise_if_lease_lost()
                if await self.control.is_pause_requested():
                    await self.control.acknowledge_pause(
                        await self.control.pause_token(), self.worker_id
                    )
                    await self.control.wait_until_resumed()

                current_progress = await self.progress.get()
                source = self.source_factory(current_progress)
                pause_token = await self._consume_until_pause(source, current_progress)
                if pause_token:
                    await self.control.acknowledge_pause(pause_token, self.worker_id)
                    logger.info(
                        f"Meilisync 增量同步已暂停，Redis 进度：{current_progress}"
                    )
                    await self.control.wait_until_resumed()
                    logger.info("Meilisync 增量同步已恢复，将从 Redis 进度重新连接")
        finally:
            heartbeat.cancel()
            with suppress(asyncio.CancelledError):
                await heartbeat
            await self.control.clear_worker(self.worker_id)

    async def _heartbeat_loop(self) -> None:
        while True:
            await asyncio.sleep(1)
            try:
                renewed = await self.control.heartbeat(self.worker_id)
            except Exception:
                self.lease_lost.set()
                return
            if not renewed:
                self.lease_lost.set()
                return

    def _raise_if_lease_lost(self) -> None:
        if self.lease_lost.is_set():
            raise RuntimeError("Meilisync 增量进程已失去 Redis worker 租约")

    async def _consume_until_pause(self, source: Any, current_progress: dict) -> str:
        collection = EventCollection()
        pending_progress = dict(current_progress)
        iterator = source.__aiter__()
        next_event: asyncio.Task | None = None
        interval = self.settings.meilisearch.insert_interval
        last_flush = monotonic()
        try:
            while True:
                self._raise_if_lease_lost()
                if await self.control.is_pause_requested():
                    await self._flush(collection, pending_progress)
                    current_progress.clear()
                    current_progress.update(pending_progress)
                    return await self.control.pause_token()

                next_event = asyncio.create_task(iterator.__anext__())
                while not next_event.done():
                    done, _ = await asyncio.wait(
                        {next_event}, timeout=self.poll_interval
                    )
                    if done:
                        break
                    self._raise_if_lease_lost()
                    if await self.control.is_pause_requested():
                        next_event.cancel()
                        with suppress(asyncio.CancelledError, StopAsyncIteration):
                            await next_event
                        next_event = None
                        await self._flush(collection, pending_progress)
                        current_progress.clear()
                        current_progress.update(pending_progress)
                        return await self.control.pause_token()
                    if (
                        interval
                        and collection.size
                        and monotonic() - last_flush >= interval
                    ):
                        await self._flush(collection, pending_progress)
                        current_progress.clear()
                        current_progress.update(pending_progress)
                        last_flush = monotonic()

                try:
                    event = next_event.result()
                except StopAsyncIteration:
                    await self._flush(collection, pending_progress)
                    current_progress.clear()
                    current_progress.update(pending_progress)
                    return ""
                finally:
                    next_event = None

                if event.progress and (
                    not isinstance(event, RowEvent) or event.commit_progress
                ):
                    pending_progress = dict(event.progress)
                if not isinstance(event, Event):
                    await self._commit_progress(pending_progress)
                    current_progress.clear()
                    current_progress.update(pending_progress)
                    continue

                sync = self.settings.get_sync(event.table)
                if sync is None:
                    continue
                collection.add_event(sync, event)
                insert_size = self.settings.meilisearch.insert_size
                if not insert_size or collection.size >= insert_size:
                    await self._flush(collection, pending_progress)
                    current_progress.clear()
                    current_progress.update(pending_progress)
                    last_flush = monotonic()
        finally:
            if next_event is not None:
                next_event.cancel()
                with suppress(asyncio.CancelledError, StopAsyncIteration):
                    await next_event
            close = getattr(iterator, "aclose", None)
            if close is not None:
                with suppress(Exception):
                    await close()

    async def _flush(self, collection: EventCollection, progress_value: dict) -> None:
        if collection.size == 0:
            return
        created, updated, deleted = collection.pop_events
        tasks = []
        for event_type, grouped in (
            (EventType.create, created),
            (EventType.update, updated),
            (EventType.delete, deleted),
        ):
            for sync, events in grouped.items():
                task = await self.meili.handle_events_by_type(sync, events, event_type)
                if task is not None:
                    tasks.append(task)
        for task in tasks:
            await wait_task(self.meili.client, task)
        await self._commit_progress(progress_value)

    async def _commit_progress(self, progress_value: dict) -> None:
        if not await self.control.commit_progress(self.worker_id, progress_value):
            self.lease_lost.set()
            raise RuntimeError(
                "Meilisync 增量进程失去 worker 租约，拒绝提交 Redis 进度"
            )
