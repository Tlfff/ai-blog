from __future__ import annotations

import json
import queue
import subprocess
import threading
import time
from pathlib import Path
from typing import Any, Callable

NotificationCallback = Callable[[dict[str, Any]], None]


class AppServerError(RuntimeError):
    pass


class AppServerClient:
    """Minimal JSON-RPC client for the local Codex App Server v2 protocol."""

    def __init__(
        self,
        notification_callback: NotificationCallback | None = None,
        process_env: dict[str, str] | None = None,
    ):
        self.notification_callback = notification_callback
        self.process_env = process_env
        self.process: subprocess.Popen[str] | None = None
        self._events: queue.Queue[dict[str, Any]] = queue.Queue()
        self._responses: dict[str, queue.Queue[dict[str, Any]]] = {}
        self._lock = threading.RLock()
        self._write_lock = threading.Lock()
        self._next_id = 0
        self._reader_thread: threading.Thread | None = None
        self._closed = False
        self.thread_info: dict[str, Any] = {}

    def start(self) -> None:
        with self._lock:
            if self.process and self.process.poll() is None:
                return
            self._closed = False
            self.process = subprocess.Popen(
                ["codex", "app-server", "--listen", "stdio://"],
                env=self.process_env,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.STDOUT,
                text=True,
                bufsize=1,
            )
            self._reader_thread = threading.Thread(
                target=self._read_loop,
                name="codex-app-server-reader",
                daemon=True,
            )
            self._reader_thread.start()

        self.request(
            "initialize",
            {
                "clientInfo": {
                    "name": "ai-blog-orchestrator",
                    "title": "AI Blog Orchestrator",
                    "version": "0.1.0",
                },
                "capabilities": {"experimentalApi": True},
            },
        )
        self._send_notification("initialized", {})

    def _read_loop(self) -> None:
        process = self.process
        if not process or not process.stdout:
            return
        for raw_line in process.stdout:
            line = raw_line.strip()
            if not line:
                continue
            try:
                message = json.loads(line)
            except json.JSONDecodeError:
                # Keep non-JSON diagnostics visible to the caller as an event.
                self._events.put({"method": "app-server/output", "params": {"text": line}})
                continue
            if "method" in message:
                if message.get("method") == "model/rerouted":
                    params = message.get("params") or {}
                    if params.get("toModel"):
                        self.thread_info["model"] = params["toModel"]
                self._events.put(message)
                continue
            request_id = str(message.get("id"))
            with self._lock:
                response_queue = self._responses.get(request_id)
            if response_queue:
                response_queue.put(message)
            else:
                self._events.put(message)

    def _send(self, message: dict[str, Any]) -> None:
        with self._write_lock:
            if not self.process or not self.process.stdin or self.process.poll() is not None:
                raise AppServerError("Codex App Server 未运行。")
            self.process.stdin.write(json.dumps(message, ensure_ascii=False) + "\n")
            self.process.stdin.flush()

    def _send_notification(self, method: str, params: dict[str, Any]) -> None:
        self._send({"jsonrpc": "2.0", "method": method, "params": params})

    def request(self, method: str, params: dict[str, Any]) -> dict[str, Any]:
        with self._lock:
            self._next_id += 1
            request_id = str(self._next_id)
            response_queue: queue.Queue[dict[str, Any]] = queue.Queue(maxsize=1)
            self._responses[request_id] = response_queue
        try:
            self._send({"jsonrpc": "2.0", "id": request_id, "method": method, "params": params})
            deadline = time.monotonic() + 30
            while True:
                remaining = deadline - time.monotonic()
                if remaining <= 0:
                    raise AppServerError(f"App Server 请求超时：{method}")
                try:
                    response = response_queue.get(timeout=remaining)
                except queue.Empty as exc:
                    raise AppServerError(f"App Server 请求超时：{method}") from exc
                self._dispatch_event_queue()
                if "error" in response:
                    raise AppServerError(f"{method} 失败：{response['error']}")
                return response.get("result") or {}
        finally:
            with self._lock:
                self._responses.pop(request_id, None)

    def _dispatch_event_queue(self) -> None:
        while True:
            try:
                message = self._events.get_nowait()
            except queue.Empty:
                return
            if self.notification_callback:
                self.notification_callback(message)

    def _wait_for_turn(self, thread_id: str, turn_id: str) -> int:
        while True:
            if self.process and self.process.poll() is not None:
                raise AppServerError("Codex App Server 在 turn 执行期间退出。")
            try:
                message = self._events.get(timeout=1)
            except queue.Empty:
                continue
            if self.notification_callback:
                self.notification_callback(message)
            if message.get("method") != "turn/completed":
                continue
            params = message.get("params") or {}
            if params.get("threadId") != thread_id:
                continue
            turn = params.get("turn") or {}
            if turn.get("id") != turn_id:
                continue
            return 0 if turn.get("status") == "completed" else 1

    @staticmethod
    def _thread_id(result: dict[str, Any]) -> str:
        thread = result.get("thread") or {}
        thread_id = thread.get("id")
        if not thread_id:
            raise AppServerError(f"thread/start 或 thread/resume 没有返回 threadId：{result}")
        return str(thread_id)

    def _remember_thread_info(self, result: dict[str, Any]) -> None:
        self.thread_info = {
            "model": result.get("model"),
            "model_provider": result.get("modelProvider"),
            "reasoning_effort": result.get("reasoningEffort"),
            "service_tier": result.get("serviceTier"),
        }

    def start_or_resume_thread(
        self,
        *,
        cwd: Path,
        thread_id: str | None,
        read_only: bool,
        auto_approve: bool,
    ) -> str:
        self.start()
        sandbox = "read-only" if read_only else "workspace-write"
        approval_policy = "never" if auto_approve or read_only else "on-request"
        if thread_id:
            try:
                result = self.request(
                    "thread/resume",
                    {
                        "threadId": thread_id,
                        "cwd": str(cwd),
                        "sandbox": sandbox,
                        "approvalPolicy": approval_policy,
                        "excludeTurns": False,
                    },
                )
                self._remember_thread_info(result)
                return self._thread_id(result)
            except AppServerError as exc:
                # A thread created before its first turn may have no rollout to
                # resume. Start a fresh thread rather than making recovery fail.
                if "no rollout found" not in str(exc):
                    raise

        result = self.request(
            "thread/start",
            {
                "cwd": str(cwd),
                "sandbox": sandbox,
                "approvalPolicy": approval_policy,
                "historyMode": "paginated",
                "ephemeral": False,
            },
        )
        self._remember_thread_info(result)
        return self._thread_id(result)

    def run_turn(
        self,
        *,
        cwd: Path,
        prompt: str,
        thread_id: str | None = None,
        read_only: bool = False,
        auto_approve: bool = False,
        on_thread_id: Callable[[str], None] | None = None,
        on_turn_id: Callable[[str], None] | None = None,
        on_thread_info: Callable[[dict[str, Any]], None] | None = None,
        output_schema: dict[str, Any] | None = None,
    ) -> tuple[str, str, int]:
        actual_thread_id = self.start_or_resume_thread(
            cwd=cwd,
            thread_id=thread_id,
            read_only=read_only,
            auto_approve=auto_approve,
        )
        if on_thread_id:
            on_thread_id(actual_thread_id)
        if on_thread_info:
            on_thread_info(dict(self.thread_info))
        turn_params: dict[str, Any] = {
            "threadId": actual_thread_id,
            "input": [{"type": "text", "text": prompt}],
            "summary": "concise",
        }
        if output_schema is not None:
            turn_params["outputSchema"] = output_schema
        result = self.request("turn/start", turn_params)
        turn = result.get("turn") or {}
        turn_id = str(turn.get("id", ""))
        if not turn_id:
            raise AppServerError(f"turn/start 没有返回 turnId：{result}")
        if on_turn_id:
            on_turn_id(turn_id)
        code = self._wait_for_turn(actual_thread_id, turn_id)
        return actual_thread_id, turn_id, code

    def interrupt(self, thread_id: str, turn_id: str) -> None:
        self.request("turn/interrupt", {"threadId": thread_id, "turnId": turn_id})

    def delete_thread(self, thread_id: str) -> None:
        self.start()
        self.request("thread/delete", {"threadId": thread_id})

    def close(self) -> None:
        with self._lock:
            self._closed = True
            process = self.process
            self.process = None
        if process and process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()

    def __enter__(self) -> "AppServerClient":
        self.start()
        return self

    def __exit__(self, *_: object) -> None:
        self.close()
