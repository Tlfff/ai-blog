from __future__ import annotations

import shutil
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Callable

PROJECT_DIR = Path(__file__).resolve().parent
LOCAL_TOOL_BIN = PROJECT_DIR / ".tools" / "bin"


@dataclass(frozen=True)
class WorktreeResult:
    path: Path
    branch: str
    base_ref: str
    created: bool
    message: str


class WorktreeManager:
    """Create or recover issue worktrees from the latest remote default branch."""

    def __init__(self, repo_root: Path, worktree_root: Path):
        self.repo_root = repo_root.resolve()
        self.worktree_root = worktree_root.resolve()

    def _run(self, *args: str, timeout: int = 120) -> subprocess.CompletedProcess[str]:
        completed = subprocess.run(
            ["git", *args],
            cwd=self.repo_root,
            text=True,
            capture_output=True,
            check=False,
            timeout=timeout,
        )
        if completed.returncode != 0:
            raise RuntimeError(
                completed.stderr.strip()
                or completed.stdout.strip()
                or f"git {' '.join(args)} 执行失败"
            )
        return completed

    def _remote_base(self) -> str:
        symbolic = subprocess.run(
            ["git", "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"],
            cwd=self.repo_root,
            text=True,
            capture_output=True,
            check=False,
            timeout=30,
        )
        if symbolic.returncode == 0 and symbolic.stdout.strip():
            return symbolic.stdout.strip()
        for candidate in ("origin/master", "origin/main"):
            exists = subprocess.run(
                ["git", "show-ref", "--verify", "--quiet", f"refs/remotes/{candidate}"],
                cwd=self.repo_root,
                check=False,
                timeout=30,
            )
            if exists.returncode == 0:
                return candidate
        raise RuntimeError("找不到 origin/master 或 origin/main，无法创建最新 worktree。")

    def _registered_worktrees(self) -> dict[str, Path]:
        output = self._run("worktree", "list", "--porcelain").stdout
        result: dict[str, Path] = {}
        current_path: Path | None = None
        for line in output.splitlines() + [""]:
            if line.startswith("worktree "):
                current_path = Path(line.removeprefix("worktree ")).resolve()
            elif line.startswith("branch ") and current_path:
                branch = line.removeprefix("branch ").removeprefix("refs/heads/")
                result[branch] = current_path
            elif not line:
                current_path = None
        return result

    def ensure(self, issue_number: int) -> WorktreeResult:
        branch = f"codex/issue-{issue_number}"
        target = self.worktree_root / f"issue-{issue_number}"
        self._run("fetch", "origin", "--prune", timeout=180)
        base_ref = self._remote_base()
        self.worktree_root.mkdir(parents=True, exist_ok=True)

        registered = self._registered_worktrees()
        existing_path = registered.get(branch)
        if existing_path:
            if existing_path == target.resolve():
                return WorktreeResult(target, branch, base_ref, False, "Worktree 已存在，继续使用。")
            raise RuntimeError(f"分支 {branch} 已被其他 worktree 使用：{existing_path}")
        if target.exists():
            raise RuntimeError(f"目标目录已存在但不是已注册 worktree：{target}")

        local_branch = subprocess.run(
            ["git", "show-ref", "--verify", "--quiet", f"refs/heads/{branch}"],
            cwd=self.repo_root,
            check=False,
            timeout=30,
        ).returncode == 0
        if local_branch:
            self._run("worktree", "add", str(target), branch)
            return WorktreeResult(target, branch, base_ref, False, "已恢复现有 Issue 分支。")

        remote_branch = subprocess.run(
            ["git", "show-ref", "--verify", "--quiet", f"refs/remotes/origin/{branch}"],
            cwd=self.repo_root,
            check=False,
            timeout=30,
        ).returncode == 0
        if remote_branch:
            self._run("branch", "--track", branch, f"origin/{branch}")
            self._run("worktree", "add", str(target), branch)
            return WorktreeResult(target, branch, base_ref, False, "已从远端恢复 Issue 分支。")

        self._run("worktree", "add", "-b", branch, str(target), base_ref)
        return WorktreeResult(target, branch, base_ref, True, f"已基于 {base_ref} 创建 Issue 分支。")


@dataclass(frozen=True)
class MissingTool:
    name: str
    reason: str
    install_hint: str


@dataclass(frozen=True)
class PreflightReport:
    required: tuple[str, ...]
    missing: tuple[MissingTool, ...]

    @property
    def passed(self) -> bool:
        return not self.missing

    def message(self) -> str:
        if self.passed:
            return "工具预检通过。"
        details = "\n".join(
            f"- {item.name}：{item.reason}\n  安装：{item.install_hint}"
            for item in self.missing
        )
        return "启动前工具预检失败：\n" + details


class ToolPreflight:
    INSTALL_HINTS = {
        "git": "brew install git",
        "gh": "brew install gh",
        "codex": "从 ChatGPT/Codex 安装并加入 PATH",
        "go": "brew install go",
        "protoc": "brew install protobuf",
        "protoc-gen-go": "go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28.1",
        "protoc-gen-go-grpc": "go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0",
        "protoc-gen-gin-client-http-leo": "go install codeup.aliyun.com/qimao/leo/code-auto/protoc-gen-gin-client-http-leo@latest",
        "protoc-gen-openapi": "go install github.com/google/gnostic/cmd/protoc-gen-openapi@v0.6.9",
        "protoc-gen-validate": "go install github.com/envoyproxy/protoc-gen-validate@latest",
        "protoc-gen-go-enum": "go install codeup.aliyun.com/qimao/go-contrib/protoc-gen-go-enum@latest",
        "protoc-gen-doc": "go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest",
        "wire": "go install github.com/google/wire/cmd/wire@latest",
    }

    VERSION_REQUIREMENTS = {
        "protoc": "libprotoc 35.1",
        "protoc-gen-go": "v1.28.1",
        "protoc-gen-go-grpc": "1.3.0",
    }

    def __init__(
        self,
        which: Callable[[str], str | None] = shutil.which,
        runner: Callable[..., subprocess.CompletedProcess[str]] = subprocess.run,
        local_bin: Path | None = None,
    ):
        self.system_which = which
        self.runner = runner
        self.local_bin = local_bin

    def which(self, name: str) -> str | None:
        if self.local_bin:
            local = self.local_bin / name
            if local.is_file():
                return str(local)
        return self.system_which(name)

    def check(
        self,
        *,
        changed_files: tuple[str, ...],
        issue_text: str,
        has_backend: bool,
    ) -> PreflightReport:
        required: dict[str, str] = {
            "git": "创建和检查 worktree",
            "gh": "读取 GitHub Issue 和父规格",
            "codex": "启动 Codex App Server",
        }
        if has_backend:
            required["go"] = "执行后端测试和生成命令"
            required.update(
                {
                    "protoc": "锁定后端 Proto 生成环境",
                    "protoc-gen-go": "锁定 Go Protobuf 生成版本",
                    "protoc-gen-go-grpc": "锁定 gRPC 生成版本",
                    "protoc-gen-gin-client-http-leo": "生成 Leo HTTP 代码",
                    "protoc-gen-openapi": "生成 OpenAPI 文档",
                    "protoc-gen-validate": "生成 Proto 校验代码",
                    "wire": "生成依赖注入代码",
                }
            )

        text = issue_text.lower()
        needs_api = any(
            path.startswith("backend/api/") and path.endswith(".proto")
            for path in changed_files
        ) or any(keyword in text for keyword in ("proto", "grpc", " rpc", "接口"))
        needs_config = "backend/internal/conf/conf.proto" in changed_files or any(
            keyword in issue_text for keyword in ("配置", "XDB 路径")
        )
        needs_wire = any(
            path.endswith("/wire.go") or path.endswith("/provider_set.go")
            for path in changed_files
        ) or any(keyword in issue_text for keyword in ("Wire", "依赖注入"))

        if needs_api:
            required.update(
                {
                    "protoc": "执行 make api",
                    "protoc-gen-go": "生成 Go Protobuf",
                    "protoc-gen-go-grpc": "生成 gRPC 代码",
                    "protoc-gen-gin-client-http-leo": "生成 Leo HTTP 代码",
                    "protoc-gen-openapi": "生成 OpenAPI 文档",
                    "protoc-gen-validate": "生成 Proto 校验代码",
                }
            )
        if needs_config:
            required.update(
                {
                    "protoc": "执行 make config",
                    "protoc-gen-go": "生成配置 Protobuf",
                    "protoc-gen-go-enum": "生成领域枚举",
                    "protoc-gen-doc": "生成配置文档",
                }
            )
        if needs_wire:
            required["wire"] = "执行 make gen 更新依赖注入"

        missing = tuple(
            MissingTool(
                name,
                reason,
                self.INSTALL_HINTS.get(name, f"请安装 {name} 并加入 PATH"),
            )
            for name, reason in required.items()
            if self.which(name) is None
        )
        version_missing: list[MissingTool] = []
        for name, expected in self.VERSION_REQUIREMENTS.items():
            if name not in required:
                continue
            path = self.which(name)
            if not path:
                continue
            try:
                result = self.runner(
                    [path, "--version"],
                    text=True,
                    capture_output=True,
                    check=False,
                    timeout=15,
                )
            except (OSError, subprocess.SubprocessError):
                continue
            actual = (result.stdout + result.stderr).strip()
            if result.returncode != 0 or expected not in actual:
                version_missing.append(
                    MissingTool(
                        name,
                        f"版本不匹配：需要 {expected}，当前为 {actual or '未知'}",
                        self.INSTALL_HINTS[name],
                    )
                )
        return PreflightReport(tuple(required), missing + tuple(version_missing))
