"""文章搜索索引的全量刷新、一致性检查与旧索引清理。"""

from __future__ import annotations

from dataclasses import dataclass
from collections.abc import Awaitable
from typing import Any, Callable
from uuid import uuid4

from loguru import logger
from meilisync.settings import Sync

from meilisync_plugin.index_settings import article_settings
from meilisync_plugin.snapshot import full_snapshot, source_count
from meilisync_plugin.tasks import TASK_TIMEOUT_MS, wait_task

TEMP_MARKER = "__refresh__"


class CommitStateUncertain(RuntimeError):
    """Redis 位点提交结果无法确认；保留新旧索引供安全恢复。"""


class ConsistencyError(RuntimeError):
    """MySQL、索引数据或正式索引设置不一致。"""

    def __init__(
        self,
        message: str,
        *,
        mysql_count: int | None = None,
        index_count: int | None = None,
    ) -> None:
        super().__init__(message)
        self.mysql_count = mysql_count
        self.index_count = index_count


@dataclass(frozen=True)
class CheckResult:
    """一次数量一致性检查结果。"""

    table: str
    index: str
    mysql_count: int
    index_count: int
    removed_indexes: tuple[str, ...] = ()


@dataclass(frozen=True)
class RefreshResult(CheckResult):
    """一次成功全量刷新的结果。"""

    temporary_index: str = ""


class _NullControl:
    """未配置 Redis 控制器时提供活动索引空实现。"""

    async def set_active_temp(self, _index: str, _owner: str) -> bool:
        return True

    async def clear_active_temp(self, _owner: str) -> bool:
        return True

    async def get_active_temp(self) -> str | None:
        return None


class SearchOperations:
    """在稳定索引名不变的前提下编排临时索引刷新。"""

    def __init__(
        self,
        source: Any,
        meili: Any,
        *,
        control: Any | None = None,
        temp_token: Callable[[], str] | None = None,
        refresh_owner: str | None = None,
    ) -> None:
        self.source = source
        self.meili = meili
        self.client = meili.client
        self.control = control or _NullControl()
        self.temp_token = temp_token or (lambda: uuid4().hex)
        self.refresh_owner = refresh_owner

    async def refresh(
        self,
        sync: Sync,
        *,
        size: int,
        after_swap: Callable[[], Awaitable[None]] | None = None,
    ) -> RefreshResult:
        """完整导入临时索引，校验后原子交换，失败则保持或恢复正式索引。"""
        if size <= 0:
            raise ValueError("刷新批量大小必须大于 0")
        stable = sync.index_name
        expected_settings, settings_model = await self._validated_live_settings(sync)
        removed_before = await self.cleanup_temporary_indexes(stable)

        temporary = f"{stable}{TEMP_MARKER}{self.temp_token()}"
        await self._set_active_temp(temporary)
        await self._renew_refresh_lease()
        swapped = False
        rolled_back = False
        preserve_for_recovery = False
        try:
            await self.client.create_index(
                temporary,
                primary_key=sync.pk,
                settings=settings_model,
                timeout_in_ms=TASK_TIMEOUT_MS,
            )
            temporary_sync = sync.model_copy(update={"index": temporary})
            imported_count = 0
            async with full_snapshot(self.source, sync, size) as snapshot:
                mysql_count = snapshot.count
                async for items in snapshot.batches():
                    await self._renew_refresh_lease()
                    await self._set_active_temp(temporary)
                    task = await self.meili.add_data(temporary_sync, items)
                    await wait_task(self.client, task)
                    imported_count += len(items)

            temporary_count = await self._index_count(temporary)
            if imported_count != mysql_count or temporary_count != mysql_count:
                raise ConsistencyError(
                    f'临时索引 "{temporary}" 与 MySQL 表 "{sync.table}" 数量不一致：'
                    f"导入 {imported_count}，MySQL {mysql_count}，索引 {temporary_count}",
                    mysql_count=mysql_count,
                    index_count=temporary_count,
                )
            await self._assert_settings(temporary, expected_settings)
            await self._renew_refresh_lease()

            await self._swap(stable, temporary)
            swapped = True

            stable_count = await self._index_count(stable)
            if stable_count != mysql_count:
                raise ConsistencyError(
                    f'交换后索引 "{stable}" 与 MySQL 快照数量不一致，将回滚：'
                    f"MySQL 快照 {mysql_count}，索引 {stable_count}",
                    mysql_count=mysql_count,
                    index_count=stable_count,
                )
            await self._assert_settings(stable, expected_settings)
            await self._renew_refresh_lease()
            if after_swap is not None:
                await after_swap()

            try:
                await self._delete_index(temporary)
            except Exception as cleanup_error:
                logger.warning(
                    f'旧索引 "{temporary}" 清理失败，保留并交由 check 后续清理：{cleanup_error}'
                )
            logger.success(
                f'索引 "{stable}" 全量刷新完成：MySQL 快照 {mysql_count}，索引 {stable_count}，临时索引 {temporary}'
            )
            return RefreshResult(
                table=sync.table,
                index=stable,
                mysql_count=mysql_count,
                index_count=stable_count,
                removed_indexes=tuple(removed_before),
                temporary_index=temporary,
            )
        except CommitStateUncertain:
            preserve_for_recovery = True
            logger.error(
                f'索引 "{stable}" 的 Redis 位点提交结果不确定；保留稳定索引和旧索引 "{temporary}" 供恢复'
            )
            raise
        except Exception:
            if swapped and not rolled_back:
                try:
                    await self._swap(stable, temporary)
                    rolled_back = True
                except Exception as rollback_error:
                    raise RuntimeError(
                        f'索引 "{stable}" 刷新失败且回滚失败；临时索引 "{temporary}" 已保留以便人工恢复'
                    ) from rollback_error
            if not swapped or rolled_back:
                await self._delete_index_if_exists(temporary)
            raise
        finally:
            if not preserve_for_recovery:
                await self._clear_active_temp()

    async def check(self, sync: Sync) -> CheckResult:
        """核对 MySQL 全表数量和正式索引文档数；成功后清理遗留临时索引。"""
        mysql_count = await source_count(self.source, sync)
        index_count = await self._index_count(sync.index_name)
        if mysql_count != index_count:
            raise ConsistencyError(
                f'索引 "{sync.index_name}" 与 MySQL 表 "{sync.table}" 数量不一致：'
                f"MySQL {mysql_count}，索引 {index_count}",
                mysql_count=mysql_count,
                index_count=index_count,
            )
        removed = await self.cleanup_temporary_indexes(sync.index_name)
        logger.info(
            f'索引 "{sync.index_name}" 与 MySQL 表 "{sync.table}" 一致：数量 {mysql_count}，清理临时索引 {len(removed)} 个'
        )
        return CheckResult(
            table=sync.table,
            index=sync.index_name,
            mysql_count=mysql_count,
            index_count=index_count,
            removed_indexes=tuple(removed),
        )

    async def cleanup_temporary_indexes(self, stable: str) -> list[str]:
        """删除命名规则匹配且不是当前刷新使用中的旧临时索引。"""
        await self._renew_refresh_lease()
        active = await self.control.get_active_temp()
        prefix = f"{stable}{TEMP_MARKER}"
        indexes = await self.client.get_indexes(limit=1000) or []
        removed: list[str] = []
        for index in indexes:
            uid = index.uid
            if uid == active or not uid.startswith(prefix):
                continue
            await self._renew_refresh_lease()
            await self._delete_index(uid)
            removed.append(uid)
        return removed

    async def _set_active_temp(self, temporary: str) -> None:
        if self.refresh_owner is None:
            await self.control.set_active_temp(temporary, "local")
            return
        if not await self.control.set_active_temp(temporary, self.refresh_owner):
            raise RuntimeError("搜索全量刷新无权设置活动临时索引")

    async def _clear_active_temp(self) -> None:
        owner = self.refresh_owner or "local"
        await self.control.clear_active_temp(owner)

    async def _renew_refresh_lease(self) -> None:
        if self.refresh_owner is None:
            return
        if not await self.control.renew_refresh(self.refresh_owner):
            raise RuntimeError("搜索全量刷新已失去 Redis refresh 租约")

    async def _validated_live_settings(self, sync: Sync) -> tuple[dict[str, Any], Any]:
        expected = article_settings() if sync.table == "articles" else None
        live = await self.client.index(sync.index_name).get_settings()
        actual = live.model_dump(by_alias=True, exclude_none=True)
        if expected is None:
            expected = actual
        else:
            self._compare_settings(sync.index_name, expected, actual)
        return expected, type(live).model_validate(expected)

    async def _assert_settings(self, index: str, expected: dict[str, Any]) -> None:
        actual_model = await self.client.index(index).get_settings()
        actual = actual_model.model_dump(by_alias=True, exclude_none=True)
        self._compare_settings(index, expected, actual)

    @staticmethod
    def _compare_settings(
        index: str, expected: dict[str, Any], actual: dict[str, Any]
    ) -> None:
        mismatched = [
            key for key, value in expected.items() if actual.get(key) != value
        ]
        if mismatched:
            raise ConsistencyError(
                f'索引 "{index}" 的正式索引设置不一致：{", ".join(mismatched)}'
            )

    async def _index_count(self, index: str) -> int:
        stats = await self.client.index(index).get_stats()
        return int(stats.number_of_documents)

    async def _swap(self, stable: str, temporary: str) -> None:
        task = await self.client.swap_indexes([(stable, temporary)])
        await wait_task(self.client, task)

    async def _delete_index(self, index: str) -> None:
        task = await self.client.index(index).delete()
        await wait_task(self.client, task)

    async def _delete_index_if_exists(self, index: str) -> None:
        try:
            await self._delete_index(index)
        except Exception as exc:
            if self._is_index_not_found(exc):
                return
            logger.warning(f'清理临时索引 "{index}" 失败：{exc}')

    @staticmethod
    def _is_index_not_found(exc: Exception) -> bool:
        code = str(getattr(exc, "code", ""))
        return code.endswith("index_not_found") or "index_not_found" in str(exc)
