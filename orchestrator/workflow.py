from __future__ import annotations

import json
import hashlib
import os
import re
import shlex
import subprocess
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Callable


@dataclass(frozen=True)
class ChangeSet:
    """Complete Git working-tree change set, including untracked files."""

    paths: tuple[str, ...]
    tracked_paths: tuple[str, ...]
    untracked_paths: tuple[str, ...]
    fingerprint: str
    head_sha: str = ""
    diff_hash: str = ""

    @staticmethod
    def _git(worktree: Path, *args: str) -> bytes:
        completed = subprocess.run(
            ["git", *args],
            cwd=worktree,
            capture_output=True,
            check=False,
            timeout=30,
        )
        if completed.returncode != 0:
            raise RuntimeError(
                completed.stderr.decode(errors="replace").strip()
                or f"git {' '.join(args)} 执行失败"
            )
        return completed.stdout

    @staticmethod
    def _split_paths(payload: bytes) -> tuple[str, ...]:
        return tuple(
            item.decode("utf-8", errors="surrogateescape")
            for item in payload.split(b"\0")
            if item
        )

    @classmethod
    def capture(cls, worktree: Path) -> "ChangeSet":
        head_sha = cls._git(worktree, "rev-parse", "HEAD").decode().strip()
        tracked = cls._split_paths(cls._git(worktree, "diff", "--name-only", "-z", "HEAD"))
        untracked = cls._split_paths(
            cls._git(worktree, "ls-files", "--others", "--exclude-standard", "-z")
        )
        paths = tuple(sorted(set(tracked) | set(untracked)))

        diff_digest = hashlib.sha256()
        for relative in paths:
            path = worktree / relative
            diff_digest.update(b"\0path\0")
            diff_digest.update(relative.encode("utf-8", errors="surrogateescape"))
            if path.is_symlink():
                diff_digest.update(b"\0symlink\0")
                diff_digest.update(os.readlink(path).encode("utf-8", errors="surrogateescape"))
            elif path.is_file():
                diff_digest.update(b"\0file\0")
                diff_digest.update(b"1" if path.stat().st_mode & 0o111 else b"0")
                diff_digest.update(path.read_bytes())
            else:
                diff_digest.update(b"\0deleted\0")
        diff_hash = diff_digest.hexdigest()
        fingerprint = hashlib.sha256(
            (head_sha + "\0" + "\0".join(paths) + "\0" + diff_hash).encode(
                "utf-8", errors="surrogateescape"
            )
        ).hexdigest()
        return cls(
            paths,
            tuple(sorted(tracked)),
            tuple(sorted(untracked)),
            fingerprint,
            head_sha,
            diff_hash,
        )


GENERATED_PATH_SUFFIXES = (
    ".pb.go",
    ".pb.validate.go",
    "_grpc.pb.go",
    "_http.pb.go",
    "validate.pb.go",
    "wire_gen.go",
    "openapi.yaml",
)


def generated_path_is_related(path: str, changed_files: tuple[str, ...]) -> bool:
    if not path.endswith(GENERATED_PATH_SUFFIXES):
        return False
    path_obj = Path(path)
    proto_sources = [item for item in changed_files if item.endswith(".proto")]
    for source in proto_sources:
        source_obj = Path(source)
        if path_obj.parent == source_obj.parent:
            if path_obj.name.startswith(source_obj.stem) or path_obj.name == "validate.pb.go":
                return True
        if source.startswith("backend/api/") and path == "backend/openapi.yaml":
            return True
    if any(
        item.endswith("/wire.go") or item.endswith("/provider_set.go")
        for item in changed_files
    ):
        return path.endswith("wire_gen.go")
    return False


def sanitize_unrelated_generated_drift(worktree: Path) -> tuple[str, ...]:
    if not (worktree / ".git").exists():
        return ()
    change_set = ChangeSet.capture(worktree)
    unrelated = tuple(
        path
        for path in change_set.paths
        if path.endswith(GENERATED_PATH_SUFFIXES)
        and not generated_path_is_related(path, change_set.paths)
    )
    for relative in unrelated:
        path = worktree / relative
        if relative in change_set.tracked_paths:
            restored = subprocess.run(
                ["git", "checkout", "HEAD", "--", relative],
                cwd=worktree,
                text=True,
                capture_output=True,
                check=False,
                timeout=30,
            )
            if restored.returncode != 0:
                raise RuntimeError(restored.stderr.strip() or f"恢复生成文件失败：{relative}")
        elif path.exists():
            path.unlink()
    return unrelated


@dataclass(frozen=True)
class ContextPack:
    source_paths: tuple[str, ...]
    parent_issue_number: int | None
    parent_issue_title: str
    parent_issue_body: str

    def instructions(self) -> str:
        sources = "\n".join(f"- {path}" for path in self.source_paths)
        parent = ""
        if self.parent_issue_number:
            parent = (
                f"\n父规格 #{self.parent_issue_number}：{self.parent_issue_title}\n"
                f"{self.parent_issue_body}\n"
            )
        return (
            "开始修改前必须读取并遵守以下上下文来源；AGENTS.md 需完整读取，规格、功能和领域文档"
            "只读取与当前 Issue/父规格相关的章节，不要遍历无关内容；如果某文件不存在则记录后继续：\n"
            f"{sources}\n{parent}\n"
            "层级更深的 AGENTS.md 优先于上层规则。完成读取后，先输出一份“验收标准 → 技术决策 → "
            "实现位置 → 测试”的映射清单；此时不要修改代码。"
        )


class ContextPackBuilder:
    CANDIDATE_SOURCES = (
        "AGENTS.md",
        "backend/AGENTS.md",
        "docs/agents/domain.md",
        "docs/specs/blog-backend-tickets.md",
        "backend/功能文档.md",
        "CONTEXT.md",
        "CONTEXT-MAP.md",
    )

    def __init__(self, issue_loader: Callable[[int], tuple[str, str]] | None = None):
        self.issue_loader = issue_loader

    @staticmethod
    def parent_issue_number(issue_body: str) -> int | None:
        match = re.search(r"## Parent\s*\n+\s*-\s*#(\d+)", issue_body)
        return int(match.group(1)) if match else None

    def build(self, worktree: Path, issue_body: str) -> ContextPack:
        paths = [path for path in self.CANDIDATE_SOURCES if (worktree / path).exists()]
        adr_root = worktree / "docs" / "adr"
        if adr_root.exists():
            paths.extend(
                str(path.relative_to(worktree))
                for path in sorted(adr_root.rglob("*.md"))
            )
        parent_number = self.parent_issue_number(issue_body)
        parent_title = ""
        parent_body = ""
        if parent_number and self.issue_loader:
            try:
                parent_title, parent_body = self.issue_loader(parent_number)
            except (OSError, RuntimeError, ValueError, subprocess.SubprocessError):
                pass
        return ContextPack(tuple(paths), parent_number, parent_title, parent_body)


@dataclass(frozen=True)
class ReviewPack:
    change_paths: tuple[str, ...]
    handwritten_paths: tuple[str, ...]
    generated_paths: tuple[str, ...]
    standards_sources: tuple[str, ...]
    spec_excerpt: str
    validation_summary: str

    def manifest(self, paths: tuple[str, ...]) -> str:
        return "\n".join(f"- {path}" for path in paths) or "- 无"


class ReviewPackBuilder:
    GENERATED_SUFFIXES = (
        ".pb.go",
        ".pb.validate.go",
        "_grpc.pb.go",
        "_http.pb.go",
        "_enum.pb.go",
        "wire_gen.go",
        "openapi.yaml",
    )
    KEYWORDS = (
        "用户", "登录", "认证", "会话", "头像", "密码", "手机号",
        "文章", "正文", "图片", "上传", "创建", "详情", "发布", "草稿",
        "评论", "回复", "点赞", "通知", "搜索", "Meili", "gRPC", "RPC",
        "MinIO", "Redis", "Kafka", "MongoDB", "MySQL", "Proto", "Wire",
        "响应", "分页", "事务", "权限", "幂等",
    )

    @classmethod
    def _keywords(cls, issue_title: str, issue_body: str) -> tuple[str, ...]:
        text = f"{issue_title}\n{issue_body}"
        selected = [keyword for keyword in cls.KEYWORDS if keyword.lower() in text.lower()]
        return tuple(selected or ("验收", "架构", "接口"))

    @staticmethod
    def _sections(document: str) -> list[str]:
        chunks = re.split(r"(?=^#{1,4}\s+)", document, flags=re.MULTILINE)
        return [chunk.strip() for chunk in chunks if chunk.strip()]

    @classmethod
    def _excerpt(cls, path: Path, keywords: tuple[str, ...], limit: int) -> str:
        if not path.is_file():
            return ""
        document = path.read_text(errors="replace")
        ranked: list[tuple[int, int, str]] = []
        for index, section in enumerate(cls._sections(document)):
            score = sum(section.lower().count(keyword.lower()) for keyword in keywords)
            if score:
                ranked.append((score, -index, section))
        selected: list[str] = []
        used = 0
        for _, _, section in sorted(ranked, reverse=True):
            if used >= limit:
                break
            remaining = limit - used
            selected.append(section[:remaining])
            used += min(len(section), remaining)
        return "\n\n".join(selected)

    @classmethod
    def build(
        cls,
        *,
        worktree: Path,
        change_set: ChangeSet,
        context_pack: ContextPack,
        issue_title: str,
        issue_body: str,
        validation_steps: tuple["ValidationStepResult", ...],
    ) -> ReviewPack:
        generated = tuple(
            path
            for path in change_set.paths
            if path.endswith(cls.GENERATED_SUFFIXES)
        )
        handwritten = tuple(path for path in change_set.paths if path not in generated)
        standards_sources = tuple(
            path
            for path in context_pack.source_paths
            if path == "AGENTS.md" or path.endswith("/AGENTS.md")
        )
        keywords = cls._keywords(issue_title, issue_body)
        excerpts: list[str] = []
        if context_pack.parent_issue_body:
            parent_sections = cls._sections(context_pack.parent_issue_body)
            matching = [
                section
                for section in parent_sections
                if any(keyword.lower() in section.lower() for keyword in keywords)
                or any(word in section for word in ("不得", "Proto", "仓储", "响应", "事务"))
            ]
            excerpts.append("## 父规格相关决策\n" + "\n\n".join(matching)[:4000])
        for relative in ("docs/specs/blog-backend-tickets.md", "backend/功能文档.md"):
            excerpt = cls._excerpt(worktree / relative, keywords, 4500)
            if excerpt:
                excerpts.append(f"## {relative} 相关章节\n{excerpt}")
        validation_summary = "\n".join(
            f"- {'PASS' if step.exit_code == 0 else 'FAIL'} · {step.name} · `{step.command}`"
            for step in validation_steps[-12:]
        ) or "- 尚无验证结果"
        return ReviewPack(
            change_set.paths,
            handwritten,
            generated,
            standards_sources,
            "\n\n".join(excerpts)[:10000],
            validation_summary,
        )


@dataclass(frozen=True)
class ValidationStep:
    name: str
    command: str


@dataclass(frozen=True)
class ValidationStepResult:
    name: str
    command: str
    exit_code: int
    output: str


@dataclass(frozen=True)
class ValidationResult:
    steps: tuple[ValidationStepResult, ...]

    @property
    def passed(self) -> bool:
        return all(step.exit_code == 0 for step in self.steps)

    def feedback(self) -> str:
        failures = [step for step in self.steps if step.exit_code != 0]
        return "\n\n".join(
            f"### {step.name}\n命令：`{step.command}`\n退出码：{step.exit_code}\n"
            f"输出：\n{step.output[-6000:]}"
            for step in failures
        )


class ValidationPlanner:
    @staticmethod
    def changed_files(worktree: Path) -> tuple[str, ...]:
        return ChangeSet.capture(worktree).paths

    @staticmethod
    def _changed_go_packages(paths: tuple[str, ...]) -> tuple[str, ...]:
        packages: set[str] = set()
        for path in paths:
            if not path.startswith("backend/") or not path.endswith(".go"):
                continue
            parent = Path(path.removeprefix("backend/")).parent.as_posix()
            packages.add("./..." if parent == "." else f"./{parent}")
        return tuple(sorted(packages))

    @staticmethod
    def _wire_directories(paths: tuple[str, ...]) -> tuple[str, ...]:
        directories: set[str] = {
            Path(path.removeprefix("backend/")).parent.as_posix()
            for path in paths
            if path.startswith("backend/cmd/")
            and (path.endswith("/wire.go") or path.endswith("/wire_gen.go"))
        }
        if any(
            path.startswith("backend/internal/app/service/")
            or path.startswith("backend/internal/server/")
            for path in paths
        ):
            directories.add("cmd/backend")
        if any(
            path.startswith("backend/internal/app/consumer/")
            or path.startswith("backend/internal/clients/eventstream/")
            for path in paths
        ):
            directories.add("cmd/consumer")
        if any(path.startswith("backend/internal/app/job/") for path in paths):
            directories.add("cmd/job")
        if "backend/internal/domain/provider_set.go" in paths:
            directories.update(("cmd/backend", "cmd/consumer", "cmd/job"))
        return tuple(sorted(directories))

    def plan(
        self,
        changed_files: tuple[str, ...],
        requested_command: str,
        issue_body: str,
        untracked_files: tuple[str, ...] = (),
    ) -> tuple[ValidationStep, ...]:
        steps: list[ValidationStep] = []
        api_proto_files = tuple(
            path.removeprefix("backend/")
            for path in changed_files
            if path.startswith("backend/api/") and path.endswith(".proto")
        )
        if api_proto_files:
            quoted = " ".join(shlex.quote(path) for path in api_proto_files)
            api_directories = " ".join(
                shlex.quote(directory)
                for directory in sorted(
                    {Path(path).parent.as_posix() for path in api_proto_files}
                )
            )
            steps.append(
                ValidationStep(
                    "生成 API",
                    "cd backend && protoc --proto_path=./api --proto_path=./third_party "
                    "--go_out=paths=source_relative:./api "
                    "--gin-client-http-leo_out=paths=source_relative:./api "
                    "--go-grpc_out=. "
                    "--go-grpc_opt=module=codeup.aliyun.com/qimao/blog/ai-blog/backend "
                    "--validate_out=paths=source_relative,lang=go:./api "
                    + quoted
                    + " && protoc --proto_path=./api --proto_path=./third_party "
                    "--openapi_out=fq_schema_naming=true,naming=proto,default_response=false:. "
                    + quoted
                    + " && for dir in " + api_directories
                    + "; do find \"$dir\" -name '*.pb.go' -type f -print0 | "
                    "xargs -0 -I{} sh -c 'sed s/,omitempty// \"{}\" > \"{}.tmp\" && mv \"{}.tmp\" \"{}\"'; done",
                )
            )
        if "backend/internal/conf/conf.proto" in changed_files:
            steps.append(
                ValidationStep(
                    "生成配置",
                    "cd backend && protoc --proto_path=./internal --proto_path=./third_party "
                    "--go_out=paths=source_relative:./internal internal/conf/conf.proto",
                )
            )
        wire_dirs = self._wire_directories(changed_files)
        if wire_dirs:
            wire_command = " && ".join(
                f"(cd {shlex.quote(directory)} && wire)"
                for directory in wire_dirs
            )
            steps.append(ValidationStep("生成依赖注入", f"cd backend && {wire_command}"))

        generated_format_dirs: set[str] = set()
        if api_proto_files:
            generated_format_dirs.update(
                Path(path).parent.as_posix()
                for path in api_proto_files
            )
        if "backend/internal/conf/conf.proto" in changed_files:
            generated_format_dirs.add("internal/conf")
        generated_format_dirs.update(wire_dirs)
        if generated_format_dirs:
            directories = " ".join(
                shlex.quote(directory)
                for directory in sorted(generated_format_dirs)
            )
            steps.append(
                ValidationStep(
                    "格式化生成产物",
                    "cd backend && for dir in " + directories
                    + "; do find \"$dir\" -type f \\( -name '*.pb.go' -o -name '*_grpc.pb.go' "
                    "-o -name '*_http.pb.go' -o -name 'validate.pb.go' -o -name 'wire_gen.go' \\) "
                    "-print0 | xargs -0 -r gofmt -w; done",
                )
            )

        go_files = [
            path.removeprefix("backend/")
            for path in changed_files
            if path.startswith("backend/") and path.endswith(".go")
        ]
        if go_files:
            quoted = " ".join(shlex.quote(path) for path in go_files)
            steps.append(
                ValidationStep(
                    "Go 格式检查",
                    f'cd backend && test -z "$(gofmt -l {quoted})"',
                )
            )

        if requested_command.strip():
            steps.append(ValidationStep("工单测试", requested_command.strip()))
        if any(path.startswith("backend/") and path.endswith(".go") for path in changed_files):
            steps.append(ValidationStep("Go Vet", "cd backend && go vet ./..."))

        packages = self._changed_go_packages(changed_files)
        if packages and any(word in issue_body for word in ("并发", "竞态", "并行")):
            steps.append(
                ValidationStep(
                    "竞态测试",
                    "cd backend && go test -race " + " ".join(packages),
                )
            )
        steps.append(ValidationStep("Git 差异检查", "git diff --check"))
        if untracked_files:
            quoted_untracked = " ".join(shlex.quote(path) for path in untracked_files)
            steps.append(
                ValidationStep(
                    "未跟踪文件差异检查",
                    "for file in " + quoted_untracked
                    + '; do if [ -f "$file" ]; then git diff --no-index --check /dev/null "$file"; '
                    + 'code=$?; if [ "$code" -gt 1 ]; then exit "$code"; fi; fi; done',
                )
            )

        unique: list[ValidationStep] = []
        seen: set[str] = set()
        for step in steps:
            if step.command not in seen:
                unique.append(step)
                seen.add(step.command)
        return tuple(unique)


@dataclass(frozen=True)
class ReviewFinding:
    severity: str
    title: str
    location: str
    evidence: str
    recommendation: str


@dataclass(frozen=True)
class ReviewResult:
    verdict: str
    summary: str
    findings: tuple[ReviewFinding, ...]
    raw: str

    @property
    def passed(self) -> bool:
        return self.verdict == "PASS"

    @property
    def has_blocking_findings(self) -> bool:
        return any(finding.severity in {"P0", "P1"} for finding in self.findings)

    def feedback(self, axis: str) -> str:
        lines = [f"## {axis}: {self.verdict}", self.summary]
        for finding in self.findings:
            lines.extend(
                (
                    f"- {finding.severity} · {finding.title}",
                    f"  位置：{finding.location}",
                    f"  依据：{finding.evidence}",
                    f"  建议：{finding.recommendation}",
                )
            )
        return "\n".join(lines)


def parse_review_result(report: str) -> ReviewResult:
    try:
        payload: dict[str, Any] = json.loads(report)
    except (json.JSONDecodeError, TypeError) as exc:
        return ReviewResult(
            "FAIL",
            f"审查报告无法解析为结构化结果：{exc}",
            (),
            report,
        )
    findings = tuple(
        ReviewFinding(
            severity=str(item.get("severity", "P2")),
            title=str(item.get("title", "未命名问题")),
            location=str(item.get("location", "")),
            evidence=str(item.get("evidence", "")),
            recommendation=str(item.get("recommendation", "")),
        )
        for item in payload.get("findings") or []
        if isinstance(item, dict)
    )
    blocking = any(finding.severity in {"P0", "P1"} for finding in findings)
    declared = str(payload.get("verdict", "FAIL")).upper()
    verdict = "FAIL" if blocking or (declared == "FAIL" and not findings) else "PASS"
    return ReviewResult(verdict, str(payload.get("summary", "")), findings, report)
