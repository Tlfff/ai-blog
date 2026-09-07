from __future__ import annotations

import subprocess
import tempfile
import unittest
import json
from pathlib import Path

from persistence import StateStore
from pipeline import PipelineConfig, PipelineManager
from reliability import ToolPreflight, WorktreeManager
from workflow import ChangeSet, ValidationPlanner, sanitize_unrelated_generated_drift


class ReliabilityTests(unittest.TestCase):
    def make_repo(self) -> tuple[tempfile.TemporaryDirectory, Path]:
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        root = Path(temp_dir.name)
        subprocess.run(["git", "init", "-q"], cwd=root, check=True)
        subprocess.run(["git", "config", "user.name", "Test"], cwd=root, check=True)
        subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=root, check=True)
        (root / "tracked.txt").write_text("base\n")
        subprocess.run(["git", "add", "tracked.txt"], cwd=root, check=True)
        subprocess.run(["git", "commit", "-qm", "base"], cwd=root, check=True)
        return temp_dir, root

    def test_change_set_includes_untracked_files_and_content_in_fingerprint(self):
        _, root = self.make_repo()
        (root / "new.go").write_text("package demo\n")
        change_set = ChangeSet.capture(root)
        self.assertIn("new.go", change_set.paths)
        first = change_set.fingerprint
        subprocess.run(["git", "add", "new.go"], cwd=root, check=True)
        self.assertEqual(first, ChangeSet.capture(root).fingerprint)
        subprocess.run(["git", "reset", "-q", "new.go"], cwd=root, check=True)
        (root / "new.go").write_text("package changed\n")
        self.assertNotEqual(first, ChangeSet.capture(root).fingerprint)

    def test_sanitize_restores_unrelated_generated_drift_but_keeps_issue_outputs(self):
        _, root = self.make_repo()
        (root / "backend" / "api" / "user").mkdir(parents=True)
        (root / "backend" / "api" / "book").mkdir(parents=True)
        user_proto = root / "backend" / "api" / "user" / "user.proto"
        unrelated = root / "backend" / "api" / "book" / "book.pb.go"
        related = root / "backend" / "api" / "user" / "user.pb.go"
        user_proto.write_text('syntax = "proto3";\n')
        unrelated.write_text("drift\n")
        related.write_text("expected issue output\n")
        subprocess.run(["git", "add", "backend/api/user/user.proto", "backend/api/book/book.pb.go", "backend/api/user/user.pb.go"], cwd=root, check=True)
        subprocess.run(["git", "commit", "-qm", "generated baseline"], cwd=root, check=True)
        user_proto.write_text('syntax = "proto3";\nmessage User {}\n')
        unrelated.write_text("unrelated changed\n")
        related.write_text("related changed\n")
        restored = sanitize_unrelated_generated_drift(root)
        self.assertEqual(restored, ("backend/api/book/book.pb.go",))
        self.assertEqual(unrelated.read_text(), "drift\n")
        self.assertEqual(related.read_text(), "related changed\n")

    def test_validation_planner_includes_untracked_files(self):
        _, root = self.make_repo()
        (root / "backend" / "api" / "user").mkdir(parents=True)
        (root / "backend" / "api" / "user" / "user.proto").write_text("syntax = \"proto3\";\n")
        paths = ValidationPlanner.changed_files(root)
        self.assertIn("backend/api/user/user.proto", paths)

    def test_untracked_files_receive_whitespace_validation(self):
        planner = ValidationPlanner()
        steps = planner.plan(("new.go",), "true", "", ("new.go",))
        command = next(step.command for step in steps if step.name == "未跟踪文件差异检查")
        self.assertIn("git diff --no-index --check", command)

    def test_review_prompt_explicitly_includes_untracked_files(self):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        root = Path(temp_dir.name)
        worktree = root / "worktree"
        worktree.mkdir()
        subprocess.run(["git", "init", "-q"], cwd=worktree, check=True)
        subprocess.run(["git", "config", "user.name", "Test"], cwd=worktree, check=True)
        subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=worktree, check=True)
        (worktree / "README.md").write_text("base\n")
        subprocess.run(["git", "add", "README.md"], cwd=worktree, check=True)
        subprocess.run(["git", "commit", "-qm", "base"], cwd=worktree, check=True)
        (worktree / "new.go").write_text("package demo\n")
        store = StateStore(root / "state.sqlite3")
        from pipeline import IssuePipelineFlow, PipelineConfig, PipelineRun

        config = PipelineConfig(3, "T02", "", worktree, "true", "", True, True)
        run_id = store.create_run(
            {
                "issue_number": 3,
                "issue_title": "T02",
                "issue_body": "",
                "worktree_path": worktree,
                "test_command": "true",
                "refactor_prompt": "",
                "auto_approve": True,
                "matt_workflow": True,
            }
        )
        prompt = IssuePipelineFlow(PipelineRun(config, store, run_id))._review_prompt("spec")
        self.assertIn("完整变更清单", prompt)
        self.assertIn("new.go", prompt)

    def test_tool_preflight_reports_required_generator_with_install_hint(self):
        checker = ToolPreflight(which=lambda name: None if name == "protoc-gen-gin-client-http-leo" else f"/bin/{name}")
        report = checker.check(
            changed_files=("backend/api/user/user.proto",),
            issue_text="修改用户 Proto 接口",
            has_backend=True,
        )
        self.assertFalse(report.passed)
        missing = next(item for item in report.missing if item.name == "protoc-gen-gin-client-http-leo")
        self.assertIn("go install", missing.install_hint)

    def test_tool_preflight_reports_pinned_generator_version_drift(self):
        checker = ToolPreflight(
            which=lambda name: "/tmp/fake-tool" if name in {"protoc", "protoc-gen-go"} else f"/bin/{name}",
            runner=lambda *args, **kwargs: subprocess.CompletedProcess(
                args[0], 0,
                "libprotoc 35.1\n" if args[0][0].endswith("/protoc") else "protoc-gen-go v1.36.11\n",
                "",
            ),
        )
        report = checker.check(
            changed_files=("backend/api/user/user.proto",),
            issue_text="修改用户 Proto 接口",
            has_backend=True,
        )
        self.assertFalse(report.passed)
        missing = next(item for item in report.missing if item.name == "protoc-gen-go")
        self.assertIn("版本不匹配", missing.reason)

    def test_missing_generator_stops_before_pipeline_or_agent_creation(self):
        _, root = self.make_repo()
        (root / "backend").mkdir()
        checker = ToolPreflight(
            which=lambda name: None if name == "protoc-gen-go" else f"/bin/{name}"
        )
        manager = PipelineManager(StateStore(root / "state.sqlite3"), tool_preflight=checker)
        config = PipelineConfig(
            5,
            "修改 Proto 接口",
            "新增 gRPC Proto",
            root,
            "cd backend && go test ./...",
            "",
            True,
            True,
        )
        with self.assertRaisesRegex(RuntimeError, "启动前工具预检失败"):
            manager.start(config)
        self.assertEqual(manager.store.list_runs(), [])

    def test_approval_rejects_changes_after_review_fingerprint(self):
        _, root = self.make_repo()
        (root / "tracked.txt").write_text("reviewed\n")
        store = StateStore(root / "state.sqlite3")
        run_id = store.create_run(
            {
                "issue_number": 3,
                "issue_title": "T02",
                "issue_body": "",
                "worktree_path": root,
                "test_command": "true",
                "refactor_prompt": "",
                "auto_approve": True,
                "matt_workflow": True,
            }
        )
        reviewed = ChangeSet.capture(root)
        store.update_run(
            run_id,
            status="awaiting_approval",
            current_stage="awaiting_approval",
            reviewed_fingerprint=reviewed.fingerprint,
            reviewed_head=reviewed.head_sha,
            reviewed_paths=json.dumps(reviewed.paths),
            reviewed_diff_hash=reviewed.diff_hash,
        )
        (root / "unreviewed.txt").write_text("late change\n")
        manager = PipelineManager(store)
        with self.assertRaisesRegex(RuntimeError, "审查后发生变化"):
            manager.approve_commit(run_id)
        self.assertEqual(store.get_run(run_id)["status"], "stale_review")

    def test_worktree_manager_recovers_existing_issue_branch(self):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        root = Path(temp_dir.name)
        origin = root / "origin.git"
        source = root / "source"
        worktrees = root / "worktrees"
        subprocess.run(["git", "init", "--bare", "-q", str(origin)], check=True)
        subprocess.run(["git", "clone", "-q", str(origin), str(source)], check=True)
        subprocess.run(["git", "config", "user.name", "Test"], cwd=source, check=True)
        subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=source, check=True)
        (source / "README.md").write_text("base\n")
        subprocess.run(["git", "add", "README.md"], cwd=source, check=True)
        subprocess.run(["git", "commit", "-qm", "base"], cwd=source, check=True)
        subprocess.run(["git", "branch", "-M", "master"], cwd=source, check=True)
        subprocess.run(["git", "push", "-qu", "origin", "master"], cwd=source, check=True)

        manager = WorktreeManager(source, worktrees)
        created = manager.ensure(3)
        self.assertTrue(created.created)
        subprocess.run(["git", "worktree", "remove", str(created.path)], cwd=source, check=True)
        resumed = manager.ensure(3)
        self.assertFalse(resumed.created)
        self.assertEqual(resumed.branch, "codex/issue-3")
        self.assertTrue(resumed.path.exists())


if __name__ == "__main__":
    unittest.main()
