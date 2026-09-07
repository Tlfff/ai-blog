from __future__ import annotations

import json
import os
import re
import subprocess
import threading
from types import SimpleNamespace
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Callable

from app_server import AppServerClient, AppServerError
from persistence import StateStore
from reliability import PreflightReport, ToolPreflight
from runtime_environment import RuntimeEnvironment
from workflow import (
    ChangeSet,
    ContextPackBuilder,
    ReviewPackBuilder,
    ReviewResult,
    sanitize_unrelated_generated_drift,
    ValidationPlanner,
    ValidationResult,
    ValidationStepResult,
    parse_review_result,
)

PROJECT_DIR = Path(__file__).resolve().parent
os.environ.setdefault("CREWAI_STORAGE_DIR", str(PROJECT_DIR / ".crewai"))
os.environ.setdefault("OTEL_SDK_DISABLED", "true")

from crewai.flow import Flow, listen, router, start  # noqa: E402
from pydantic import BaseModel  # noqa: E402

NotificationSink = Callable[[dict[str, Any]], None]
DRIVER_KEY = "refactor"
DRIVER_ROLE = "driver"
REVIEW_ROLE = "review_sidecar"


def prompt_scope_mismatches(issue_number: int, prompt: str) -> tuple[int, ...]:
    """Return explicit target Issue numbers in a prompt that conflict with the run."""
    if not prompt.strip():
        return ()
    patterns = (
        r"(?:接手并完成|负责独立完成|请实现|只处理当前|实现)\s*(?:GitHub\s+)?Issue\s*#(\d+)",
        r"\.worktrees/issue-(\d+)",
        r"(?:当前分支[:：]?\s*|branch\s+)(?:`)?codex/issue-(\d+)",
    )
    targets = {
        int(match)
        for pattern in patterns
        for match in re.findall(pattern, prompt, flags=re.IGNORECASE)
    }
    return tuple(sorted(target for target in targets if target != issue_number))


class PipelineState(BaseModel):
    issue_number: int
    issue_title: str
    issue_body: str = ""
    worktree_path: str
    test_command: str
    refactor_prompt: str
    auto_approve: bool = False
    status: str = "queued"
    test_exit_code: int | None = None
    refactor_exit_code: int | None = None
    repair_exit_code: int | None = None
    repair_attempted: bool = False
    matt_workflow: bool = False
    review_cycle: int = 0
    max_review_cycles: int = 2


@dataclass(frozen=True)
class PipelineConfig:
    issue_number: int
    issue_title: str
    issue_body: str
    worktree_path: Path
    test_command: str
    refactor_prompt: str
    auto_approve: bool
    matt_workflow: bool = False
    max_review_cycles: int = 2
    resource_lock: str = ""

    def __post_init__(self) -> None:
        if not 0 <= self.max_review_cycles <= 2:
            raise ValueError("自动返工轮次必须在 0 到 2 之间。")
        mismatches = prompt_scope_mismatches(self.issue_number, self.refactor_prompt)
        if mismatches:
            targets = "、".join(f"#{number}" for number in mismatches)
            raise ValueError(
                f"所选 Issue #{self.issue_number} 与 Driver 提示词目标 {targets} 不一致；"
                "请清空旧提示词并重新填写。"
            )


class PipelineRun:
    """Thread-safe in-memory view backed by SQLite for restart/recovery."""

    def __init__(
        self,
        config: PipelineConfig,
        store: StateStore,
        run_id: str,
        runtime_environment: RuntimeEnvironment | None = None,
    ):
        self.config = config
        self.store = store
        self.run_id = run_id
        self.runtime_environment = runtime_environment or RuntimeEnvironment.create(
            run_id=run_id,
            issue_number=config.issue_number,
            worktree_path=config.worktree_path,
            slot=0,
        )
        self.status = "queued"
        self.logs: list[str] = store.logs_for_run(run_id)
        self.processes: dict[str, subprocess.Popen[str]] = {}
        self.app_clients: dict[str, AppServerClient] = {}
        self.stop_event = threading.Event()
        self.started_at = datetime.now(timezone.utc)
        self.finished_at: datetime | None = None
        self.error: str | None = None
        self.review_reports: dict[str, str] = {}
        self.review_results: dict[str, ReviewResult] = {}
        self.validation_results: list[ValidationStepResult] = []
        self.activities: list[dict[str, str]] = store.events_for_run(run_id)
        self.agents: dict[str, dict[str, object]] = {}
        self.current_agent: str | None = None
        self.current_command: str | None = None
        self.model_info: dict[str, str] = {}
        self._lock = threading.RLock()
        self.thread: threading.Thread | None = None

    def log(self, message: str) -> None:
        timestamp = datetime.now().strftime("%H:%M:%S")
        line = f"[{timestamp}] {message}"
        with self._lock:
            self.logs.append(line)
            self.logs = self.logs[-3000:]
        self.store.append_log(self.run_id, line)

    def add_activity(self, agent_key: str, agent_label: str, kind: str, content: str) -> None:
        content = content.strip()
        if not content:
            return
        event = {
            "agent_key": agent_key,
            "agent_label": agent_label,
            "kind": kind,
            "content": content,
            "created_at": datetime.now(timezone.utc).isoformat(),
        }
        with self._lock:
            self.activities.append(event)
            self.activities = self.activities[-500:]
        self.store.append_event(self.run_id, agent_key, agent_label, kind, content)

    def set_status(self, status: str) -> None:
        with self._lock:
            self.status = status
        self.store.update_run(self.run_id, status=status, current_stage=status)

    def set_process(self, name: str, process: subprocess.Popen[str] | None) -> None:
        with self._lock:
            if process is None:
                self.processes.pop(name, None)
            else:
                self.processes[name] = process

    def update_agent_command(self, key: str, command: str) -> None:
        with self._lock:
            agent = self.agents.get(key)
            if agent is None:
                return
            agent["command"] = command
            label = str(agent.get("label", key))
            status = str(agent.get("status", "running"))
            self.current_agent = key
            self.current_command = command
        self.store.upsert_session(
            self.run_id,
            key,
            label,
            status=status,
            command=command,
        )

    def set_app_client(self, name: str, client: AppServerClient | None) -> None:
        with self._lock:
            if client is None:
                self.app_clients.pop(name, None)
            else:
                self.app_clients[name] = client
                if client.process:
                    self.processes[name] = client.process

    def begin_agent(
        self,
        key: str,
        label: str,
        command: str,
        stage: str,
        *,
        role: str = "executor",
        read_only: bool = False,
    ) -> None:
        now = datetime.now(timezone.utc)
        with self._lock:
            self.agents[key] = {
                "label": label,
                "status": "running",
                "command": command,
                "stage": stage,
                "started_at": now,
                "finished_at": None,
                "exit_code": None,
                "role": role,
                "read_only": read_only,
            }
            self.current_agent = key
            self.current_command = command
        self.store.upsert_session(
            self.run_id,
            key,
            label,
            status="running",
            command=command,
            role=role,
            read_only=read_only,
        )

    def set_thread_id(self, key: str, *, thread_id: str) -> None:
        with self._lock:
            agent = self.agents.get(key)
            label = str(agent.get("label", key)) if agent else key
            command = str(agent.get("command", "")) if agent else ""
        self.store.upsert_session(
            self.run_id,
            key,
            label,
            thread_id=thread_id,
            status="running",
            command=command,
        )

    def set_session_ids(self, key: str, *, thread_id: str, turn_id: str) -> None:
        with self._lock:
            agent = self.agents.get(key)
            label = str(agent.get("label", key)) if agent else key
            command = str(agent.get("command", "")) if agent else ""
        self.store.upsert_session(
            self.run_id,
            key,
            label,
            thread_id=thread_id,
            turn_id=turn_id,
            status="running",
            command=command,
        )

    def set_model_info(self, key: str, info: dict[str, Any]) -> None:
        model = str(info.get("model") or "")
        provider = str(info.get("model_provider") or "")
        effort = str(info.get("reasoning_effort") or "")
        with self._lock:
            agent = self.agents.get(key)
            label = str(agent.get("label", key)) if agent else key
            command = str(agent.get("command", "")) if agent else ""
            if agent is not None:
                agent["model"] = model
                agent["model_provider"] = provider
                agent["reasoning_effort"] = effort
            self.model_info = {
                "model": model,
                "model_provider": provider,
                "reasoning_effort": effort,
            }
        self.store.update_run(
            self.run_id,
            model=model,
            model_provider=provider,
            reasoning_effort=effort,
        )
        self.store.upsert_session(
            self.run_id,
            key,
            label,
            status="running",
            command=command,
            model=model,
            model_provider=provider,
            reasoning_effort=effort,
        )

    def end_agent(self, key: str, exit_code: int, report: str = "") -> None:
        now = datetime.now(timezone.utc)
        with self._lock:
            agent = self.agents.get(key)
            if agent is not None:
                status = "stopped" if self.stop_event.is_set() else (
                    "succeeded" if exit_code == 0 else "failed"
                )
                agent["status"] = status
                agent["finished_at"] = now
                agent["exit_code"] = exit_code
                label = str(agent.get("label", key))
                command = str(agent.get("command", ""))
            else:
                status, label, command = "failed", key, ""
            running = [
                name for name, item in self.agents.items() if item.get("status") == "running"
            ]
            if running:
                self.current_agent = running[-1]
                self.current_command = str(self.agents[running[-1]].get("command", ""))
            else:
                self.current_agent = None
                self.current_command = None
        self.store.upsert_session(
            self.run_id,
            key,
            label,
            status=status,
            command=command,
            report=report,
        )

    def request_stop(self) -> None:
        self.stop_event.set()
        with self._lock:
            clients = list(self.app_clients.values())
            processes = list(self.processes.values())
        for client in clients:
            client.close()
        for process in processes:
            if process.poll() is None:
                process.terminate()

    def snapshot(self) -> dict[str, object]:
        with self._lock:
            return {
                "run_id": self.run_id,
                "status": self.status,
                "logs": list(self.logs),
                "started_at": self.started_at,
                "finished_at": self.finished_at,
                "error": self.error,
                "review_reports": dict(self.review_reports),
                "review_results": dict(self.review_results),
                "validation_results": list(self.validation_results),
                "activities": [dict(item) for item in self.activities],
                "agents": {key: dict(value) for key, value in self.agents.items()},
                "current_agent": self.current_agent,
                "current_command": self.current_command,
                "model_info": dict(self.model_info),
                "config": self.config,
                "running": self.thread is not None and self.thread.is_alive(),
            }


class IssuePipelineFlow(Flow[PipelineState]):
    """Deterministic CrewAI Flow using a persistent Codex App Server thread."""

    def __init__(self, run: PipelineRun):
        super().__init__(
            initial_state=PipelineState(
                issue_number=run.config.issue_number,
                issue_title=run.config.issue_title,
                issue_body=run.config.issue_body,
                worktree_path=str(run.config.worktree_path),
                test_command=run.config.test_command,
                refactor_prompt=run.config.refactor_prompt,
                auto_approve=run.config.auto_approve,
                matt_workflow=run.config.matt_workflow,
                max_review_cycles=run.config.max_review_cycles,
            ),
            tracing=False,
        )
        self.run = run
        self.validation_planner = ValidationPlanner()
        self._review_recheck_results: dict[str, ReviewResult] = {}
        self.context_pack = ContextPackBuilder(self._load_issue).build(
            run.config.worktree_path,
            run.config.issue_body,
        )

    def _load_issue(self, issue_number: int) -> tuple[str, str]:
        completed = subprocess.run(
            ["gh", "issue", "view", str(issue_number), "--json", "title,body"],
            cwd=self.run.config.worktree_path,
            text=True,
            capture_output=True,
            check=False,
            timeout=30,
        )
        if completed.returncode != 0:
            raise RuntimeError(completed.stderr.strip() or f"读取 Issue #{issue_number} 失败")
        payload = json.loads(completed.stdout)
        return str(payload.get("title", "")), str(payload.get("body", ""))

    def _status(self, status: str, message: str) -> None:
        self.state.status = status
        self.run.set_status(status)
        self.run.log(message)

    @staticmethod
    def _display_agent_command(*, read_only: bool) -> str:
        sandbox = "read-only" if read_only else "workspace-write"
        return f"codex app-server → thread/resume|start → turn/start ({sandbox})"

    @staticmethod
    def _event_text(message: dict[str, Any]) -> str:
        params = message.get("params") or {}
        for key in ("delta", "text", "output", "explanation"):
            value = params.get(key)
            if isinstance(value, str) and value.strip():
                return value.strip()
        item = params.get("item") or {}
        if isinstance(item, dict):
            for key in ("command", "text", "name", "status"):
                value = item.get(key)
                if isinstance(value, (str, list)):
                    return str(value).strip()
        return ""

    @staticmethod
    def _plan_text(message: dict[str, Any]) -> str:
        params = message.get("params") or {}
        lines: list[str] = []
        explanation = params.get("explanation")
        if isinstance(explanation, str) and explanation.strip():
            lines.append(explanation.strip())
        for step in params.get("plan") or []:
            if not isinstance(step, dict):
                continue
            text = step.get("step") or step.get("text") or step.get("description")
            status = step.get("status")
            if text:
                marker = {
                    "completed": "✅",
                    "inProgress": "🔵",
                    "in_progress": "🔵",
                    "pending": "⚪",
                }.get(
                    str(status), "•"
                )
                lines.append(f"{marker} {text}")
        return "\n".join(lines)

    @staticmethod
    def _item_activity(message: dict[str, Any]) -> str:
        item = (message.get("params") or {}).get("item") or {}
        if not isinstance(item, dict):
            return ""
        item_type = str(item.get("type", ""))
        if "command" in item_type.lower():
            command = item.get("command")
            if command:
                return f"准备执行命令：{command}"
        if "file" in item_type.lower():
            path = item.get("path") or item.get("file")
            return f"正在处理文件：{path}" if path else "正在处理文件变更"
        if "tool" in item_type.lower():
            name = item.get("name") or item.get("server")
            return f"正在调用工具：{name}" if name else "正在调用工具"
        return ""

    def _notification_sink(
        self,
        agent_key: str,
        label: str,
        collector: list[str] | None = None,
    ) -> NotificationSink:
        def sink(message: dict[str, Any]) -> None:
            method = str(message.get("method", "event"))
            text = self._event_text(message)
            if method == "model/rerouted":
                params = message.get("params") or {}
                current = dict(self.run.model_info)
                current["model"] = str(params.get("toModel") or current.get("model") or "")
                self.run.set_model_info(agent_key, current)
            if method == "item/reasoning/summaryTextDelta" and text:
                self.run.add_activity(agent_key, label, "reasoning", text)
                return
            if method == "turn/plan/updated":
                plan_text = self._plan_text(message)
                if plan_text:
                    self.run.add_activity(agent_key, label, "plan", plan_text)
                return
            if method == "item/agentMessage/delta" and text:
                self.run.add_activity(agent_key, label, "commentary", text)
                return
            if method == "item/completed" and collector is not None:
                item = (message.get("params") or {}).get("item") or {}
                if (
                    isinstance(item, dict)
                    and item.get("type") == "agentMessage"
                    and item.get("phase") in {"final_answer", None}
                    and isinstance(item.get("text"), str)
                ):
                    collector.clear()
                    collector.append(item["text"])
                    return
            if method == "item/started":
                activity = self._item_activity(message)
                if activity:
                    self.run.add_activity(agent_key, label, "action", activity)
            if text:
                self.run.log(f"[{label}] {method}: {text.replace(chr(10), ' ')[:500]}")
            elif method not in {
                "turn/started", "turn/completed", "thread/status/changed",
                "item/reasoning/summaryPartAdded",
            }:
                self.run.log(f"[{label}] {method}")

        return sink

    def _run_codex_turn(
        self,
        prompt: str,
        label: str,
        process_name: str,
        *,
        read_only: bool = False,
        collector: list[str] | None = None,
        output_schema: dict[str, Any] | None = None,
        role: str = "executor",
        turn_kind: str = "execution",
        force_new_thread: bool = False,
    ) -> tuple[int, str, str, str]:
        display_command = self._display_agent_command(read_only=read_only)
        self.run.begin_agent(
            process_name,
            label,
            display_command,
            self.state.status,
            role=role,
            read_only=read_only,
        )
        session = self.run.store.get_session(
            self.run.run_id,
            process_name,
        )
        previous_thread_id = None if force_new_thread else (
            session.get("thread_id") if session else None
        )
        turn_state: dict[str, Any] = {
            "thread_id": str(previous_thread_id or ""),
            "turn_id": "",
            "started": False,
        }

        def record_turn_start() -> None:
            if turn_state["started"] or not turn_state["thread_id"] or not turn_state["turn_id"]:
                return
            self.run.store.start_turn(
                run_id=self.run.run_id,
                thread_id=str(turn_state["thread_id"]),
                turn_id=str(turn_state["turn_id"]),
                turn_kind=turn_kind,
                role=role,
                read_only=read_only,
                prompt_summary=prompt[:500],
                session_key=process_name,
            )
            turn_state["started"] = True

        def remember_thread(value: str) -> None:
            turn_state["thread_id"] = value
            self.run.set_thread_id(process_name, thread_id=value)
            record_turn_start()

        def remember_turn(value: str) -> None:
            turn_state["turn_id"] = value
            self.run.store.upsert_session(
                self.run.run_id,
                process_name,
                label,
                turn_id=value,
            )
            record_turn_start()
        client = AppServerClient(
            self._notification_sink(process_name, label, collector),
            process_env=self.run.runtime_environment.process_env(),
        )
        self.run.set_app_client(process_name, client)
        try:
            client.start()
            self.run.set_app_client(process_name, client)
            thread_id, turn_id, code = client.run_turn(
                cwd=self.run.config.worktree_path,
                prompt=prompt,
                thread_id=str(previous_thread_id) if previous_thread_id else None,
                read_only=read_only,
                auto_approve=self.run.config.auto_approve or read_only,
                on_thread_id=remember_thread,
                on_turn_id=remember_turn,
                on_thread_info=lambda info: self.run.set_model_info(process_name, info),
                output_schema=output_schema,
            )
            self.run.set_session_ids(process_name, thread_id=thread_id, turn_id=turn_id)
            turn_state["thread_id"] = thread_id
            turn_state["turn_id"] = turn_id
            record_turn_start()
            report = "".join(collector or []).strip()
            self.run.store.finish_turn(
                run_id=self.run.run_id,
                turn_id=turn_id,
                status="succeeded" if code == 0 else "failed",
                report=report,
            )
            self.run.end_agent(process_name, code, report)
            return code, thread_id, turn_id, report
        except (AppServerError, OSError) as exc:
            self.run.log(f"{label}失败：{exc}")
            if turn_state["started"]:
                self.run.store.finish_turn(
                    run_id=self.run.run_id,
                    turn_id=str(turn_state["turn_id"]),
                    status="failed",
                    report=str(exc),
                )
            self.run.end_agent(process_name, 1, "")
            return 1, str(previous_thread_id or ""), str(session.get("turn_id", "") if session else ""), ""
        finally:
            client.close()
            self.run.set_app_client(process_name, None)

    @start()
    def refactor(self) -> str:
        self._status("driver_plan", f"Driver 开始规划 Issue #{self.state.issue_number}。")
        plan_prompt = (
            f"你是 Issue #{self.state.issue_number} 的唯一 Codex Driver。当前阶段只做实现规划，"
            "不要修改文件。\n\n"
            + self.context_pack.instructions()
            + "\n\n"
            f"当前 Issue：{self.state.issue_title}\n{self.state.issue_body}\n\n"
            "输出可执行的“验收标准 → 技术决策 → 实现位置 → 测试”映射清单，"
            "并明确预计读取和修改的最小文件范围。"
        )
        if self.state.refactor_prompt.strip():
            plan_prompt += "\n\n用户补充要求：\n" + self.state.refactor_prompt.strip()
        if self.run.stop_event.is_set():
            self._status("stopped", "Driver 规划尚未启动，任务已停止。")
            return "STOPPED"
        plan_code, _, _, _ = self._run_codex_turn(
            plan_prompt,
            "Codex Driver · 规划",
            DRIVER_KEY,
            role=DRIVER_ROLE,
            turn_kind="driver_plan",
        )
        if plan_code != 0:
            self._status("failed", "Driver 规划失败，不进入实现阶段。")
            return "FAILED"

        self._status("driver_implement", f"Driver 开始实现 Issue #{self.state.issue_number}。")
        prompt = (
            f"请实现 Issue #{self.state.issue_number}：{self.state.issue_title}\n\n"
            f"Issue 正文：\n{self.state.issue_body}\n\n"
            "请先检查仓库现状，只在当前独立 worktree 中修改代码；完成后运行必要检查，"
            "不要切换到 master、不要提交或合并 Git 分支。不要运行会重写全仓库生成文件的 "
            "make api、make config 或 make gen；控制器会按变更源文件执行定向生成和验证。"
        )
        if self.state.refactor_prompt.strip():
            prompt += (
                "\n\n以下是用户追加的执行要求，请在不违背 Issue 和仓库规范的前提下遵守：\n"
                + self.state.refactor_prompt.strip()
            )
        if self.state.matt_workflow:
            prompt = (
                "继续刚才同一个 Driver Thread，按已输出的映射清单实施。"
                "用 TDD 思路逐个行为切片，先写失败测试，再实现，再运行相关测试。"
                "控制器会在后续执行测试和双轴审查；只在当前独立 worktree 中修改，"
                "不要提交或合并；提交由审查全通过后的人工批准步骤处理。完成后说明检查结果。\n\n"
                + prompt
            )
        if self.run.stop_event.is_set():
            self._status("stopped", "重构 Agent 尚未启动，任务已停止。")
            return "STOPPED"
        code, _, _, _ = self._run_codex_turn(
            prompt,
            "Codex Driver · 实现",
            DRIVER_KEY,
            role=DRIVER_ROLE,
            turn_kind="driver_implement",
        )
        self.state.refactor_exit_code = code
        if self.run.stop_event.is_set():
            self._status("stopped", "重构 Agent 已停止。")
            return "STOPPED"
        if code != 0:
            self._status("failed", "重构 Agent 执行失败，不进入测试阶段。")
            return "FAILED"
        return "READY_FOR_TEST"

    @router(refactor, emit=["TEST", "FAILED", "STOPPED"])
    def after_refactor(self, outcome: str) -> str:
        return "TEST" if outcome == "READY_FOR_TEST" else outcome

    @listen("TEST")
    def test(self) -> str:
        self._status("validate", "开始执行确定性验证矩阵。")
        result = self._run_validation_matrix("test", "测试执行器", "testing")
        self.state.test_exit_code = 0 if result.passed else 1
        if self.run.stop_event.is_set():
            self._status("stopped", "测试阶段已停止。")
            return "STOPPED"
        if not result.passed:
            return "FAIL"
        if self.state.matt_workflow and not self._driver_pre_review_check():
            return "FAIL"
        return "PASS"

    def _driver_pre_review_check(self) -> bool:
        self._status(
            "driver_verify",
            "验证通过，原 Driver 在 Sidecar 审查前执行验收与高风险链路自检。",
        )
        changes = ChangeSet.capture(self.run.config.worktree_path)
        prompt = (
            f"Issue #{self.state.issue_number} 已通过确定性验证。现在仍在同一个 Driver Thread 中，"
            "请在只查看当前完整 diff 的基础上做一次审查前自检，并立即修复确认存在的问题。\n\n"
            "必须逐条核对：\n"
            "1. 每项验收标准都有实现证据和对应成功/失败/边界测试；\n"
            "2. 凭证消费、并发计数、事务和锁等操作在正确的原子边界内；\n"
            "3. Outbox/Inbox、发布器、Consumer 和投影形成实际闭环，不是只有落库；\n"
            "4. 新增 Adapter、补偿 Job、Redis Lua 和失败重试有分层测试；\n"
            "5. 手写字段、具名函数和多参数函数符合相关 AGENTS.md 的强制注释要求；\n"
            "6. 不修改与当前 Issue 无关的代码和生成文件。\n\n"
            "不要运行 make api、make config 或 make gen；控制器稍后会定向生成。"
            "如果无需修改，明确说明每项证据；如果需要修改，完成后运行最小相关测试。\n\n"
            "当前变更清单：\n- " + "\n- ".join(changes.paths)
        )
        code, _, _, _ = self._run_codex_turn(
            prompt,
            "Codex Driver · 审查前自检",
            DRIVER_KEY,
            role=DRIVER_ROLE,
            turn_kind="driver_pre_review",
        )
        if code != 0 or self.run.stop_event.is_set():
            return False
        self._status("validate", "Driver 自检完成，重新执行确定性验证。")
        validation = self._run_validation_matrix(
            "pre-review-validation",
            "审查前验证执行器",
            "testing",
        )
        self.state.test_exit_code = 0 if validation.passed else 1
        return validation.passed

    def _run_validation_matrix(
        self,
        process_name: str,
        label: str,
        stage: str,
    ) -> ValidationResult:
        sanitized = sanitize_unrelated_generated_drift(self.run.config.worktree_path)
        if sanitized:
            self.run.log(
                "[生成隔离] 已恢复无关生成漂移，不回传 Driver："
                + ", ".join(sanitized)
            )
        change_set = ChangeSet.capture(self.run.config.worktree_path)
        steps = self.validation_planner.plan(
            change_set.paths,
            self.state.test_command,
            self.state.issue_body,
            change_set.untracked_paths,
        )
        summary = " → ".join(step.name for step in steps)
        self.run.begin_agent(process_name, label, summary, stage)
        results: list[ValidationStepResult] = []
        for step in steps:
            if self.run.stop_event.is_set():
                break
            self.run.update_agent_command(process_name, step.command)
            code, output = self._run_shell_command(step.command, step.name, process_name)
            result = ValidationStepResult(step.name, step.command, code, output)
            results.append(result)
            self.run.validation_results.append(result)
            if code != 0:
                break
        validation = ValidationResult(tuple(results))
        self.run.end_agent(process_name, 0 if validation.passed else 1)
        return validation

    def _run_shell_command(
        self,
        command: str,
        label: str,
        process_name: str,
    ) -> tuple[int, str]:
        self.run.log(f"启动 {label}：{command}")
        process = subprocess.Popen(
            ["/bin/zsh", "-lc", command],
            cwd=self.run.config.worktree_path,
            env=self.run.runtime_environment.process_env(),
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1,
        )
        self.run.set_process(process_name, process)
        output_lines: list[str] = []
        assert process.stdout is not None
        for line in process.stdout:
            if self.run.stop_event.is_set():
                break
            line = line.rstrip()
            if line:
                output_lines.append(line)
                self.run.log(line)
        if self.run.stop_event.is_set() and process.poll() is None:
            process.terminate()
        code = process.wait()
        process.stdout.close()
        self.run.set_process(process_name, None)
        self.run.log(f"{label}结束，退出码：{code}")
        return code, "\n".join(output_lines)

    @router(test, emit=["COMPLETE", "REVIEW", "REPAIR", "STOPPED"])
    def after_test(self, outcome: str) -> str:
        if outcome == "PASS":
            return "REVIEW"
        if outcome == "FAIL":
            return "REPAIR"
        return "STOPPED"

    @listen("REPAIR")
    def repair(self) -> str:
        self.state.repair_attempted = True
        self._status("driver_repair", "测试失败，回传原 Codex Driver 修复。")
        latest_validation = ValidationResult(tuple(self.run.validation_results[-8:]))
        prompt = (
            f"Issue #{self.state.issue_number}（{self.state.issue_title}）的自动测试失败。\n\n"
            f"失败详情：\n{latest_validation.feedback()}\n\n"
            "请检查当前 worktree 中的测试输出和代码，修复导致失败的问题。"
            "只修改必要文件，不要提交或合并 Git 分支。"
        )
        code, _, _, _ = self._run_codex_turn(
            prompt,
            "Codex Driver · 验证修复",
            DRIVER_KEY,
            role=DRIVER_ROLE,
            turn_kind="validation_repair",
        )
        self.state.repair_exit_code = code
        if self.run.stop_event.is_set():
            self._status("stopped", "修复 Agent 已停止。")
            return "STOPPED"
        return "RETEST" if code == 0 else "FAILED"

    @router(repair, emit=["RETEST", "FAILED", "STOPPED"])
    def after_repair(self, outcome: str) -> str:
        if outcome == "RETEST":
            return "RETEST"
        if outcome == "STOPPED":
            return "STOPPED"
        self._status("failed", "修复 Agent 执行失败。")
        return "FAILED"

    @listen("RETEST")
    def retest(self) -> str:
        self._status("targeted_validate", "修复完成，执行针对性验证。")
        result = self._run_validation_matrix(
            "retest",
            "修复后测试执行器",
            "retesting",
        )
        self.state.test_exit_code = 0 if result.passed else 1
        if self.run.stop_event.is_set():
            self._status("stopped", "修复后测试已停止。")
            return "STOPPED"
        return "PASS" if result.passed else "FAIL"

    @router(retest, emit=["COMPLETE", "REVIEW", "FAILED", "STOPPED"])
    def after_retest(self, outcome: str) -> str:
        if outcome == "PASS":
            return "REVIEW"
        if outcome == "STOPPED":
            return "STOPPED"
        self._status("failed", "修复后测试仍然失败，请查看日志并人工处理。")
        return "FAILED"

    def _review_prompt(self, axis: str) -> str:
        change_set = ChangeSet.capture(self.run.config.worktree_path)
        review_pack = ReviewPackBuilder.build(
            worktree=self.run.config.worktree_path,
            change_set=change_set,
            context_pack=self.context_pack,
            issue_title=self.state.issue_title,
            issue_body=self.state.issue_body,
            validation_steps=tuple(self.run.validation_results),
        )
        supplemental = ""
        if self.state.refactor_prompt.strip():
            supplemental = (
                "\n本次运行的用户补充要求（作为当前 Issue 的最新澄清）：\n"
                f"{self.state.refactor_prompt.strip()}\n"
            )
        common = (
            f"Issue #{self.state.issue_number}：{self.state.issue_title}\n\n"
            f"Issue 正文：\n{self.state.issue_body}\n\n"
            f"{supplemental}\n"
            "本次 ReviewPack 已由控制器预先收集，不允许无目标遍历整个仓库。\n"
            f"完整变更清单：\n{review_pack.manifest(review_pack.change_paths)}\n\n"
            f"验证矩阵结果：\n{review_pack.validation_summary}\n\n"
            "这是只读审查，不要修改文件、不要提交、不要合并。\n"
            "最多报告 5 项；结论和每项说明保持简洁。"
            "只有 P0/P1 或明确违反验收标准/仓库硬性规范才导致 FAIL；"
            "P2、代码异味和优化建议必须标为 Advisory，不触发自动返工。\n"
            "初次审查必须一次性报告当前范围内所有可发现的 P0/P1，不要把明显问题留到后续轮次。\n"
        )
        previous = self._review_recheck_results.get(axis)
        if self.state.review_cycle > 0 and previous:
            blocking = ReviewResult(
                "FAIL",
                previous.summary,
                tuple(
                    finding
                    for finding in previous.findings
                    if finding.severity in {"P0", "P1"}
                ),
                previous.raw,
            ).feedback("Standards" if axis == "standards" else "Spec")
            common += (
                "\n这是返工后的定向复核，不是重新开放整个仓库审查。只核验下列既有 P0/P1 是否已修复，"
                "以及本次返工是否直接引入新的 P0/P1 回归；不得把初审时已存在但未报告的机械问题"
                "在后续轮次新升级为阻断项。\n\n"
                f"既有阻断发现：\n{blocking}\n"
            )
        if axis == "standards":
            return common + (
                f"规范来源仅限：\n{review_pack.manifest(review_pack.standards_sources)}\n\n"
                f"手写代码、测试和配置源：\n{review_pack.manifest(review_pack.handwritten_paths)}\n\n"
                f"自动生成文件：\n{review_pack.manifest(review_pack.generated_paths)}\n\n"
                "执行 Standards 轴精确审查：逐项检查手写代码、测试和配置源。"
                "自动生成文件不逐行做代码风格审查，只检查生成源是否在变更清单、"
                "生成命令是否通过、以及是否出现与当前工单无关的生成漂移。"
                "只返回符合输出结构的结果，不要使用 Markdown 代码块。"
            )
        return common + (
            f"与当前工单匹配的规格摘录：\n{review_pack.spec_excerpt or '无额外摘录，以当前 Issue 为准。'}\n\n"
            "执行 Spec 轴精确审查：先逐条核对当前 Issue 验收标准，再检查父规格架构决策和"
            "相关功能文档摘录；不要阅读与当前模块、接口和行为无关的章节。"
            "自动生成文件只检查协议契约和生成一致性，不审查其内部实现风格。"
            "只返回符合输出结构的结果，不要使用 Markdown 代码块。"
        )

    @staticmethod
    def _review_output_schema() -> dict[str, Any]:
        return {
            "type": "object",
            "additionalProperties": False,
            "properties": {
                "verdict": {"type": "string", "enum": ["PASS", "FAIL"]},
                "summary": {"type": "string", "maxLength": 300},
                "findings": {
                    "type": "array",
                    "maxItems": 5,
                    "items": {
                        "type": "object",
                        "additionalProperties": False,
                        "properties": {
                            "severity": {
                                "type": "string",
                                "enum": ["P0", "P1", "P2", "Advisory"],
                            },
                            "title": {"type": "string", "maxLength": 80},
                            "location": {"type": "string", "maxLength": 180},
                            "evidence": {"type": "string", "maxLength": 300},
                            "recommendation": {"type": "string", "maxLength": 300},
                        },
                        "required": [
                            "severity",
                            "title",
                            "location",
                            "evidence",
                            "recommendation",
                        ],
                    },
                },
            },
            "required": ["verdict", "summary", "findings"],
        }

    @staticmethod
    def _extract_review_text(lines: list[str]) -> str:
        return "\n".join(lines).strip()

    def _run_review_agent(self, axis: str, cycle: int = 0) -> tuple[str, int, str]:
        label = "规范审查 Agent" if axis == "standards" else "规格审查 Agent"
        session_key = f"review-{axis}" if cycle == 0 else f"review-{axis}-{cycle}"
        lines: list[str] = []
        code, _, _, report = self._run_codex_turn(
            self._review_prompt(axis),
            label,
            session_key,
            read_only=True,
            collector=lines,
            output_schema=self._review_output_schema(),
            role=REVIEW_ROLE,
            turn_kind=f"{axis}_review",
            force_new_thread=True,
        )
        return axis, code, report or self._extract_review_text(lines)

    def _run_review_pair(
        self,
        cycle: int = 0,
    ) -> tuple[dict[str, str], dict[str, ReviewResult], list[str]]:
        reports: dict[str, str] = {}
        results: dict[str, ReviewResult] = {}
        process_failures: list[str] = []
        with ThreadPoolExecutor(max_workers=2, thread_name_prefix="review-agent") as executor:
            futures = {
                executor.submit(self._run_review_agent, "standards", cycle): "规范审查",
                executor.submit(self._run_review_agent, "spec", cycle): "规格审查",
            }
            for future in as_completed(futures):
                axis, code, report = future.result()
                reports[axis] = report
                results[axis] = parse_review_result(report)
                if code != 0:
                    process_failures.append(futures[future])
        self.run.review_reports = reports
        self.run.review_results = results
        return reports, results, process_failures

    @staticmethod
    def _review_feedback(results: dict[str, ReviewResult]) -> str:
        return "\n\n".join(
            ReviewResult(
                "FAIL",
                result.summary,
                tuple(
                    finding
                    for finding in result.findings
                    if finding.severity in {"P0", "P1"}
                ),
                result.raw,
            ).feedback("Standards" if axis == "standards" else "Spec")
            for axis, result in sorted(results.items())
            if not result.passed
        )

    def _repair_review_findings(
        self,
        results: dict[str, ReviewResult],
        cycle: int,
    ) -> bool:
        feedback = self._review_feedback(results)
        self._status(
            "driver_repair",
            f"P0/P1 审查失败，回传原 Driver 执行第 {cycle}/{self.state.max_review_cycles} 轮返工。",
        )
        prompt = (
            f"Issue #{self.state.issue_number} 的双轴审查未通过。"
            "请在原实现上下文中逐项修复下面的发现；不要忽略、弱化或仅解释问题。\n\n"
            f"{feedback}\n\n"
            "修复时继续遵守 ContextPack 中的全部规范和规格；只修改当前 worktree，"
            "不要提交或合并。完成后逐项说明修改位置。"
        )
        code, _, _, _ = self._run_codex_turn(
            prompt,
            f"Codex Driver · 审查返工第 {cycle} 轮",
            DRIVER_KEY,
            role=DRIVER_ROLE,
            turn_kind="review_repair",
        )
        if code != 0 or self.run.stop_event.is_set():
            return False

        self._status("targeted_validate", f"第 {cycle} 轮返工完成，执行针对性验证。")
        validation = self._run_validation_matrix(
            f"review-retest-{cycle}",
            f"返工验证执行器 · 第 {cycle} 轮",
            "retesting",
        )
        if validation.passed:
            return True

        self._status("driver_repair", f"第 {cycle} 轮验证失败，回传原 Driver。")
        code, _, _, _ = self._run_codex_turn(
            "审查返工后的验证矩阵仍然失败。请修复以下失败，不要扩大范围：\n\n"
            + validation.feedback(),
            f"Codex Driver · 针对性验证修复第 {cycle} 轮",
            DRIVER_KEY,
            role=DRIVER_ROLE,
            turn_kind="targeted_validation_repair",
        )
        if code != 0 or self.run.stop_event.is_set():
            return False
        self._status("targeted_validate", f"重新验证第 {cycle} 轮修复。")
        second_validation = self._run_validation_matrix(
            f"validation-retest-{cycle}",
            f"二次验证执行器 · 第 {cycle} 轮",
            "retesting",
        )
        return second_validation.passed

    @listen("REVIEW")
    def review(self) -> str:
        for cycle in range(self.state.max_review_cycles + 1):
            self.state.review_cycle = cycle
            self._status(
                "parallel_review",
                "启动 Matt 双轴审查：两个独立的只读 Codex 会话并行运行。",
            )
            if self.run.stop_event.is_set():
                self._status("stopped", "审查尚未启动，任务已停止。")
                return "STOPPED"
            _, results, process_failures = self._run_review_pair(cycle)
            if process_failures:
                self._status("failed", f"{'、'.join(process_failures)}执行失败。")
                return "FAILED"
            if results and all(result.passed for result in results.values()):
                reviewed = ChangeSet.capture(self.run.config.worktree_path)
                self.run.store.update_run(
                    self.run.run_id,
                    reviewed_fingerprint=reviewed.fingerprint,
                    reviewed_head=reviewed.head_sha,
                    reviewed_paths=json.dumps(reviewed.paths, ensure_ascii=False),
                    reviewed_diff_hash=reviewed.diff_hash,
                )
                self.run.log("双轴审查均为 PASS，等待人工批准提交。")
                return "AWAIT_APPROVAL"
            if not any(result.has_blocking_findings for result in results.values()):
                self._status(
                    "failed",
                    "审查未通过但没有结构化 P0/P1 发现，禁止让 Driver 盲目返工，已转人工处理。",
                )
                return "FAILED"
            if cycle >= self.state.max_review_cycles:
                self._status(
                    "failed",
                    f"双轴审查在 {self.state.max_review_cycles} 轮自动返工后仍未通过，转人工处理。",
                )
                return "FAILED"
            self._review_recheck_results = dict(results)
            if not self._repair_review_findings(results, cycle + 1):
                if self.run.stop_event.is_set():
                    self._status("stopped", "审查返工已停止。")
                    return "STOPPED"
                self._status("failed", "审查返工或返工验证失败。")
                return "FAILED"
        return "FAILED"

    @router(review, emit=["AWAIT_APPROVAL", "FAILED", "STOPPED"])
    def after_review(self, outcome: str) -> str:
        return outcome

    @listen("AWAIT_APPROVAL")
    def await_approval(self) -> None:
        self._status("awaiting_approval", "测试和双轴审查通过，等待人工批准提交。")


class PipelineManager:
    MAX_CONCURRENCY = 5
    ACTIVE_STATUSES = {
        "queued",
        "precheck",
        "driver_plan",
        "driver_implement",
        "driver_verify",
        "validate",
        "driver_repair",
        "targeted_validate",
        "parallel_review",
        "refactoring",
        "testing",
        "repairing",
        "retesting",
        "reviewing",
        "stopping",
        "session_running",
    }

    def __init__(
        self,
        store: StateStore | None = None,
        max_concurrency: int = 2,
        tool_preflight: ToolPreflight | None = None,
        enforce_preflight: bool = True,
    ):
        self.store = store or StateStore()
        self._lock = threading.RLock()
        self._runs: dict[str, PipelineRun] = {}
        self._session_calls: dict[str, threading.Thread] = {}
        self._session_clients: dict[str, AppServerClient] = {}
        self.tool_preflight = tool_preflight or ToolPreflight(local_bin=PROJECT_DIR / ".tools" / "bin")
        self.enforce_preflight = enforce_preflight
        self._max_concurrency = 1
        self.set_max_concurrency(max_concurrency)

    @property
    def max_concurrency(self) -> int:
        with self._lock:
            return self._max_concurrency

    def set_max_concurrency(self, value: int) -> int:
        value = int(value)
        if not 1 <= value <= self.MAX_CONCURRENCY:
            raise ValueError(f"最大并发数必须在 1 到 {self.MAX_CONCURRENCY} 之间。")
        with self._lock:
            self._max_concurrency = value
        return value

    def running_count(self) -> int:
        return len(self.active_run_rows())

    def active_run_rows(self) -> list[dict[str, Any]]:
        return [
            row
            for row in self.store.list_runs(200)
            if row.get("status") in self.ACTIVE_STATUSES
        ]

    def _allocate_slot(self) -> int:
        used = {
            int(row.get("port_slot") or 0)
            for row in self.active_run_rows()
            if row.get("port_slot") is not None
        }
        return next(slot for slot in range(self.MAX_CONCURRENCY) if slot not in used)

    def preflight(self, config: PipelineConfig) -> PreflightReport:
        if not self.enforce_preflight:
            return PreflightReport((), ())
        change_set = ChangeSet.capture(config.worktree_path)
        issue_text = f"{config.issue_title}\n{config.issue_body}\n{config.refactor_prompt}"
        return self.tool_preflight.check(
            changed_files=change_set.paths,
            issue_text=issue_text,
            has_backend=(config.worktree_path / "backend").is_dir(),
        )

    def start(self, config: PipelineConfig) -> PipelineRun:
        preflight = self.preflight(config)
        if not preflight.passed:
            raise RuntimeError(preflight.message())
        with self._lock:
            active_rows = self.active_run_rows()
            if len(active_rows) >= self._max_concurrency:
                raise RuntimeError(f"并发已满：{len(active_rows)}/{self._max_concurrency}。")
            if any(Path(row["worktree_path"]) == config.worktree_path for row in active_rows):
                raise RuntimeError(f"Issue #{config.issue_number} 的 worktree 已有流水线运行。")
            if any(int(row["issue_number"]) == config.issue_number for row in active_rows):
                raise RuntimeError(f"Issue #{config.issue_number} 已有 Driver Thread 运行。")
            if config.resource_lock and any(
                row.get("resource_lock") == config.resource_lock for row in active_rows
            ):
                raise RuntimeError(f"资源锁 {config.resource_lock} 正被其他流水线占用。")
            blocked = [
                row for row in self.store.list_runs(200)
                if Path(row["worktree_path"]) == config.worktree_path
                and row.get("status") == "awaiting_approval"
            ]
            if blocked:
                raise RuntimeError("该 worktree 正等待人工批准提交，不能启动新流水线。")
            sanitized_generated = sanitize_unrelated_generated_drift(config.worktree_path)
            run_id = self.store.create_run(
                {
                    "issue_number": config.issue_number,
                    "issue_title": config.issue_title,
                    "issue_body": config.issue_body,
                    "worktree_path": config.worktree_path,
                    "test_command": config.test_command,
                    "refactor_prompt": config.refactor_prompt,
                    "auto_approve": config.auto_approve,
                    "matt_workflow": config.matt_workflow,
                }
            )
            previous_driver = self.store.latest_driver_session(
                issue_number=config.issue_number,
                worktree_path=config.worktree_path,
            )
            if previous_driver:
                self.store.upsert_session(
                    run_id,
                    DRIVER_KEY,
                    "Codex Driver",
                    thread_id=previous_driver.get("thread_id"),
                    turn_id=previous_driver.get("turn_id"),
                    status="queued",
                    role=DRIVER_ROLE,
                    read_only=False,
                    model=previous_driver.get("model"),
                    model_provider=previous_driver.get("model_provider"),
                    reasoning_effort=previous_driver.get("reasoning_effort"),
                )
            environment = RuntimeEnvironment.create(
                run_id=run_id,
                issue_number=config.issue_number,
                worktree_path=config.worktree_path,
                slot=self._allocate_slot(),
            )
            self.store.update_run(
                run_id,
                compose_project=environment.compose_project,
                port_slot=environment.slot,
                resource_lock=config.resource_lock,
            )
            run = PipelineRun(config, self.store, run_id, environment)
            run.set_status("precheck")
            run.log("PRECHECK 通过，准备启动唯一 Codex Driver Thread。")
            if sanitized_generated:
                run.log(
                    "[生成预检] 已恢复与当前 Issue 无关的生成漂移："
                    + ", ".join(sanitized_generated)
                )
            thread = threading.Thread(
                target=self._execute,
                args=(run,),
                name=f"pipeline-issue-{config.issue_number}",
                daemon=True,
            )
            run.thread = thread
            self._runs[run_id] = run
            thread.start()
            return run

    def _execute(self, run: PipelineRun) -> None:
        try:
            IssuePipelineFlow(run).kickoff()
            if run.status not in {"awaiting_approval", "completed", "failed", "stopped"}:
                run.set_status("failed")
                run.log("流水线在未知状态结束，已按失败处理。")
        except Exception as exc:  # pragma: no cover
            run.error = repr(exc)
            run.set_status("failed")
            run.log(f"流水线异常：{exc!r}")
        finally:
            run.finished_at = datetime.now(timezone.utc)
            self.store.update_run(
                run.run_id,
                finished_at=run.finished_at.isoformat(),
                status=run.status,
            )
            if run.stop_event.is_set():
                run.request_stop()

    def stop(self, run_id: str) -> None:
        with self._lock:
            run = self._runs.get(run_id)
        if not run:
            raise RuntimeError("运行实例不存在或控制台已经重启。")
        run.request_stop()
        run.set_status("stopping")
        run.log("已请求停止当前流水线。")

    def snapshots(self) -> list[dict[str, object]]:
        with self._lock:
            snapshots = [run.snapshot() for run in self._runs.values()]
        return sorted(snapshots, key=lambda item: item["started_at"], reverse=True)

    def _database_snapshot(self, row: dict[str, Any]) -> dict[str, object]:
        sessions = self.store.sessions_for_run(str(row["id"]))
        agents = {
            str(session["agent_key"]): {
                "label": session["label"],
                "status": session["status"],
                "command": session["command"],
                "thread_id": session["thread_id"],
                "turn_id": session["turn_id"],
                "model": session.get("model"),
                "model_provider": session.get("model_provider"),
                "reasoning_effort": session.get("reasoning_effort"),
                "role": session.get("role"),
                "read_only": bool(session.get("read_only")),
            }
            for session in sessions
        }
        running_sessions = [session for session in sessions if session["status"] == "running"]
        current_session = running_sessions[-1] if running_sessions else (sessions[-1] if sessions else None)
        try:
            started_at = datetime.fromisoformat(str(row["created_at"]))
        except ValueError:
            started_at = datetime.now(timezone.utc)
        reports: dict[str, str] = {}
        for session in sessions:
            key = str(session["agent_key"])
            if key.startswith("review-standards") and session.get("report"):
                reports["standards"] = session["report"]
            elif key.startswith("review-spec") and session.get("report"):
                reports["spec"] = session["report"]
        return {
            "run_id": row["id"],
            "run_kind": row.get("run_kind") or "pipeline",
            "parent_run_id": row.get("parent_run_id"),
            "status": row["status"],
            "logs": self.store.logs_for_run(str(row["id"])),
            "started_at": started_at,
            "finished_at": row.get("finished_at"),
            "error": row.get("error"),
            "review_reports": reports,
            "review_results": {},
            "validation_results": [],
            "activities": self.store.events_for_run(str(row["id"])),
            "agents": agents,
            "current_agent": current_session["agent_key"] if current_session else None,
            "current_command": current_session["command"] if current_session else None,
            "model_info": {
                "model": (current_session or {}).get("model") or row.get("model") or "",
                "model_provider": (current_session or {}).get("model_provider") or row.get("model_provider") or "",
                "reasoning_effort": (current_session or {}).get("reasoning_effort") or row.get("reasoning_effort") or "",
            },
            "config": SimpleNamespace(
                issue_number=row["issue_number"],
                issue_title=row["issue_title"],
                issue_body=row["issue_body"],
                worktree_path=Path(row["worktree_path"]),
                matt_workflow=bool(row["matt_workflow"]),
                resource_lock=row.get("resource_lock") or "",
            ),
            "running": row["status"] in self.ACTIVE_STATUSES,
            "attached": str(row["id"]) in self._runs
            or str(row["id"]) in self._session_calls,
        }

    def workbench_snapshots(self) -> list[dict[str, object]]:
        rows = [
            row
            for row in self.store.list_runs(200)
            if row.get("status") in self.ACTIVE_STATUSES
            or row.get("status") in {"awaiting_approval", "stale_review"}
        ]
        with self._lock:
            memory = {snapshot["run_id"]: snapshot for snapshot in self.snapshots()}
        snapshots: list[dict[str, object]] = []
        for row in rows:
            run_id = str(row["id"])
            snapshot = memory.get(run_id) or self._database_snapshot(row)
            snapshot.setdefault("run_kind", row.get("run_kind") or "pipeline")
            snapshot.setdefault("attached", run_id in memory)
            snapshots.append(snapshot)
        return sorted(snapshots, key=lambda item: item["started_at"], reverse=True)

    def snapshot(self, run_id: str | None = None) -> dict[str, object] | None:
        snapshots = self.snapshots()
        if run_id is None:
            return snapshots[0] if snapshots else None
        return next((item for item in snapshots if item["run_id"] == run_id), None)

    def list_runs(self) -> list[dict[str, Any]]:
        return self.store.list_runs()

    def approve_commit(self, run_id: str) -> str:
        run = self.store.get_run(run_id)
        if not run:
            raise RuntimeError("运行记录不存在。")
        if run["status"] != "awaiting_approval":
            raise RuntimeError("只有测试和双轴审查通过的运行才能提交。")
        worktree = Path(run["worktree_path"])
        if any(
            row["id"] != run_id
            and Path(row["worktree_path"]) == worktree
            for row in self.active_run_rows()
        ):
            raise RuntimeError("该 worktree 正有 Driver 写入，不能同时批准提交。")
        reviewed_fingerprint = run.get("reviewed_fingerprint")
        reviewed_head = run.get("reviewed_head")
        reviewed_diff_hash = run.get("reviewed_diff_hash")
        try:
            reviewed_paths = tuple(json.loads(run.get("reviewed_paths") or "[]"))
        except (json.JSONDecodeError, TypeError):
            reviewed_paths = ()
        current = ChangeSet.capture(worktree)
        review_is_current = (
            bool(reviewed_fingerprint)
            and current.fingerprint == reviewed_fingerprint
            and current.head_sha == reviewed_head
            and current.paths == reviewed_paths
            and current.diff_hash == reviewed_diff_hash
        )
        if not review_is_current:
            self.store.update_run(
                run_id,
                status="stale_review",
                current_stage="stale_review",
            )
            self.store.append_log(
                run_id,
                "[提交保护] 审查完成后工作区发生变化，已取消提交并要求重新验证。",
            )
            with self._lock:
                active = self._runs.get(run_id)
            if active:
                active.set_status("stale_review")
                active.log("审查完成后工作区发生变化，必须重新运行验证和审查。")
            raise RuntimeError("审查后发生变化，已取消提交；请重新运行验证和双轴审查。")
        status = subprocess.run(
            ["git", "status", "--short"], cwd=worktree, text=True,
            capture_output=True, check=False, timeout=30,
        )
        if status.returncode != 0:
            raise RuntimeError(status.stderr.strip() or "读取 Git 状态失败。")
        if not status.stdout.strip():
            raise RuntimeError("当前 worktree 没有可提交的修改。")
        add = subprocess.run(
            ["git", "add", "-A", "--", *reviewed_paths], cwd=worktree, text=True,
            capture_output=True, check=False, timeout=30,
        )
        if add.returncode != 0:
            raise RuntimeError(add.stderr.strip() or "Git add 失败。")
        staged = ChangeSet.capture(worktree)
        staged_paths_result = subprocess.run(
            ["git", "diff", "--cached", "--name-only", "-z"],
            cwd=worktree,
            capture_output=True,
            check=False,
            timeout=30,
        )
        staged_paths = ChangeSet._split_paths(staged_paths_result.stdout)
        if (
            staged_paths_result.returncode != 0
            or staged.fingerprint != reviewed_fingerprint
            or tuple(sorted(staged_paths)) != tuple(sorted(reviewed_paths))
        ):
            self.store.update_run(
                run_id,
                status="stale_review",
                current_stage="stale_review",
            )
            self.store.append_log(
                run_id,
                "[提交保护] 暂存期间工作区发生变化，已取消提交并要求重新验证。",
            )
            raise RuntimeError("暂存期间工作区发生变化，已取消提交；请重新验证和审查。")
        message = f"feat: complete issue #{run['issue_number']}"
        commit = subprocess.run(
            ["git", "commit", "-m", message], cwd=worktree, text=True,
            capture_output=True, check=False, timeout=120,
        )
        if commit.returncode != 0:
            raise RuntimeError(commit.stderr.strip() or commit.stdout.strip() or "Git commit 失败。")
        sha = subprocess.run(
            ["git", "rev-parse", "--short", "HEAD"], cwd=worktree,
            text=True, capture_output=True, check=True, timeout=30,
        ).stdout.strip()
        self.store.update_run(
            run_id,
            status="completed",
            current_stage="completed",
            commit_sha=sha,
        )
        self.store.append_log(run_id, f"[人工批准] 已提交 {sha}：{message}")
        with self._lock:
            active = self._runs.get(run_id)
        if active:
            active.set_status("completed")
            active.log(f"人工批准完成，已提交 {sha}。")
        return sha

    def cleanup_environment(self, run_id: str, *, remove_volumes: bool) -> str:
        with self._lock:
            active = self._runs.get(run_id)
        if active and active.snapshot()["running"]:
            raise RuntimeError("流水线仍在运行，不能清理运行环境。")
        row = self.store.get_run(run_id)
        if not row:
            raise RuntimeError("运行记录不存在。")
        environment = active.runtime_environment if active else RuntimeEnvironment.create(
            run_id=run_id,
            issue_number=int(row["issue_number"]),
            worktree_path=Path(row["worktree_path"]),
            slot=int(row.get("port_slot") or 0),
        )
        result = environment.cleanup(remove_volumes=remove_volumes)
        if result.returncode != 0:
            raise RuntimeError(result.output)
        self.store.append_log(run_id, "[环境清理] " + (result.output or "Compose 环境已清理。"))
        return result.output or "Compose 环境已清理。"

    def run_history(self, run_id: str) -> dict[str, Any] | None:
        run = self.store.get_run(run_id)
        if not run:
            return None
        return {
            "run": run,
            "logs": self.store.logs_for_run(run_id),
            "events": self.store.events_for_run(run_id),
            "sessions": [
                session for session in self.store.list_sessions(500)
                if session.get("run_id") == run_id
            ],
            "turns": self.store.turns_for_run(run_id),
        }

    def list_sessions(self) -> list[dict[str, Any]]:
        return self.store.list_driver_sessions()

    def list_all_driver_sessions(self) -> list[dict[str, Any]]:
        return self.store.list_driver_sessions(include_archived=True)

    def archive_session(self, session_id: int) -> None:
        session = self.store.get_session_by_id(session_id)
        if not session or not session.get("thread_id"):
            raise RuntimeError("会话不存在或还没有 threadId。")
        if session.get("role") != DRIVER_ROLE or bool(session.get("read_only")):
            raise RuntimeError("只允许归档 Driver 会话。")
        self.store.archive_sessions_by_thread(str(session["thread_id"]), True)

    def reject_run(self, run_id: str) -> None:
        run = self.store.get_run(run_id)
        if not run:
            raise RuntimeError("运行记录不存在。")
        if run.get("status") not in {"awaiting_approval", "stale_review"}:
            raise RuntimeError("只有等待人工决策或审查失效的运行可以拒绝。")
        self.store.update_run(
            run_id,
            status="rejected",
            current_stage="rejected",
            finished_at=run.get("finished_at") or datetime.now(timezone.utc).isoformat(),
        )
        self.store.append_log(run_id, "[人工决策] 已拒绝本次变更；Worktree 和代码保持不变。")

    def rerun(self, run_id: str) -> PipelineRun:
        run = self.store.get_run(run_id)
        if not run:
            raise RuntimeError("运行记录不存在。")
        return self.start(
            PipelineConfig(
                issue_number=int(run["issue_number"]),
                issue_title=str(run["issue_title"]),
                issue_body=str(run.get("issue_body") or ""),
                worktree_path=Path(str(run["worktree_path"])),
                test_command=str(run.get("test_command") or "cd backend && go test ./..."),
                refactor_prompt=str(run.get("refactor_prompt") or ""),
                auto_approve=bool(run.get("auto_approve")),
                matt_workflow=bool(run.get("matt_workflow")),
                resource_lock=str(run.get("resource_lock") or ""),
            )
        )

    def request_review_repair(self, run_id: str, prompt: str) -> PipelineRun:
        run = self.store.get_run(run_id)
        if not run:
            raise RuntimeError("运行记录不存在。")
        if run.get("status") not in {"awaiting_approval", "stale_review"}:
            raise RuntimeError("只有等待人工决策或审查失效的运行可以请求返工。")
        self.store.update_run(
            run_id,
            status="stale_review",
            current_stage="stale_review",
        )
        self.store.append_log(
            run_id,
            "[人工决策] 已请求原 Driver 小范围返工；原审查快照失效。",
        )
        return self.start(
            PipelineConfig(
                issue_number=int(run["issue_number"]),
                issue_title=str(run["issue_title"]),
                issue_body=str(run.get("issue_body") or ""),
                worktree_path=Path(str(run["worktree_path"])),
                test_command=str(run.get("test_command") or "cd backend && go test ./..."),
                refactor_prompt=prompt.strip() or "根据人工审查意见做小范围返工并重新验证。",
                auto_approve=bool(run.get("auto_approve")),
                matt_workflow=bool(run.get("matt_workflow")),
                resource_lock=str(run.get("resource_lock") or ""),
            )
        )

    @staticmethod
    def _command(
        args: list[str],
        *,
        cwd: Path | None = None,
        timeout: int = 120,
    ) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            args,
            cwd=cwd,
            text=True,
            capture_output=True,
            check=False,
            timeout=timeout,
        )

    def _repository_slug(self, worktree: Path) -> str:
        result = self._command(["git", "remote", "get-url", "origin"], cwd=worktree)
        if result.returncode != 0:
            raise RuntimeError(result.stderr.strip() or "无法读取 origin 远程仓库。")
        remote = result.stdout.strip()
        if remote.startswith("git@"):
            remote = remote.split(":", 1)[-1]
        else:
            from urllib.parse import urlparse

            remote = urlparse(remote).path.lstrip("/")
        return remote.removesuffix(".git")

    def _pr_view(self, run: dict[str, Any]) -> dict[str, Any] | None:
        pr_number = run.get("pr_number")
        if not pr_number:
            return None
        result = self._command(
            [
                "gh", "pr", "view", str(pr_number),
                "--repo", self._repository_slug(Path(run["worktree_path"])),
                "--json",
                "number,url,state,baseRefName,headRefName,headRefOid,mergeStateStatus,statusCheckRollup,mergedAt,mergeCommit",
            ],
            cwd=Path(run["worktree_path"]),
        )
        if result.returncode != 0:
            raise RuntimeError(result.stderr.strip() or "读取 GitHub PR 状态失败。")
        return json.loads(result.stdout)

    def create_pull_request(self, run_id: str) -> dict[str, Any]:
        run = self.store.get_run(run_id)
        if not run:
            raise RuntimeError("运行记录不存在。")
        if run.get("status") != "completed" or not run.get("commit_sha"):
            raise RuntimeError("只有人工批准并已创建提交的运行才能创建 PR。")
        worktree = Path(run["worktree_path"])
        branch_result = self._command(["git", "branch", "--show-current"], cwd=worktree)
        head_result = self._command(["git", "rev-parse", "HEAD"], cwd=worktree)
        status_result = self._command(["git", "status", "--porcelain"], cwd=worktree)
        if branch_result.returncode or not branch_result.stdout.strip():
            raise RuntimeError("无法确定 Issue 分支。")
        if head_result.returncode or not head_result.stdout.strip().startswith(str(run["commit_sha"])):
            self.store.update_run(run_id, status="stale_review", current_stage="stale_review")
            raise RuntimeError("本地 HEAD 已偏离批准提交，禁止创建 PR；请重新验证和审查。")
        if status_result.returncode != 0 or status_result.stdout.strip():
            raise RuntimeError("Worktree 不是干净状态，禁止创建 PR。")
        if run.get("pr_number") and run.get("pr_url"):
            return {
                "number": run["pr_number"],
                "url": run["pr_url"],
                "state": "OPEN",
            }
        repo = self._repository_slug(worktree)
        branch = branch_result.stdout.strip()
        if branch in {"master", "main"}:
            raise RuntimeError("当前 worktree 不在 Issue 分支，禁止创建 PR。")
        push = self._command(
            ["git", "push", "--set-upstream", "origin", branch],
            cwd=worktree,
            timeout=180,
        )
        if push.returncode != 0:
            raise RuntimeError(push.stderr.strip() or "推送 Issue 分支到 origin 失败。")
        existing = self._command(
            [
                "gh", "pr", "list", "--repo", repo, "--head", branch,
                "--base", "master", "--state", "all", "--json", "number,url,state",
            ],
            cwd=worktree,
        )
        if existing.returncode != 0:
            raise RuntimeError(existing.stderr.strip() or "查询现有 PR 失败。")
        existing_prs = json.loads(existing.stdout or "[]")
        open_prs = [item for item in existing_prs if str(item.get("state") or "").upper() == "OPEN"]
        if open_prs:
            pr = open_prs[0]
        else:
            title = f"{run['issue_title']} (#{run['issue_number']})"
            body = (
                f"## Issue\n\nImplements #{run['issue_number']}\n\n"
                "## Verification\n\n"
                "- Codex Driver 已完成实现和人工审查门禁。\n"
                "- Standards/Spec 只读审查已通过。\n"
                "- 提交由 CrewOps 人工批准后创建。\n"
            )
            created = self._command(
                [
                    "gh", "pr", "create", "--repo", repo,
                    "--base", "master", "--head", branch,
                    "--title", title, "--body", body,
                ],
                cwd=worktree,
            )
            if created.returncode != 0:
                raise RuntimeError(created.stderr.strip() or "创建 GitHub PR 失败。")
            pr_url = created.stdout.strip().splitlines()[-1]
            viewed = self._command(
                ["gh", "pr", "view", pr_url, "--repo", repo, "--json", "number,url,state"],
                cwd=worktree,
            )
            if viewed.returncode != 0:
                raise RuntimeError(viewed.stderr.strip() or "读取新建 PR 失败。")
            pr = json.loads(viewed.stdout)
        self.store.update_run(
            run_id,
            pr_number=int(pr["number"]),
            pr_url=str(pr["url"]),
            pr_created_at=self.store._now(),
        )
        self.store.append_log(run_id, f"[GitHub] 已创建 PR #{pr['number']}：{pr['url']}")
        return pr

    def pull_request(self, run_id: str) -> dict[str, Any] | None:
        run = self.store.get_run(run_id)
        if not run:
            raise RuntimeError("运行记录不存在。")
        return self._pr_view(run)

    @staticmethod
    def _checks_passed(checks: list[dict[str, Any]] | None) -> tuple[bool, str]:
        pending = {"QUEUED", "IN_PROGRESS", "PENDING", "EXPECTED", "REQUESTED"}
        failed = {"FAILURE", "FAILED", "CANCELLED", "TIMED_OUT", "ACTION_REQUIRED", "ERROR"}
        for check in checks or []:
            status = str(check.get("status") or check.get("state") or "").upper()
            conclusion = str(check.get("conclusion") or "").upper()
            if status in pending:
                return False, "仍有 GitHub 检查正在运行。"
            if status in failed or conclusion in failed:
                return False, f"GitHub 检查未通过：{check.get('name') or '未命名检查'}。"
        return True, ""

    def merge_pull_request(self, run_id: str) -> dict[str, Any]:
        run = self.store.get_run(run_id)
        if not run:
            raise RuntimeError("运行记录不存在。")
        if not run.get("pr_number"):
            raise RuntimeError("请先创建 PR。")
        pr = self._pr_view(run)
        if not pr:
            raise RuntimeError("PR 状态不存在。")
        if str(pr.get("baseRefName")) != "master":
            raise RuntimeError("PR 目标分支不是远程 master，禁止合并。")
        if str(pr.get("state")).upper() == "MERGED":
            merged = pr.get("mergeCommit") or {}
            merge_sha = merged.get("oid") if isinstance(merged, dict) else str(merged or "")
            self.store.update_run(
                run_id,
                status="merged",
                current_stage="merged",
                merged_at=pr.get("mergedAt") or self.store._now(),
                merge_commit_sha=merge_sha,
            )
            return pr
        if str(pr.get("state")).upper() != "OPEN":
            raise RuntimeError(f"PR 当前状态为 {pr.get('state')}，不能合并。")
        local_head = self._command(["git", "rev-parse", "HEAD"], cwd=Path(run["worktree_path"]))
        if local_head.returncode != 0 or not str(pr.get("headRefOid") or "").startswith(local_head.stdout.strip()):
            self.store.update_run(run_id, status="stale_review", current_stage="stale_review")
            raise RuntimeError("PR head 已偏离本地批准提交，禁止合并。")
        checks_ok, reason = self._checks_passed(pr.get("statusCheckRollup"))
        if not checks_ok:
            raise RuntimeError(reason)
        if str(pr.get("mergeStateStatus") or "").upper() not in {"CLEAN", "UNSTABLE"}:
            raise RuntimeError(f"PR 当前不可合并：{pr.get('mergeStateStatus') or '未知状态'}。")
        repo = self._repository_slug(Path(run["worktree_path"]))
        merged = self._command(
            [
                "gh", "pr", "merge", str(run["pr_number"]), "--repo", repo,
                "--merge", "--match-head-commit", str(pr["headRefOid"]),
            ],
            cwd=Path(run["worktree_path"]),
            timeout=180,
        )
        if merged.returncode != 0:
            raise RuntimeError(merged.stderr.strip() or merged.stdout.strip() or "远程 PR 合并失败。")
        final = self._pr_view(run) or pr
        merge_commit = final.get("mergeCommit") or {}
        merge_sha = merge_commit.get("oid") if isinstance(merge_commit, dict) else str(merge_commit or "")
        self.store.update_run(
            run_id,
            status="merged",
            current_stage="merged",
            merged_at=final.get("mergedAt") or self.store._now(),
            merge_commit_sha=merge_sha,
        )
        self.store.append_log(run_id, f"[GitHub] PR #{run['pr_number']} 已合并到远程 master。")
        return final

    def call_session(self, session_id: int, prompt: str) -> str:
        session = self.store.get_session_by_id(session_id)
        if not session or not session.get("thread_id"):
            raise RuntimeError("会话不存在或还没有 threadId。")
        if session.get("role") != DRIVER_ROLE or bool(session.get("read_only")):
            raise RuntimeError("只允许调用/继续当前 Issue 的 Codex Driver 会话。")
        if any(
            thread.is_alive()
            for run_id, thread in self._session_calls.items()
            if self.store.get_run(run_id)
            and self.store.get_run(run_id).get("parent_run_id") == session["run_id"]
        ):
            raise RuntimeError("这个会话已经有调用正在执行。")
        run = self.store.get_run(session["run_id"])
        if not run:
            raise RuntimeError("会话所属流水线不存在。")
        active_rows = self.active_run_rows()
        if len(active_rows) >= self.max_concurrency:
            raise RuntimeError(f"并发已满：{len(active_rows)}/{self.max_concurrency}。")
        if any(Path(row["worktree_path"]) == Path(run["worktree_path"]) for row in active_rows):
            raise RuntimeError("该 worktree 有任务正在运行，不能同时手动调用会话。")
        if any(int(row["issue_number"]) == int(run["issue_number"]) for row in active_rows):
            raise RuntimeError(f"Issue #{run['issue_number']} 已有 Driver Thread 运行。")
        if run.get("resource_lock") and any(
            row.get("resource_lock") == run.get("resource_lock") for row in active_rows
        ):
            raise RuntimeError(f"资源锁 {run['resource_lock']} 正被其他任务占用。")
        continuation_run_id = self.store.create_run(
            {
                "issue_number": run["issue_number"],
                "issue_title": f"{run['issue_title']} · 会话继续",
                "issue_body": run["issue_body"],
                "worktree_path": run["worktree_path"],
                "test_command": "",
                "refactor_prompt": prompt,
                "auto_approve": bool(run["auto_approve"]),
                "matt_workflow": False,
                "run_kind": "session",
                "parent_run_id": run["id"],
            }
        )
        self.store.update_run(
            continuation_run_id,
            status="session_running",
            current_stage="session_running",
            compose_project=run.get("compose_project"),
            port_slot=run.get("port_slot"),
            resource_lock=run.get("resource_lock"),
        )
        if run.get("status") == "awaiting_approval":
            self.store.update_run(
                str(run["id"]),
                status="stale_review",
                current_stage="stale_review",
            )
            self.store.append_log(
                str(run["id"]),
                "[会话继续] Driver 将继续修改代码，原审查结果已立即失效。",
            )
        thread = threading.Thread(
            target=self._call_session_worker,
            args=(continuation_run_id, session, run, prompt),
            name=f"session-run-{continuation_run_id[:8]}",
            daemon=True,
        )
        self._session_calls[continuation_run_id] = thread
        thread.start()
        return continuation_run_id

    def _call_session_worker(
        self,
        continuation_run_id: str,
        session: dict[str, Any],
        run: dict[str, Any],
        prompt: str,
    ) -> None:
        session_id = int(session["id"])
        agent_key = str(session["agent_key"])
        label = str(session["label"])
        store = self.store
        continuation_key = f"session-{agent_key}"
        continuation_label = f"{label} · 继续"
        store.upsert_session(
            continuation_run_id, continuation_key, continuation_label,
            thread_id=session.get("thread_id"),
            turn_id=session.get("turn_id"), status="running",
            command="App Server thread/resume → turn/start", report=session.get("report", ""),
            role="driver_continuation", read_only=False,
        )
        store.append_log(continuation_run_id, f"[会话继续] 开始调用 {label}（session #{session_id}）。")
        def session_event(message: dict[str, Any]) -> None:
            method = str(message.get("method", "event"))
            params = message.get("params") or {}
            text = params.get("delta") or params.get("text") or params.get("explanation") or ""
            if method == "model/rerouted" and params.get("toModel"):
                store.update_run(continuation_run_id, model=params["toModel"])
                store.upsert_session(
                    continuation_run_id,
                    continuation_key,
                    continuation_label,
                    status="running",
                    command="App Server thread/resume → turn/start",
                    model=params["toModel"],
                )
            elif method == "item/reasoning/summaryTextDelta" and isinstance(text, str):
                store.append_event(continuation_run_id, continuation_key, continuation_label, "reasoning", text)
            elif method == "item/agentMessage/delta" and isinstance(text, str):
                store.append_event(continuation_run_id, continuation_key, continuation_label, "commentary", text)
            elif method == "turn/plan/updated":
                store.append_event(continuation_run_id, continuation_key, continuation_label, "plan", json.dumps(params.get("plan") or [], ensure_ascii=False))
            else:
                store.append_log(continuation_run_id, f"[{label}] {method} {json.dumps(params, ensure_ascii=False)[:500]}")
        env = RuntimeEnvironment.create(
            run_id=str(run["id"]), issue_number=int(run["issue_number"]),
            worktree_path=Path(run["worktree_path"]), slot=int(run.get("port_slot") or 0),
        )
        client = AppServerClient(session_event, process_env=env.process_env())
        with self._lock:
            self._session_clients[continuation_run_id] = client
        try:
            thread_id, turn_id, code = client.run_turn(
                cwd=Path(run["worktree_path"]), prompt=prompt,
                thread_id=str(session["thread_id"]), read_only=False,
                auto_approve=bool(run["auto_approve"]),
                on_thread_info=lambda info: (
                    store.update_run(
                        continuation_run_id,
                        model=info.get("model"),
                        model_provider=info.get("model_provider"),
                        reasoning_effort=info.get("reasoning_effort"),
                    ),
                    store.upsert_session(
                        continuation_run_id,
                        continuation_key,
                        continuation_label,
                        status="running",
                        command="App Server thread/resume → turn/start",
                        model=info.get("model"),
                        model_provider=info.get("model_provider"),
                        reasoning_effort=info.get("reasoning_effort"),
                    ),
                ),
            )
            store.upsert_session(
                continuation_run_id, continuation_key, continuation_label, thread_id=thread_id,
                turn_id=turn_id, status="succeeded" if code == 0 else "failed",
                command="App Server thread/resume → turn/start",
                role="driver_continuation", read_only=False,
            )
            store.start_turn(
                run_id=continuation_run_id,
                thread_id=thread_id,
                turn_id=turn_id,
                turn_kind="manual_driver_continue",
                role=DRIVER_ROLE,
                read_only=False,
                prompt_summary=prompt[:500],
                session_key=DRIVER_KEY,
            )
            store.finish_turn(
                run_id=continuation_run_id,
                turn_id=turn_id,
                status="succeeded" if code == 0 else "failed",
            )
            final_status = "completed" if code == 0 else "failed"
            store.update_run(
                continuation_run_id,
                status=final_status,
                current_stage=final_status,
                finished_at=datetime.now(timezone.utc).isoformat(),
            )
            store.append_log(continuation_run_id, f"[会话继续] {label}结束，退出码：{code}。")
        except Exception as exc:  # pragma: no cover
            store.upsert_session(
                continuation_run_id, continuation_key, continuation_label,
                thread_id=session.get("thread_id"),
                turn_id=session.get("turn_id"), status="failed",
                command="App Server thread/resume → turn/start",
            )
            store.update_run(
                continuation_run_id,
                status="failed",
                current_stage="failed",
                error=str(exc),
                finished_at=datetime.now(timezone.utc).isoformat(),
            )
            store.append_log(continuation_run_id, f"[会话继续] {label}失败：{exc}")
        finally:
            client.close()
            with self._lock:
                self._session_clients.pop(continuation_run_id, None)
                self._session_calls.pop(continuation_run_id, None)

    def mark_interrupted(self, run_id: str) -> None:
        with self._lock:
            active = self._runs.get(run_id)
            client = self._session_clients.get(run_id)
        if active:
            self.stop(run_id)
            return
        if client:
            client.close()
        if not active and not client:
            raise RuntimeError("该任务由热更新前的后台进程管理，当前页面无法安全停止。")
        row = self.store.get_run(run_id)
        if not row:
            raise RuntimeError("运行记录不存在。")
        self.store.update_run(
            run_id,
            status="stopped",
            current_stage="stopped",
            finished_at=datetime.now(timezone.utc).isoformat(),
        )
        self.store.finish_running_sessions(run_id, "stopped")
        self.store.append_log(run_id, "[人工操作] 已标记为停止。")

    def delete_session(self, session_id: int) -> None:
        session = self.store.get_session_by_id(session_id)
        if not session:
            raise RuntimeError("会话不存在。")
        if session.get("role") != DRIVER_ROLE or bool(session.get("read_only")):
            raise RuntimeError("只允许删除 Driver 会话；审查 Sidecar 由运行历史管理。")
        if session_id in self._session_calls and self._session_calls[session_id].is_alive():
            raise RuntimeError("会话正在调用，不能删除。")
        thread_id = session.get("thread_id")
        if thread_id:
            client = AppServerClient()
            try:
                client.delete_thread(str(thread_id))
            finally:
                client.close()
        if thread_id:
            self.store.delete_sessions_by_thread(str(thread_id))
        else:
            self.store.delete_session(session_id)


# Process-wide singleton shared by the local CrewOps API server. Pipeline/run
# history is persisted by StateStore; the singleton owns current workers only.
_MANAGER = PipelineManager(max_concurrency=2)


def pipeline_manager() -> PipelineManager:
    """Return the process-wide pipeline manager used by the CrewOps API."""
    return _MANAGER
