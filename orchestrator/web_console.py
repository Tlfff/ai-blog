from __future__ import annotations

import json
import os
import re
import subprocess
from dataclasses import asdict, is_dataclass
from datetime import datetime
from http.server import SimpleHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

from pipeline import PipelineConfig, pipeline_manager
from reliability import WorktreeManager
from workflow import ChangeSet, ReviewPackBuilder

ROOT = Path(__file__).resolve().parents[1]
WEB_ROOT = Path(__file__).resolve().parent / "web"
WORKTREE_ROOT = ROOT / ".worktrees"


def normalize(value: Any) -> Any:
    if is_dataclass(value):
        return normalize(asdict(value))
    if isinstance(value, Path):
        return str(value)
    if isinstance(value, datetime):
        return value.isoformat()
    if isinstance(value, dict):
        return {str(key): normalize(item) for key, item in value.items()}
    if isinstance(value, (list, tuple, set)):
        return [normalize(item) for item in value]
    if hasattr(value, "model_dump"):
        return normalize(value.model_dump())
    return value


def command(args: list[str], cwd: Path = ROOT) -> subprocess.CompletedProcess[str]:
    return subprocess.run(args, cwd=cwd, text=True, capture_output=True, timeout=30, check=False)


def repository_name() -> str:
    result = command(["git", "remote", "get-url", "origin"])
    if result.returncode:
        return ROOT.name
    remote = result.stdout.strip()
    if remote.startswith("git@"):
        remote = remote.split(":", 1)[1]
    else:
        remote = urlparse(remote).path.lstrip("/")
    return remote.removesuffix(".git")


def load_issues() -> list[dict[str, Any]]:
    repo = repository_name()
    result = command([
        "gh", "issue", "list", "--repo", repo, "--state", "open", "--limit", "100",
        "--json", "number,title,body,labels,assignees,updatedAt",
    ])
    if result.returncode:
        return []
    return json.loads(result.stdout)


def worktrees() -> dict[int, dict[str, str]]:
    result = command(["git", "worktree", "list", "--porcelain"])
    found: dict[int, dict[str, str]] = {}
    current: dict[str, str] = {}
    for line in result.stdout.splitlines() + [""]:
        if line.startswith("worktree "):
            current = {"path": line[9:]}
        elif line.startswith("branch "):
            current["branch"] = line[7:].removeprefix("refs/heads/")
        elif not line and current:
            match = re.search(r"issue-(\d+)", current.get("path", ""))
            if match:
                found[int(match.group(1))] = current
            current = {}
    return found


def change_stats(path: str) -> dict[str, Any]:
    root = Path(path)
    if not root.is_dir():
        return {"files": 0, "untracked": 0, "additions": 0, "deletions": 0}
    change_set = ChangeSet.capture(root)
    status = command(["git", "status", "--porcelain"], root)
    numstat = command(["git", "diff", "--numstat", "HEAD"], root)
    files = [line for line in status.stdout.splitlines() if line.strip()]
    additions = deletions = 0
    for line in numstat.stdout.splitlines():
        parts = line.split("\t")
        if len(parts) >= 2:
            additions += int(parts[0]) if parts[0].isdigit() else 0
            deletions += int(parts[1]) if parts[1].isdigit() else 0
    return {
        "files": len(files), "untracked": sum(line.startswith("??") for line in files),
        "additions": additions, "deletions": deletions,
        "paths": list(change_set.paths),
        "trackedPaths": list(change_set.tracked_paths),
        "untrackedPaths": list(change_set.untracked_paths),
        "generatedPaths": [
            path for path in change_set.paths
            if path.endswith(ReviewPackBuilder.GENERATED_SUFFIXES)
        ],
        "head": change_set.head_sha,
        "diffHash": change_set.diff_hash,
    }


def bootstrap() -> dict[str, Any]:
    manager = pipeline_manager()
    runs = manager.list_runs()
    sessions = manager.list_sessions()
    all_sessions = manager.list_all_driver_sessions()
    active = manager.workbench_snapshots()
    issues = load_issues()
    trees = worktrees()
    return normalize({
        "repository": repository_name(), "maxConcurrency": manager.max_concurrency,
        "running": manager.running_count(), "runs": runs, "active": active,
        "sessions": sessions, "archivedSessions": [row for row in all_sessions if row.get("archived")],
        "issues": issues, "worktrees": trees,
        "reviewRuns": [
            row for row in runs
            if row.get("reviewed_fingerprint")
            or row.get("status") in {"awaiting_approval", "stale_review", "rejected"}
        ],
        "reviewPending": sum(
            row.get("status") in {"awaiting_approval", "stale_review"} for row in runs
        ),
    })


def run_detail(run_id: str) -> dict[str, Any]:
    manager = pipeline_manager()
    history = manager.run_history(run_id)
    if not history:
        raise ValueError("运行记录不存在")
    history["stats"] = change_stats(history["run"]["worktree_path"])
    if history["run"].get("pr_number"):
        try:
            history["pullRequest"] = manager.pull_request(run_id)
        except RuntimeError as exc:
            history["pullRequestError"] = str(exc)
    return normalize(history)


def session_detail(session_id: int) -> dict[str, Any]:
    manager = pipeline_manager()
    session = manager.store.get_session_by_id(session_id)
    if not session or not session.get("thread_id"):
        raise ValueError("会话不存在")
    run_ids = manager.store.run_ids_for_thread(str(session["thread_id"]))
    runs = [manager.store.get_run(run_id) for run_id in run_ids]
    events: list[dict[str, Any]] = []
    logs: list[str] = []
    turns: list[dict[str, Any]] = []
    for run_id in run_ids:
        events.extend(
            event for event in manager.store.events_for_run(run_id)
            if event.get("agent_key") in {"refactor", "session-refactor"}
        )
        logs.extend(manager.store.logs_for_run(run_id))
        turns.extend(
            turn for turn in manager.store.turns_for_run(run_id)
            if turn.get("role") == "driver"
        )
    active_run = next(
        (
            row for row in runs
            if row and row.get("status") in manager.ACTIVE_STATUSES
        ),
        None,
    )
    return normalize({
        "session": session,
        "runs": [row for row in runs if row],
        "events": sorted(events, key=lambda item: str(item.get("created_at") or "")),
        "logs": logs[-3000:],
        "turns": sorted(turns, key=lambda item: str(item.get("created_at") or "")),
        "activeRunId": active_run.get("id") if active_run else None,
    })


def report_payload(run_id: str) -> dict[str, Any]:
    detail = run_detail(run_id)
    return {
        "exportedAt": datetime.now().astimezone().isoformat(),
        "repository": repository_name(),
        **detail,
    }


def json_body(handler: SimpleHTTPRequestHandler) -> dict[str, Any]:
    length = int(handler.headers.get("Content-Length", "0"))
    return json.loads(handler.rfile.read(length) or b"{}")


class Handler(SimpleHTTPRequestHandler):
    def __init__(self, *args: Any, **kwargs: Any):
        super().__init__(*args, directory=str(WEB_ROOT), **kwargs)

    def send_json(self, payload: Any, status: int = 200) -> None:
        data = json.dumps(normalize(payload), ensure_ascii=False).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def send_download(self, payload: Any, filename: str) -> None:
        data = json.dumps(normalize(payload), ensure_ascii=False, indent=2).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Disposition", f'attachment; filename="{filename}"')
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self) -> None:  # noqa: N802
        try:
            parsed = urlparse(self.path)
            path = parsed.path
            if path == "/api/bootstrap":
                self.send_json(bootstrap()); return
            match = re.fullmatch(r"/api/runs/([^/?]+)", path)
            if match:
                self.send_json(run_detail(match.group(1))); return
            match = re.fullmatch(r"/api/runs/([^/?]+)/diff", path)
            if match:
                run = pipeline_manager().store.get_run(match.group(1))
                if not run: raise ValueError("运行记录不存在")
                result = command(["git", "diff", "HEAD"], Path(run["worktree_path"]))
                self.send_json({"diff": result.stdout or result.stderr}); return
            match = re.fullmatch(r"/api/runs/([^/?]+)/report", path)
            if match:
                self.send_download(report_payload(match.group(1)), f"run-{match.group(1)[:8]}.json")
                return
            match = re.fullmatch(r"/api/runs/([^/?]+)/pr", path)
            if match:
                self.send_json({"pullRequest": pipeline_manager().pull_request(match.group(1))})
                return
            match = re.fullmatch(r"/api/sessions/(\d+)", path)
            if match:
                self.send_json(session_detail(int(match.group(1)))); return
            if path in {"/", "/index.html"}:
                self.path = "/index.html"
            super().do_GET()
        except Exception as exc:  # noqa: BLE001
            self.send_json({"error": str(exc)}, 400)

    def do_POST(self) -> None:  # noqa: N802
        try:
            payload = json_body(self)
            manager = pipeline_manager()
            if self.path == "/api/settings":
                maximum = manager.set_max_concurrency(int(payload["maxConcurrency"]))
                self.send_json({"maxConcurrency": maximum}); return
            if self.path == "/api/worktrees":
                result = WorktreeManager(ROOT, WORKTREE_ROOT).ensure(int(payload["issueNumber"]))
                self.send_json(result); return
            if self.path == "/api/runs":
                issue = next(item for item in load_issues() if int(item["number"]) == int(payload["issueNumber"]))
                tree = worktrees().get(int(issue["number"]))
                if not tree: raise ValueError("请先创建独立 Worktree")
                run = manager.start(PipelineConfig(
                    issue_number=int(issue["number"]), issue_title=issue["title"], issue_body=issue.get("body") or "",
                    worktree_path=Path(tree["path"]), test_command=payload.get("testCommand") or "cd backend && go test ./...",
                    refactor_prompt=payload.get("prompt") or "", auto_approve=bool(payload.get("autoApprove")),
                    matt_workflow=bool(payload.get("tdd", True)), max_review_cycles=int(payload.get("maxReviewCycles", 2)),
                    resource_lock=payload.get("resourceLock") or "",
                ))
                self.send_json({"runId": run.run_id}, 201); return
            match = re.fullmatch(
                r"/api/runs/([^/]+)/(stop|approve|cleanup|reject|rerun|repair|open|create-pr|merge-pr)",
                self.path,
            )
            if match:
                run_id, action = match.groups()
                if action == "stop": manager.mark_interrupted(run_id); result = {"status": "stopped"}
                elif action == "approve": result = {"sha": manager.approve_commit(run_id)}
                elif action == "cleanup": result = {"message": manager.cleanup_environment(run_id, remove_volumes=bool(payload.get("removeVolumes")))}
                elif action == "reject": manager.reject_run(run_id); result = {"status": "rejected"}
                elif action == "rerun": result = {"runId": manager.rerun(run_id).run_id}
                elif action == "repair": result = {
                    "runId": manager.request_review_repair(
                        run_id,
                        str(payload.get("prompt") or ""),
                    ).run_id
                }
                elif action == "create-pr": result = {"pullRequest": manager.create_pull_request(run_id)}
                elif action == "merge-pr": result = {"pullRequest": manager.merge_pull_request(run_id)}
                else:
                    run = manager.store.get_run(run_id)
                    if not run: raise ValueError("运行记录不存在")
                    worktree = Path(str(run["worktree_path"]))
                    if not worktree.is_dir(): raise ValueError("Worktree 不存在")
                    opened = command(["open", str(worktree)])
                    if opened.returncode: raise ValueError(opened.stderr or "Finder 打开失败")
                    result = {"opened": str(worktree)}
                self.send_json(result); return
            match = re.fullmatch(r"/api/sessions/(\d+)/(continue|stop|archive|delete)", self.path)
            if match:
                session_id, action = int(match.group(1)), match.group(2)
                if action == "continue": result = {"runId": manager.call_session(session_id, payload.get("prompt") or "继续当前工作")}
                elif action == "stop":
                    detail = session_detail(session_id)
                    if not detail.get("activeRunId"): raise ValueError("当前会话没有运行中的调用")
                    manager.mark_interrupted(str(detail["activeRunId"])); result = {"stopped": True}
                elif action == "archive": manager.archive_session(session_id); result = {"archived": True}
                else: manager.delete_session(session_id); result = {"deleted": True}
                self.send_json(result); return
            raise ValueError("未知操作")
        except Exception as exc:  # noqa: BLE001
            self.send_json({"error": str(exc)}, 400)

    def log_message(self, format: str, *args: Any) -> None:
        return


def main() -> None:
    host = os.environ.get("CREWOPS_HOST", "127.0.0.1")
    port = int(os.environ.get("CREWOPS_PORT", "8765"))
    print(f"CrewOps: http://{host}:{port}")
    server = ThreadingHTTPServer((host, port), Handler)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
