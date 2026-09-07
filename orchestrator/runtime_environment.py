from __future__ import annotations

import os
import re
import subprocess
from dataclasses import dataclass
from pathlib import Path

PROJECT_DIR = Path(__file__).resolve().parent
LOCAL_TOOL_BIN = PROJECT_DIR / ".tools" / "bin"


@dataclass(frozen=True)
class CleanupResult:
    returncode: int
    output: str


@dataclass(frozen=True)
class RuntimeEnvironment:
    """Per-run process and Docker Compose isolation behind one interface."""

    run_id: str
    issue_number: int
    worktree_path: Path
    slot: int
    compose_project: str

    @classmethod
    def create(
        cls,
        *,
        run_id: str,
        issue_number: int,
        worktree_path: Path,
        slot: int,
    ) -> "RuntimeEnvironment":
        suffix = re.sub(r"[^a-z0-9]", "", run_id.lower())[:8]
        project = f"ai-blog-issue-{issue_number}-{suffix}"
        return cls(run_id, issue_number, worktree_path, slot, project)

    @property
    def port_offset(self) -> int:
        return self.slot * 100

    def process_env(self) -> dict[str, str]:
        env = dict(os.environ)
        env["PATH"] = str(LOCAL_TOOL_BIN) + os.pathsep + env.get("PATH", "")
        offset = self.port_offset
        env.update(
            {
                "COMPOSE_PROJECT_NAME": self.compose_project,
                "AI_BLOG_RUN_ID": self.run_id,
                "AI_BLOG_ISSUE_NUMBER": str(self.issue_number),
                "AI_BLOG_PORT_SLOT": str(self.slot),
                "AI_BLOG_PORT_OFFSET": str(offset),
                "AI_BLOG_HTTP_PORT": str(18080 + offset),
                "AI_BLOG_GRPC_PORT": str(19090 + offset),
                "AI_BLOG_MYSQL_PORT": str(13306 + offset),
                "AI_BLOG_REDIS_PORT": str(16379 + offset),
                "AI_BLOG_KAFKA_PORT": str(19092 + offset),
                "AI_BLOG_MONGO_PORT": str(17017 + offset),
                "AI_BLOG_MEILI_PORT": str(17700 + offset),
                "AI_BLOG_MINIO_API_PORT": str(19000 + offset),
                "AI_BLOG_MINIO_CONSOLE_PORT": str(19001 + offset),
            }
        )
        return env

    def compose_file(self) -> Path | None:
        candidates = (
            "compose.yaml",
            "compose.yml",
            "docker-compose.yaml",
            "docker-compose.yml",
            "backend/compose.yaml",
            "backend/compose.yml",
            "backend/docker-compose.yaml",
            "backend/docker-compose.yml",
        )
        return next(
            (self.worktree_path / path for path in candidates if (self.worktree_path / path).is_file()),
            None,
        )

    def cleanup(self, *, remove_volumes: bool) -> CleanupResult:
        compose_file = self.compose_file()
        if compose_file is None:
            return CleanupResult(
                2,
                "当前 worktree 没有 Compose 文件，因此没有可由控制台清理的隔离运行环境。",
            )
        command = [
            "docker",
            "compose",
            "-p",
            self.compose_project,
            "-f",
            str(compose_file),
            "down",
            "--remove-orphans",
        ]
        if remove_volumes:
            command.append("-v")
        completed = subprocess.run(
            command,
            cwd=self.worktree_path,
            env=self.process_env(),
            text=True,
            capture_output=True,
            check=False,
            timeout=180,
        )
        output = "\n".join(
            part.strip() for part in (completed.stdout, completed.stderr) if part.strip()
        )
        return CleanupResult(completed.returncode, output)
