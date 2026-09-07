"""Redis 暂停、恢复和刷新控制状态测试。"""

from __future__ import annotations

import asyncio
import unittest

from meilisync_plugin.control import RedisControl


class FakeRedis:
    """提供控制器所需的 Redis 命令。"""

    def __init__(self) -> None:
        self.hashes: dict[str, dict[str, str]] = {}
        self.values: dict[str, str] = {}

    async def hgetall(self, key: str) -> dict[str, str]:
        return dict(self.hashes.get(key, {}))

    async def hget(self, key: str, field: str) -> str | None:
        return self.hashes.get(key, {}).get(field)

    async def hset(self, key: str, mapping: dict[str, str]) -> int:
        self.hashes.setdefault(key, {}).update(
            {name: str(value) for name, value in mapping.items()}
        )
        return len(mapping)

    async def hdel(self, key: str, *fields: str) -> int:
        values = self.hashes.setdefault(key, {})
        removed = 0
        for field in fields:
            removed += int(values.pop(field, None) is not None)
        return removed

    async def set(self, key: str, value: str, **kwargs) -> bool | None:
        if kwargs.get("nx") and key in self.values:
            return None
        self.values[key] = value
        return True

    async def eval(self, script: str, numkeys: int, *args) -> int:
        keys = args[:numkeys]
        values = args[numkeys:]
        owner_key = keys[0]
        owner = values[0]
        if "string.sub" in script:
            current = self.values.get(owner_key, "")
            if not current.startswith(owner + "|"):
                return 0
            self.values.pop(owner_key, None)
            return 1
        if self.values.get(owner_key) != owner:
            return 0
        if "KEYS[2], ARGV[1]" in script:
            self.values[keys[1]] = owner + "|" + values[1]
            return 1
        if "HSET" in script:
            progress_key = keys[1]
            mapping = dict(zip(values[1::2], values[2::2], strict=True))
            self.hashes.setdefault(progress_key, {}).update(mapping)
            return 1
        if "EXPIRE" in script:
            return 1
        self.values.pop(owner_key, None)
        return 1

    async def get(self, key: str) -> str | None:
        return self.values.get(key)

    async def exists(self, key: str) -> int:
        return int(key in self.values)

    async def delete(self, key: str) -> int:
        return int(self.values.pop(key, None) is not None)


class RedisControlTest(unittest.TestCase):
    """验证暂停令牌、进程确认与恢复语义。"""

    def test_active_worker_acknowledges_pause_and_resume_clears_it(self) -> None:
        async def scenario() -> tuple[bool, bool]:
            redis = FakeRedis()
            control = RedisControl(redis, "progress", poll_interval=0.001)
            self.assertTrue(await control.acquire_worker("worker-1"))

            pause_task = asyncio.create_task(control.request_pause(timeout=1))
            while not await control.is_pause_requested():
                await asyncio.sleep(0)
            token = await control.pause_token()
            await control.acknowledge_pause(token, "worker-1")
            request = await pause_task
            acknowledged = request.acknowledged

            await control.resume()
            return acknowledged, await control.is_pause_requested()

        acknowledged, paused = asyncio.run(scenario())

        self.assertTrue(acknowledged)
        self.assertFalse(paused)

    def test_pause_without_worker_is_immediately_safe_and_stays_paused(self) -> None:
        async def scenario() -> tuple[bool, bool, bool]:
            control = RedisControl(FakeRedis(), "progress", poll_interval=0.001)
            first = await control.request_pause(timeout=0.1)
            second = await control.request_pause(timeout=0.1)
            return (
                first.already_paused,
                second.already_paused,
                await control.is_pause_requested(),
            )

        first_already, second_already, paused = asyncio.run(scenario())

        self.assertFalse(first_already)
        self.assertTrue(second_already)
        self.assertTrue(paused)

    def test_worker_lease_is_exclusive_and_owner_guarded(self) -> None:
        async def scenario() -> tuple[bool, bool, bool, bool]:
            control = RedisControl(FakeRedis(), "progress")
            first = await control.acquire_worker("worker-1")
            second = await control.acquire_worker("worker-2")
            wrong_release = await control.clear_worker("worker-2")
            owner_release = await control.clear_worker("worker-1")
            return first, second, wrong_release, owner_release

        first, second, wrong_release, owner_release = asyncio.run(scenario())

        self.assertTrue(first)
        self.assertFalse(second)
        self.assertFalse(wrong_release)
        self.assertTrue(owner_release)

    def test_only_worker_lease_owner_can_commit_progress(self) -> None:
        async def scenario() -> tuple[bool, bool, dict[str, str]]:
            redis = FakeRedis()
            control = RedisControl(redis, "progress")
            await control.acquire_worker("worker-1")
            rejected = await control.commit_progress(
                "worker-2", {"master_log_position": 100}
            )
            committed = await control.commit_progress(
                "worker-1", {"master_log_position": 200}
            )
            return rejected, committed, await redis.hgetall("progress")

        rejected, committed, progress = asyncio.run(scenario())

        self.assertFalse(rejected)
        self.assertTrue(committed)
        self.assertEqual("200", progress["master_log_position"])

    def test_refresh_lease_rejects_concurrent_owner(self) -> None:
        async def scenario() -> tuple[bool, bool, bool, bool]:
            control = RedisControl(FakeRedis(), "progress")
            first = await control.acquire_refresh("refresh-1")
            second = await control.acquire_refresh("refresh-2")
            wrong_release = await control.release_refresh("refresh-2")
            owner_release = await control.release_refresh("refresh-1")
            return first, second, wrong_release, owner_release

        first, second, wrong_release, owner_release = asyncio.run(scenario())

        self.assertTrue(first)
        self.assertFalse(second)
        self.assertFalse(wrong_release)
        self.assertTrue(owner_release)

    def test_active_temporary_index_is_separate_from_progress(self) -> None:
        async def scenario() -> tuple[str | None, str | None]:
            redis = FakeRedis()
            control = RedisControl(redis, "progress")
            await control.acquire_refresh("refresh-1")
            await control.set_active_temp("articles__refresh__one", "refresh-1")
            overwritten = await control.set_active_temp(
                "articles__refresh__two", "refresh-2"
            )
            wrong_clear = await control.clear_active_temp("refresh-2")
            current = await control.get_active_temp()
            await control.clear_active_temp("refresh-1")
            return overwritten, wrong_clear, current, await control.get_active_temp()

        overwritten, wrong_clear, current, cleared = asyncio.run(scenario())

        self.assertFalse(overwritten)
        self.assertFalse(wrong_clear)
        self.assertEqual("articles__refresh__one", current)
        self.assertIsNone(cleared)


if __name__ == "__main__":
    unittest.main()
