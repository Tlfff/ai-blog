"""Meilisearch 异步任务完成语义。"""

from __future__ import annotations

from typing import Any

TASK_TIMEOUT_MS = 120_000


async def wait_task(client: Any, task: Any) -> None:
    """等待任务成功；失败、取消、超时均不得提交同步进度或交换结果。"""
    if task is None:
        return
    result = await client.wait_for_task(
        task_id=task.task_uid,
        timeout_in_ms=TASK_TIMEOUT_MS,
        raise_for_status=True,
    )
    if getattr(result, "status", None) != "succeeded":
        raise RuntimeError(
            f"Meilisearch 任务 {task.task_uid} 未成功：{getattr(result, 'status', None)}"
        )
