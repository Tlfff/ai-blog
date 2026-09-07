"""使用 Redis 协调增量同步暂停、恢复与全量刷新。"""

from __future__ import annotations

import asyncio
from dataclasses import dataclass
from uuid import uuid4

PAUSE_STATE = "paused"
WORKER_TTL_SECONDS = 5
REFRESH_LEASE_SECONDS = 300
_RENEW_WORKER = """
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('EXPIRE', KEYS[1], ARGV[2])
end
return 0
"""
_RELEASE_WORKER = """
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
"""
_COMMIT_PROGRESS = """
if redis.call('GET', KEYS[1]) ~= ARGV[1] then
  return 0
end
for i = 2, #ARGV, 2 do
  redis.call('HSET', KEYS[2], ARGV[i], ARGV[i + 1])
end
return 1
"""
_SET_ACTIVE_REFRESH = """
if redis.call('GET', KEYS[1]) ~= ARGV[1] then
  return 0
end
redis.call('SET', KEYS[2], ARGV[1] .. '|' .. ARGV[2], 'EX', ARGV[3])
return 1
"""
_CLEAR_ACTIVE_REFRESH = """
local current = redis.call('GET', KEYS[1])
local prefix = ARGV[1] .. '|'
if current and string.sub(current, 1, string.len(prefix)) == prefix then
  return redis.call('DEL', KEYS[1])
end
return 0
"""


@dataclass(frozen=True)
class PauseRequest:
    """一次暂停请求及增量进程确认结果。"""

    token: str
    already_paused: bool
    acknowledged: bool


class RedisControl:
    """把控制状态放在独立 Redis Key，避免污染 Binlog 进度。"""

    def __init__(
        self, redis_client, progress_key: str, *, poll_interval: float = 0.2
    ) -> None:
        self.redis = redis_client
        self.progress_key = progress_key
        self.control_key = f"{progress_key}:control"
        self.worker_key = f"{progress_key}:worker"
        self.active_temp_key = f"{progress_key}:refresh"
        self.refresh_lock_key = f"{progress_key}:refresh-lock"
        self.poll_interval = poll_interval

    async def request_pause(self, *, timeout: float = 30) -> PauseRequest:
        """请求暂停；活跃进程必须在刷完已接收事件后确认同一令牌。"""
        state = await self.redis.hgetall(self.control_key)
        already_paused = state.get("state") == PAUSE_STATE and bool(state.get("token"))
        token = state.get("token") if already_paused else uuid4().hex
        if not already_paused:
            await self.redis.hset(
                self.control_key,
                mapping={
                    "state": PAUSE_STATE,
                    "token": token,
                    "ack_token": "",
                    "ack_worker": "",
                },
            )

        if not await self.worker_alive():
            return PauseRequest(
                token=token, already_paused=already_paused, acknowledged=False
            )

        deadline = asyncio.get_running_loop().time() + timeout
        while asyncio.get_running_loop().time() < deadline:
            if await self.redis.hget(self.control_key, "ack_token") == token:
                return PauseRequest(
                    token=token, already_paused=already_paused, acknowledged=True
                )
            if not await self.worker_alive():
                return PauseRequest(
                    token=token, already_paused=already_paused, acknowledged=False
                )
            await asyncio.sleep(self.poll_interval)
        raise TimeoutError("等待 Meilisync 增量进程确认暂停超时")

    async def resume(self) -> None:
        """清除暂停与确认令牌，使当前或下次启动的进程继续消费 Redis 进度。"""
        await self.redis.hdel(
            self.control_key, "state", "token", "ack_token", "ack_worker"
        )

    async def is_pause_requested(self) -> bool:
        return await self.redis.hget(self.control_key, "state") == PAUSE_STATE

    async def pause_token(self) -> str:
        return (await self.redis.hget(self.control_key, "token")) or ""

    async def acknowledge_pause(self, token: str, worker_id: str) -> bool:
        """仅确认当前暂停令牌，避免旧进程覆盖新请求。"""
        if (
            not token
            or token != await self.pause_token()
            or not await self.is_pause_requested()
        ):
            return False
        await self.redis.hset(
            self.control_key, mapping={"ack_token": token, "ack_worker": worker_id}
        )
        return True

    async def wait_until_resumed(self) -> None:
        while await self.is_pause_requested():
            await asyncio.sleep(self.poll_interval)

    async def acquire_worker(self, worker_id: str) -> bool:
        """以带 TTL 的所有权租约保证同一进度 Key 只有一个增量进程。"""
        return bool(
            await self.redis.set(
                self.worker_key,
                worker_id,
                ex=WORKER_TTL_SECONDS,
                nx=True,
            )
        )

    async def heartbeat(self, worker_id: str) -> bool:
        """仅租约所有者可以续期。"""
        return bool(
            await self.redis.eval(
                _RENEW_WORKER,
                1,
                self.worker_key,
                worker_id,
                WORKER_TTL_SECONDS,
            )
        )

    async def commit_progress(self, worker_id: str, progress: dict) -> bool:
        """仅当前 worker 租约所有者可原子提交 Binlog 进度。"""
        arguments = [worker_id]
        for field, value in progress.items():
            arguments.extend((str(field), str(value)))
        return bool(
            await self.redis.eval(
                _COMMIT_PROGRESS,
                2,
                self.worker_key,
                self.progress_key,
                *arguments,
            )
        )

    async def worker_alive(self) -> bool:
        return bool(await self.redis.exists(self.worker_key))

    async def clear_worker(self, worker_id: str) -> bool:
        """仅租约所有者可以释放，避免旧进程删除新进程心跳。"""
        return bool(
            await self.redis.eval(
                _RELEASE_WORKER,
                1,
                self.worker_key,
                worker_id,
            )
        )

    async def acquire_refresh(self, owner: str) -> bool:
        """以所有权租约拒绝并发全量刷新。"""
        return bool(
            await self.redis.set(
                self.refresh_lock_key,
                owner,
                ex=REFRESH_LEASE_SECONDS,
                nx=True,
            )
        )

    async def renew_refresh(self, owner: str) -> bool:
        return bool(
            await self.redis.eval(
                _RENEW_WORKER,
                1,
                self.refresh_lock_key,
                owner,
                REFRESH_LEASE_SECONDS,
            )
        )

    async def release_refresh(self, owner: str) -> bool:
        return bool(
            await self.redis.eval(
                _RELEASE_WORKER,
                1,
                self.refresh_lock_key,
                owner,
            )
        )

    async def set_active_temp(self, index: str, owner: str) -> bool:
        """仅 refresh 租约所有者可设置或续期活动临时索引。"""
        return bool(
            await self.redis.eval(
                _SET_ACTIVE_REFRESH,
                2,
                self.refresh_lock_key,
                self.active_temp_key,
                owner,
                index,
                REFRESH_LEASE_SECONDS,
            )
        )

    async def clear_active_temp(self, owner: str) -> bool:
        """仅写入该标记的 refresh owner 可清除。"""
        return bool(
            await self.redis.eval(
                _CLEAR_ACTIVE_REFRESH,
                1,
                self.active_temp_key,
                owner,
            )
        )

    async def get_active_temp(self) -> str | None:
        current = await self.redis.get(self.active_temp_key)
        if not current:
            return None
        _owner, separator, index = current.partition("|")
        return index if separator else current
