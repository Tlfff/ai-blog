"""Meilisync 事件扩展。"""

from __future__ import annotations

from meilisync.schemas import Event


class RowEvent(Event):
    """标记一行是否是当前 MySQL ROW 包的最后一行。"""

    commit_progress: bool = True
