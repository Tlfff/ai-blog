from __future__ import annotations

import sqlite3
import threading
from contextlib import contextmanager
import uuid
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

PROJECT_DIR = Path(__file__).resolve().parent
DEFAULT_DB_PATH = PROJECT_DIR / "data" / "orchestrator.sqlite3"


class StateStore:
    """Small SQLite store for pipeline state, logs, and Codex thread IDs."""

    def __init__(self, path: Path | None = None):
        self.path = Path(path or DEFAULT_DB_PATH).expanduser().resolve()
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self._lock = threading.RLock()
        self._initialize()

    @contextmanager
    def _connection(self):
        connection = sqlite3.connect(self.path, timeout=30)
        connection.row_factory = sqlite3.Row
        connection.execute("PRAGMA busy_timeout = 30000")
        connection.execute("PRAGMA journal_mode = WAL")
        try:
            yield connection
            connection.commit()
        finally:
            connection.close()

    def _initialize(self) -> None:
        with self._lock, self._connection() as connection:
            connection.executescript(
                """
                CREATE TABLE IF NOT EXISTS pipeline_runs (
                    id TEXT PRIMARY KEY,
                    issue_number INTEGER NOT NULL,
                    issue_title TEXT NOT NULL,
                    issue_body TEXT NOT NULL DEFAULT '',
                    worktree_path TEXT NOT NULL,
                    test_command TEXT NOT NULL,
                    refactor_prompt TEXT NOT NULL DEFAULT '',
                    auto_approve INTEGER NOT NULL DEFAULT 0,
                    matt_workflow INTEGER NOT NULL DEFAULT 0,
                    status TEXT NOT NULL DEFAULT 'queued',
                    current_stage TEXT NOT NULL DEFAULT 'queued',
                    error TEXT,
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    finished_at TEXT
                );
                CREATE TABLE IF NOT EXISTS pipeline_logs (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    run_id TEXT NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
                    created_at TEXT NOT NULL,
                    line TEXT NOT NULL
                );
                CREATE TABLE IF NOT EXISTS agent_sessions (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    run_id TEXT NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
                    agent_key TEXT NOT NULL,
                    label TEXT NOT NULL,
                    thread_id TEXT,
                    turn_id TEXT,
                    status TEXT NOT NULL DEFAULT 'queued',
                    command TEXT NOT NULL DEFAULT '',
                    report TEXT NOT NULL DEFAULT '',
                    created_at TEXT NOT NULL,
                    updated_at TEXT NOT NULL,
                    UNIQUE(run_id, agent_key)
                );
                CREATE TABLE IF NOT EXISTS pipeline_events (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    run_id TEXT NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
                    agent_key TEXT NOT NULL,
                    agent_label TEXT NOT NULL,
                    kind TEXT NOT NULL,
                    content TEXT NOT NULL,
                    created_at TEXT NOT NULL
                );
                CREATE TABLE IF NOT EXISTS agent_turns (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    run_id TEXT NOT NULL REFERENCES pipeline_runs(id) ON DELETE CASCADE,
                    thread_id TEXT NOT NULL,
                    turn_id TEXT NOT NULL,
                    turn_kind TEXT NOT NULL,
                    role TEXT NOT NULL,
                    read_only INTEGER NOT NULL DEFAULT 0,
                    status TEXT NOT NULL DEFAULT 'running',
                    prompt_summary TEXT NOT NULL DEFAULT '',
                    report TEXT NOT NULL DEFAULT '',
                    created_at TEXT NOT NULL,
                    completed_at TEXT,
                    UNIQUE(run_id, turn_id)
                );
                CREATE INDEX IF NOT EXISTS idx_pipeline_logs_run_id ON pipeline_logs(run_id);
                CREATE INDEX IF NOT EXISTS idx_pipeline_events_run_id ON pipeline_events(run_id);
                CREATE INDEX IF NOT EXISTS idx_agent_sessions_thread_id ON agent_sessions(thread_id);
                CREATE INDEX IF NOT EXISTS idx_agent_turns_run_id ON agent_turns(run_id);
                CREATE INDEX IF NOT EXISTS idx_agent_turns_thread_id ON agent_turns(thread_id);
                """
            )
            columns = {
                row["name"]
                for row in connection.execute("PRAGMA table_info(pipeline_runs)").fetchall()
            }
            if "compose_project" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN compose_project TEXT")
            if "port_slot" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN port_slot INTEGER")
            if "resource_lock" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN resource_lock TEXT")
            if "reviewed_fingerprint" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN reviewed_fingerprint TEXT")
            if "reviewed_head" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN reviewed_head TEXT")
            if "reviewed_paths" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN reviewed_paths TEXT")
            if "reviewed_diff_hash" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN reviewed_diff_hash TEXT")
            if "run_kind" not in columns:
                connection.execute(
                    "ALTER TABLE pipeline_runs ADD COLUMN run_kind TEXT NOT NULL DEFAULT 'pipeline'"
                )
            if "parent_run_id" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN parent_run_id TEXT")
            if "model" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN model TEXT")
            if "model_provider" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN model_provider TEXT")
            if "reasoning_effort" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN reasoning_effort TEXT")
            if "commit_sha" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN commit_sha TEXT")
            if "pr_number" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN pr_number INTEGER")
            if "pr_url" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN pr_url TEXT")
            if "pr_created_at" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN pr_created_at TEXT")
            if "merged_at" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN merged_at TEXT")
            if "merge_commit_sha" not in columns:
                connection.execute("ALTER TABLE pipeline_runs ADD COLUMN merge_commit_sha TEXT")
            session_columns = {
                row["name"]
                for row in connection.execute("PRAGMA table_info(agent_sessions)").fetchall()
            }
            if "model" not in session_columns:
                connection.execute("ALTER TABLE agent_sessions ADD COLUMN model TEXT")
            if "model_provider" not in session_columns:
                connection.execute("ALTER TABLE agent_sessions ADD COLUMN model_provider TEXT")
            if "reasoning_effort" not in session_columns:
                connection.execute("ALTER TABLE agent_sessions ADD COLUMN reasoning_effort TEXT")
            if "role" not in session_columns:
                connection.execute(
                    "ALTER TABLE agent_sessions ADD COLUMN role TEXT NOT NULL DEFAULT 'legacy'"
                )
            if "read_only" not in session_columns:
                connection.execute(
                    "ALTER TABLE agent_sessions ADD COLUMN read_only INTEGER NOT NULL DEFAULT 0"
                )
            if "archived" not in session_columns:
                connection.execute(
                    "ALTER TABLE agent_sessions ADD COLUMN archived INTEGER NOT NULL DEFAULT 0"
                )
            turn_columns = {
                row["name"]
                for row in connection.execute("PRAGMA table_info(agent_turns)").fetchall()
            }
            if "session_key" not in turn_columns:
                connection.execute(
                    "ALTER TABLE agent_turns ADD COLUMN session_key TEXT NOT NULL DEFAULT 'legacy'"
                )
            connection.execute(
                """
                UPDATE agent_sessions
                SET role = 'driver', read_only = 0
                WHERE agent_key = 'refactor' AND role = 'legacy'
                """
            )
            connection.execute(
                """
                UPDATE agent_sessions
                SET role = 'review_sidecar', read_only = 1
                WHERE agent_key LIKE 'review-standards%'
                   OR agent_key LIKE 'review-spec%'
                """
            )

    @staticmethod
    def _now() -> str:
        return datetime.now(timezone.utc).isoformat()

    def create_run(self, config: dict[str, Any]) -> str:
        run_id = str(uuid.uuid4())
        now = self._now()
        with self._lock, self._connection() as connection:
            connection.execute(
                """
                INSERT INTO pipeline_runs (
                    id, issue_number, issue_title, issue_body, worktree_path,
                    test_command, refactor_prompt, auto_approve, matt_workflow,
                    status, current_stage, created_at, updated_at, run_kind, parent_run_id
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'queued', 'queued', ?, ?, ?, ?)
                """,
                (
                    run_id,
                    config["issue_number"],
                    config["issue_title"],
                    config.get("issue_body", ""),
                    str(config["worktree_path"]),
                    config["test_command"],
                    config.get("refactor_prompt", ""),
                    int(bool(config.get("auto_approve", False))),
                    int(bool(config.get("matt_workflow", False))),
                    now,
                    now,
                    config.get("run_kind", "pipeline"),
                    config.get("parent_run_id"),
                ),
            )
        return run_id

    def update_run(self, run_id: str, **fields: Any) -> None:
        allowed = {
            "status",
            "current_stage",
            "error",
            "updated_at",
            "finished_at",
            "test_command",
            "refactor_prompt",
            "compose_project",
            "port_slot",
            "resource_lock",
            "reviewed_fingerprint",
            "reviewed_head",
            "reviewed_paths",
            "reviewed_diff_hash",
            "run_kind",
            "parent_run_id",
            "model",
            "model_provider",
            "reasoning_effort",
            "commit_sha",
            "pr_number",
            "pr_url",
            "pr_created_at",
            "merged_at",
            "merge_commit_sha",
        }
        updates = {key: value for key, value in fields.items() if key in allowed}
        if not updates:
            return
        updates.setdefault("updated_at", self._now())
        assignments = ", ".join(f"{key} = ?" for key in updates)
        with self._lock, self._connection() as connection:
            connection.execute(
                f"UPDATE pipeline_runs SET {assignments} WHERE id = ?",
                (*updates.values(), run_id),
            )

    def append_log(self, run_id: str, line: str) -> None:
        with self._lock, self._connection() as connection:
            connection.execute(
                "INSERT INTO pipeline_logs (run_id, created_at, line) VALUES (?, ?, ?)",
                (run_id, self._now(), line),
            )
            connection.execute(
                "UPDATE pipeline_runs SET updated_at = ? WHERE id = ?",
                (self._now(), run_id),
            )

    def append_event(
        self,
        run_id: str,
        agent_key: str,
        agent_label: str,
        kind: str,
        content: str,
    ) -> None:
        if not content.strip():
            return
        with self._lock, self._connection() as connection:
            connection.execute(
                """
                INSERT INTO pipeline_events (
                    run_id, agent_key, agent_label, kind, content, created_at
                ) VALUES (?, ?, ?, ?, ?, ?)
                """,
                (run_id, agent_key, agent_label, kind, content, self._now()),
            )
            connection.execute(
                "UPDATE pipeline_runs SET updated_at = ? WHERE id = ?",
                (self._now(), run_id),
            )

    def events_for_run(self, run_id: str, limit: int = 500) -> list[dict[str, Any]]:
        with self._lock, self._connection() as connection:
            rows = connection.execute(
                """
                SELECT * FROM pipeline_events
                WHERE run_id = ?
                ORDER BY id DESC LIMIT ?
                """,
                (run_id, limit),
            ).fetchall()
        return [dict(row) for row in reversed(rows)]

    def upsert_session(self, run_id: str, agent_key: str, label: str, **fields: Any) -> None:
        now = self._now()
        with self._lock, self._connection() as connection:
            existing = connection.execute(
                "SELECT * FROM agent_sessions WHERE run_id = ? AND agent_key = ?",
                (run_id, agent_key),
            ).fetchone()
            def value(name: str, default: Any) -> Any:
                if name in fields:
                    return fields[name]
                return existing[name] if existing else default

            connection.execute(
                """
                INSERT INTO agent_sessions (
                    run_id, agent_key, label, thread_id, turn_id, status,
                    command, report, model, model_provider, reasoning_effort,
                    role, read_only, created_at, updated_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                ON CONFLICT(run_id, agent_key) DO UPDATE SET
                    label = excluded.label,
                    thread_id = excluded.thread_id,
                    turn_id = excluded.turn_id,
                    status = excluded.status,
                    command = excluded.command,
                    report = excluded.report,
                    model = excluded.model,
                    model_provider = excluded.model_provider,
                    reasoning_effort = excluded.reasoning_effort,
                    role = excluded.role,
                    read_only = excluded.read_only,
                    updated_at = excluded.updated_at
                """,
                (
                    run_id,
                    agent_key,
                    label,
                    value("thread_id", None),
                    value("turn_id", None),
                    value("status", "queued"),
                    value("command", ""),
                    value("report", ""),
                    value("model", None),
                    value("model_provider", None),
                    value("reasoning_effort", None),
                    value("role", "legacy"),
                    int(bool(value("read_only", False))),
                    existing["created_at"] if existing else now,
                    now,
                ),
            )

    def get_run(self, run_id: str) -> dict[str, Any] | None:
        with self._lock, self._connection() as connection:
            row = connection.execute(
                "SELECT * FROM pipeline_runs WHERE id = ?", (run_id,)
            ).fetchone()
        return dict(row) if row else None

    def get_session(self, run_id: str, agent_key: str) -> dict[str, Any] | None:
        with self._lock, self._connection() as connection:
            row = connection.execute(
                "SELECT * FROM agent_sessions WHERE run_id = ? AND agent_key = ?",
                (run_id, agent_key),
            ).fetchone()
        return dict(row) if row else None

    def get_session_by_id(self, session_id: int) -> dict[str, Any] | None:
        with self._lock, self._connection() as connection:
            row = connection.execute(
                """
                SELECT s.*, r.issue_number, r.issue_title, r.worktree_path, r.status AS run_status,
                       r.auto_approve, r.matt_workflow
                FROM agent_sessions s
                JOIN pipeline_runs r ON r.id = s.run_id
                WHERE s.id = ?
                """,
                (session_id,),
            ).fetchone()
        return dict(row) if row else None

    def list_runs(self, limit: int = 50) -> list[dict[str, Any]]:
        with self._lock, self._connection() as connection:
            rows = connection.execute(
                "SELECT * FROM pipeline_runs ORDER BY updated_at DESC LIMIT ?", (limit,)
            ).fetchall()
        return [dict(row) for row in rows]

    def sessions_for_run(self, run_id: str) -> list[dict[str, Any]]:
        with self._lock, self._connection() as connection:
            rows = connection.execute(
                "SELECT * FROM agent_sessions WHERE run_id = ? ORDER BY updated_at",
                (run_id,),
            ).fetchall()
        return [dict(row) for row in rows]

    def driver_session_for_run(self, run_id: str) -> dict[str, Any] | None:
        with self._lock, self._connection() as connection:
            row = connection.execute(
                """
                SELECT * FROM agent_sessions
                WHERE run_id = ? AND role = 'driver' AND read_only = 0
                ORDER BY updated_at DESC LIMIT 1
                """,
                (run_id,),
            ).fetchone()
        return dict(row) if row else None

    def list_driver_sessions(
        self,
        limit: int = 100,
        *,
        include_archived: bool = False,
    ) -> list[dict[str, Any]]:
        archived_clause = "" if include_archived else "AND s.archived = 0"
        with self._lock, self._connection() as connection:
            rows = connection.execute(
                f"""
                SELECT s.*, r.issue_number, r.issue_title, r.worktree_path,
                       r.status AS run_status
                FROM agent_sessions s
                JOIN pipeline_runs r ON r.id = s.run_id
                WHERE s.role = 'driver' AND s.read_only = 0 AND s.thread_id IS NOT NULL
                  {archived_clause}
                ORDER BY s.updated_at DESC
                LIMIT ?
                """,
                (limit,),
            ).fetchall()
        sessions: list[dict[str, Any]] = []
        seen: set[tuple[int, str, str]] = set()
        for row in rows:
            item = dict(row)
            identity = (
                int(item["issue_number"]),
                str(item["worktree_path"]),
                str(item["thread_id"]),
            )
            if identity in seen:
                continue
            seen.add(identity)
            sessions.append(item)
        return sessions

    def latest_driver_session(
        self,
        *,
        issue_number: int,
        worktree_path: Path,
    ) -> dict[str, Any] | None:
        with self._lock, self._connection() as connection:
            row = connection.execute(
                """
                SELECT s.*
                FROM agent_sessions s
                JOIN pipeline_runs r ON r.id = s.run_id
                WHERE r.issue_number = ?
                  AND r.worktree_path = ?
                  AND s.role = 'driver'
                  AND s.read_only = 0
                  AND s.thread_id IS NOT NULL
                ORDER BY s.updated_at DESC
                LIMIT 1
                """,
                (issue_number, str(worktree_path)),
            ).fetchone()
        return dict(row) if row else None

    def start_turn(
        self,
        *,
        run_id: str,
        thread_id: str,
        turn_id: str,
        turn_kind: str,
        role: str,
        read_only: bool,
        prompt_summary: str,
        session_key: str = "legacy",
    ) -> None:
        with self._lock, self._connection() as connection:
            connection.execute(
                """
                INSERT INTO agent_turns (
                    run_id, thread_id, turn_id, turn_kind, role, read_only,
                    status, prompt_summary, created_at, session_key
                ) VALUES (?, ?, ?, ?, ?, ?, 'running', ?, ?, ?)
                ON CONFLICT(run_id, turn_id) DO UPDATE SET
                    thread_id = excluded.thread_id,
                    turn_kind = excluded.turn_kind,
                    role = excluded.role,
                    read_only = excluded.read_only,
                    status = 'running',
                    prompt_summary = excluded.prompt_summary,
                    session_key = excluded.session_key
                """,
                (
                    run_id,
                    thread_id,
                    turn_id,
                    turn_kind,
                    role,
                    int(read_only),
                    prompt_summary,
                    self._now(),
                    session_key,
                ),
            )

    def finish_turn(
        self,
        *,
        run_id: str,
        turn_id: str,
        status: str,
        report: str = "",
    ) -> None:
        with self._lock, self._connection() as connection:
            connection.execute(
                """
                UPDATE agent_turns
                SET status = ?, report = ?, completed_at = ?
                WHERE run_id = ? AND turn_id = ?
                """,
                (status, report, self._now(), run_id, turn_id),
            )

    def turns_for_run(self, run_id: str) -> list[dict[str, Any]]:
        with self._lock, self._connection() as connection:
            rows = connection.execute(
                "SELECT * FROM agent_turns WHERE run_id = ? ORDER BY id",
                (run_id,),
            ).fetchall()
        return [dict(row) for row in rows]

    def run_ids_for_thread(self, thread_id: str) -> list[str]:
        with self._lock, self._connection() as connection:
            rows = connection.execute(
                """
                SELECT DISTINCT run_id
                FROM agent_sessions
                WHERE thread_id = ?
                ORDER BY updated_at
                """,
                (thread_id,),
            ).fetchall()
        return [str(row["run_id"]) for row in rows]

    def finish_running_sessions(self, run_id: str, status: str) -> None:
        with self._lock, self._connection() as connection:
            connection.execute(
                """
                UPDATE agent_sessions
                SET status = ?, updated_at = ?
                WHERE run_id = ? AND status = 'running'
                """,
                (status, self._now(), run_id),
            )

    def list_sessions(self, limit: int = 100) -> list[dict[str, Any]]:
        with self._lock, self._connection() as connection:
            rows = connection.execute(
                """
                SELECT s.*, r.issue_number, r.issue_title, r.worktree_path, r.status AS run_status
                FROM agent_sessions s
                JOIN pipeline_runs r ON r.id = s.run_id
                ORDER BY s.updated_at DESC
                LIMIT ?
                """,
                (limit,),
            ).fetchall()
        return [dict(row) for row in rows]

    def delete_session(self, session_id: int) -> None:
        with self._lock, self._connection() as connection:
            connection.execute("DELETE FROM agent_sessions WHERE id = ?", (session_id,))

    def delete_sessions_by_thread(self, thread_id: str) -> None:
        with self._lock, self._connection() as connection:
            connection.execute("DELETE FROM agent_sessions WHERE thread_id = ?", (thread_id,))

    def archive_sessions_by_thread(self, thread_id: str, archived: bool = True) -> None:
        with self._lock, self._connection() as connection:
            connection.execute(
                "UPDATE agent_sessions SET archived = ?, updated_at = ? WHERE thread_id = ?",
                (int(archived), self._now(), thread_id),
            )

    def delete_run(self, run_id: str) -> None:
        with self._lock, self._connection() as connection:
            connection.execute("DELETE FROM pipeline_runs WHERE id = ?", (run_id,))

    def logs_for_run(self, run_id: str, limit: int = 3000) -> list[str]:
        with self._lock, self._connection() as connection:
            rows = connection.execute(
                "SELECT line FROM pipeline_logs WHERE run_id = ? ORDER BY id DESC LIMIT ?",
                (run_id, limit),
            ).fetchall()
        return [row["line"] for row in reversed(rows)]
