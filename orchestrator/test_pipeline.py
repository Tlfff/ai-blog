import tempfile
import unittest
import subprocess
import threading
import json
from pathlib import Path
from unittest.mock import patch

from persistence import StateStore
from pipeline import (
    IssuePipelineFlow,
    PipelineConfig,
    PipelineManager,
    PipelineRun,
    prompt_scope_mismatches,
)
from workflow import (
    ChangeSet,
    ContextPackBuilder,
    ReviewFinding,
    ReviewResult,
    ValidationPlanner,
    parse_review_result,
)
from runtime_environment import RuntimeEnvironment


class PipelineTestCase(unittest.TestCase):
    def test_prompt_scope_rejects_cross_issue_worktree_and_target(self):
        prompt = (
            "请接手并完成 Issue #10。\n"
            "工作目录：/repo/.worktrees/issue-10\n"
            "当前分支：codex/issue-10"
        )
        self.assertEqual(prompt_scope_mismatches(14, prompt), (10,))
        with self.assertRaisesRegex(ValueError, "Issue #14.*#10"):
            PipelineConfig(14, "search", "", Path("/tmp/issue-14"), "true", prompt, True)

    def test_prompt_scope_allows_dependency_references(self):
        prompt = "只处理当前 Issue #14；依赖 #7 已完成，不要修改 #10 的评论代码。"
        self.assertEqual(prompt_scope_mismatches(14, prompt), ())
    def make_flow(self, *, auto_approve: bool, matt_workflow: bool = False):
        temp_dir = tempfile.TemporaryDirectory()
        root = Path(temp_dir.name)
        worktree = root / "worktree"
        worktree.mkdir()
        subprocess.run(["git", "init", "-q"], cwd=worktree, check=True)
        subprocess.run(["git", "config", "user.name", "Test"], cwd=worktree, check=True)
        subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=worktree, check=True)
        (worktree / "README.md").write_text("base\n")
        subprocess.run(["git", "add", "README.md"], cwd=worktree, check=True)
        subprocess.run(["git", "commit", "-qm", "base"], cwd=worktree, check=True)
        store_dir = tempfile.TemporaryDirectory()
        self.addCleanup(store_dir.cleanup)
        store = StateStore(Path(store_dir.name) / "state.sqlite3")
        config = PipelineConfig(
            issue_number=3,
            issue_title="login",
            issue_body="",
            worktree_path=worktree,
            test_command="true",
            refactor_prompt="smoke",
            auto_approve=auto_approve,
            matt_workflow=matt_workflow,
        )
        run_id = store.create_run(
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
        return temp_dir, store, IssuePipelineFlow(PipelineRun(config, store, run_id))

    def test_app_server_command_display_is_safe(self):
        temp_dir, _, flow = self.make_flow(auto_approve=True)
        self.addCleanup(temp_dir.cleanup)
        self.assertEqual(
            flow._display_agent_command(read_only=False),
            "codex app-server → thread/resume|start → turn/start (workspace-write)",
        )
        self.assertEqual(
            flow._display_agent_command(read_only=True),
            "codex app-server → thread/resume|start → turn/start (read-only)",
        )

    def test_matt_flow_persists_agent_sessions(self):
        temp_dir, store, flow = self.make_flow(auto_approve=True, matt_workflow=True)
        self.addCleanup(temp_dir.cleanup)
        def fake_codex_turn(prompt, label, process_name, **kwargs):
            flow.run.begin_agent(
                process_name, label,
                "codex app-server → thread/start → turn/start",
                flow.state.status,
                role=kwargs.get("role", "executor"),
                read_only=kwargs.get("read_only", False),
            )
            flow.run.set_session_ids(
                process_name,
                thread_id=f"thread-{process_name}",
                turn_id=f"turn-{process_name}",
            )
            report = (
                '{"verdict":"PASS","summary":"通过","findings":[]}'
                if process_name.startswith("review-")
                else "实现完成"
            )
            flow.run.end_agent(process_name, 0, report)
            return 0, f"thread-{process_name}", f"turn-{process_name}", report

        flow._run_codex_turn = fake_codex_turn
        flow.validation_planner.plan = lambda *args: ()
        flow.kickoff()
        self.assertEqual(flow.run.status, "awaiting_approval")
        sessions = store.list_sessions()
        self.assertEqual(
            {session["agent_key"] for session in sessions},
            {"refactor", "test", "pre-review-validation", "review-standards", "review-spec"},
        )
        self.assertEqual(
            {session["thread_id"] for session in sessions},
            {"thread-refactor", "thread-review-standards", "thread-review-spec", None},
        )
        driver = store.get_session(flow.run.run_id, "refactor")
        self.assertEqual(driver["role"], "driver")
        self.assertEqual(driver["read_only"], 0)
        sidecars = [session for session in sessions if session["agent_key"].startswith("review-")]
        self.assertTrue(all(session["role"] == "review_sidecar" for session in sidecars))
        self.assertTrue(all(session["read_only"] == 1 for session in sidecars))

    def test_review_fail_is_not_complete(self):
        temp_dir, _, flow = self.make_flow(auto_approve=True, matt_workflow=True)
        self.addCleanup(temp_dir.cleanup)
        failed = '{"verdict":"FAIL","summary":"未通过","findings":[]}'
        flow._run_review_agent = lambda axis, cycle=0: (axis, 0, failed)
        flow.state.max_review_cycles = 0
        self.assertEqual(flow.review(), "FAILED")
        self.assertEqual(flow.run.status, "failed")

    def test_invalid_review_without_p0_p1_does_not_trigger_driver_repair(self):
        temp_dir, _, flow = self.make_flow(auto_approve=True, matt_workflow=True)
        self.addCleanup(temp_dir.cleanup)
        invalid = ReviewResult("FAIL", "报告格式错误", (), "not-json")
        flow._run_review_pair = lambda cycle=0: ({}, {"standards": invalid}, [])
        repair_called = False

        def repair(*args):
            nonlocal repair_called
            repair_called = True
            return True

        flow._repair_review_findings = repair
        self.assertEqual(flow.review(), "FAILED")
        self.assertFalse(repair_called)

    def test_review_fail_reworks_then_passes(self):
        temp_dir, _, flow = self.make_flow(auto_approve=True, matt_workflow=True)
        self.addCleanup(temp_dir.cleanup)
        failed = ReviewResult(
            "FAIL",
            "有问题",
            (ReviewFinding("P1", "阻断问题", "a.go", "证据", "修复"),),
            "",
        )
        passed = ReviewResult("PASS", "通过", (), "")
        cycles = iter(
            [
                ({"standards": failed, "spec": passed}),
                ({"standards": passed, "spec": passed}),
            ]
        )
        flow._run_review_pair = lambda cycle=0: ({}, next(cycles), [])
        repair_cycles: list[int] = []
        flow._repair_review_findings = lambda results, cycle: repair_cycles.append(cycle) or True
        flow.state.max_review_cycles = 2
        self.assertEqual(flow.review(), "AWAIT_APPROVAL")
        self.assertEqual(repair_cycles, [1])

    def test_driver_plan_implementation_and_repairs_share_one_thread(self):
        temp_dir, store, flow = self.make_flow(auto_approve=True, matt_workflow=True)
        self.addCleanup(temp_dir.cleanup)

        class FakeClient:
            calls: list[dict[str, object]] = []

            def __init__(self, sink, process_env=None):
                self.sink = sink
                self.process = None

            def start(self):
                return None

            def close(self):
                return None

            def run_turn(self, **kwargs):
                thread_id = kwargs.get("thread_id") or "driver-thread"
                turn_id = f"turn-{len(self.calls) + 1}"
                self.calls.append({**kwargs, "resolved_thread_id": thread_id})
                kwargs["on_thread_id"](thread_id)
                kwargs["on_turn_id"](turn_id)
                return thread_id, turn_id, 0

        with patch("pipeline.AppServerClient", FakeClient):
            for kind in ("driver_plan", "driver_implement", "validation_repair", "review_repair"):
                flow._run_codex_turn(
                    kind,
                    "Codex Driver",
                    "refactor",
                    role="driver",
                    turn_kind=kind,
                )

        self.assertEqual([call["thread_id"] for call in FakeClient.calls], [None, "driver-thread", "driver-thread", "driver-thread"])
        self.assertEqual({turn["thread_id"] for turn in store.turns_for_run(flow.run.run_id)}, {"driver-thread"})
        self.assertEqual(
            [turn["turn_kind"] for turn in store.turns_for_run(flow.run.run_id)],
            ["driver_plan", "driver_implement", "validation_repair", "review_repair"],
        )
        self.assertEqual(
            [session["agent_key"] for session in store.sessions_for_run(flow.run.run_id)],
            ["refactor"],
        )

    def test_pre_review_self_check_uses_same_driver_and_revalidates(self):
        temp_dir, _, flow = self.make_flow(auto_approve=True, matt_workflow=True)
        self.addCleanup(temp_dir.cleanup)
        turns: list[tuple[str, str, str]] = []
        flow._run_codex_turn = lambda prompt, label, process_name, **kwargs: (
            turns.append((process_name, kwargs.get("role"), kwargs.get("turn_kind")))
            or (0, "driver-thread", "self-review-turn", "ok")
        )
        validations: list[str] = []
        flow._run_validation_matrix = lambda process_name, label, stage: (
            validations.append(process_name) or type("Result", (), {"passed": True})()
        )
        self.assertTrue(flow._driver_pre_review_check())
        self.assertEqual(turns, [("refactor", "driver", "driver_pre_review")])
        self.assertEqual(validations, ["pre-review-validation"])

    def test_review_sidecars_are_new_read_only_threads(self):
        temp_dir, store, flow = self.make_flow(auto_approve=True, matt_workflow=True)
        self.addCleanup(temp_dir.cleanup)
        calls: list[dict[str, object]] = []

        def fake_turn(prompt, label, process_name, **kwargs):
            calls.append({"process_name": process_name, **kwargs})
            flow.run.begin_agent(
                process_name,
                label,
                "turn/start",
                flow.state.status,
                role=kwargs["role"],
                read_only=kwargs["read_only"],
            )
            flow.run.set_session_ids(
                process_name,
                thread_id=f"thread-{process_name}",
                turn_id=f"turn-{process_name}",
            )
            flow.run.end_agent(process_name, 0, '{"verdict":"PASS","summary":"ok","findings":[]}')
            return 0, f"thread-{process_name}", f"turn-{process_name}", '{"verdict":"PASS","summary":"ok","findings":[]}'

        flow._run_codex_turn = fake_turn
        flow._run_review_pair(1)
        self.assertEqual({call["process_name"] for call in calls}, {"review-standards-1", "review-spec-1"})
        self.assertTrue(all(call["read_only"] is True for call in calls))
        self.assertTrue(all(call["role"] == "review_sidecar" for call in calls))
        self.assertTrue(all(call["force_new_thread"] is True for call in calls))
        self.assertTrue(all(session["read_only"] == 1 for session in store.sessions_for_run(flow.run.run_id)))

    def test_review_fail_is_returned_to_driver_without_new_writer(self):
        temp_dir, _, flow = self.make_flow(auto_approve=True, matt_workflow=True)
        self.addCleanup(temp_dir.cleanup)
        calls: list[tuple[str, dict[str, object]]] = []
        flow._run_codex_turn = lambda prompt, label, process_name, **kwargs: (
            calls.append((process_name, kwargs)) or (0, "driver-thread", "repair-turn", "")
        )
        flow.validation_planner.plan = lambda *args: ()
        failed = ReviewResult(
            "FAIL",
            "阻断",
            (ReviewFinding("P1", "错误", "a.go", "证据", "修复"),),
            "",
        )
        self.assertTrue(flow._repair_review_findings({"standards": failed}, 1))
        self.assertEqual([name for name, _ in calls], ["refactor"])
        self.assertEqual(calls[0][1]["role"], "driver")
        self.assertEqual(calls[0][1]["turn_kind"], "review_repair")

    def test_only_p0_p1_findings_are_returned_for_automatic_repair(self):
        result = ReviewResult(
            "FAIL",
            "包含阻断项和建议项",
            (
                ReviewFinding("P1", "阻断错误", "a.go", "证据 A", "必须修复"),
                ReviewFinding("P2", "普通问题", "b.go", "证据 B", "建议修复"),
                ReviewFinding("Advisory", "优化建议", "c.go", "证据 C", "可选优化"),
            ),
            "",
        )
        feedback = IssuePipelineFlow._review_feedback({"standards": result})
        self.assertIn("阻断错误", feedback)
        self.assertNotIn("普通问题", feedback)
        self.assertNotIn("优化建议", feedback)

    def test_only_driver_sessions_are_manually_callable(self):
        temp_dir, store, flow = self.make_flow(auto_approve=True, matt_workflow=True)
        self.addCleanup(temp_dir.cleanup)
        store.upsert_session(
            flow.run.run_id,
            "refactor",
            "Codex Driver",
            thread_id="driver-thread",
            role="driver",
            read_only=False,
        )
        store.upsert_session(
            flow.run.run_id,
            "review-spec",
            "规格审查 Agent",
            thread_id="review-thread",
            role="review_sidecar",
            read_only=True,
        )
        manager = PipelineManager(store, enforce_preflight=False)
        self.assertEqual([session["agent_key"] for session in manager.list_sessions()], ["refactor"])
        review_id = store.get_session(flow.run.run_id, "review-spec")["id"]
        with self.assertRaisesRegex(RuntimeError, "只允许调用/继续"):
            manager.call_session(review_id, "修改代码")

    def test_context_pack_includes_backend_rules_and_specs(self):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        root = Path(temp_dir.name)
        for relative in (
            "AGENTS.md",
            "backend/AGENTS.md",
            "backend/功能文档.md",
            "docs/specs/blog-backend-tickets.md",
        ):
            path = root / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(relative)
        pack = ContextPackBuilder(lambda number: ("父规格", "父规格正文")).build(
            root,
            "## Parent\n\n- #1\n",
        )
        self.assertIn("backend/AGENTS.md", pack.source_paths)
        self.assertIn("backend/功能文档.md", pack.source_paths)
        self.assertEqual(pack.parent_issue_number, 1)
        self.assertIn("父规格正文", pack.instructions())

    def test_validation_plan_is_change_aware(self):
        planner = ValidationPlanner()
        steps = planner.plan(
            (
                "backend/api/user/user.proto",
                "backend/internal/conf/conf.proto",
                "backend/internal/app/service/provider_set.go",
                "backend/internal/domain/user/service_test.go",
            ),
            "cd backend && go test ./...",
            "测试覆盖并发查询",
        )
        commands = [step.command for step in steps]
        self.assertTrue(any(command.startswith("cd backend && protoc") for command in commands))
        api_command = next(command for command in commands if "api/user/user.proto" in command)
        self.assertNotIn("make api", api_command)
        self.assertIn("api/user/user.proto", api_command)
        self.assertIn("--openapi_out", api_command)
        self.assertNotIn("$(find api -name '*.proto' -print)", api_command)
        self.assertTrue(any("internal/conf/conf.proto" in command for command in commands))
        self.assertTrue(any("wire)" in command or " && wire" in command for command in commands))
        wire_command = next(command for command in commands if "wire" in command)
        self.assertIn("cmd/backend", wire_command)
        self.assertNotIn("internal/app/service", wire_command)
        generated_format = next(
            step.command for step in steps if step.name == "格式化生成产物"
        )
        self.assertIn("gofmt -w", generated_format)
        self.assertIn("cd backend && go vet ./...", commands)
        self.assertTrue(any("go test -race" in command for command in commands))

    def test_unparseable_review_is_fail_closed(self):
        result = parse_review_result("not json")
        self.assertFalse(result.passed)
        self.assertEqual(result.verdict, "FAIL")

    def test_p2_and_advisory_do_not_block_review(self):
        result = parse_review_result(
            '{"verdict":"FAIL","summary":"建议优化","findings":['
            '{"severity":"P2","title":"命名","location":"x.go",'
            '"evidence":"可读性","recommendation":"重命名"},'
            '{"severity":"Advisory","title":"抽取","location":"y.go",'
            '"evidence":"重复较少","recommendation":"按需处理"}]}'
        )
        self.assertTrue(result.passed)
        self.assertEqual(result.verdict, "PASS")

    def test_p1_blocks_review_even_if_model_declares_pass(self):
        result = parse_review_result(
            '{"verdict":"PASS","summary":"有阻断项","findings":['
            '{"severity":"P1","title":"权限缺失","location":"x.go",'
            '"evidence":"验收要求权限校验","recommendation":"补充校验"}]}'
        )
        self.assertFalse(result.passed)
        self.assertEqual(result.verdict, "FAIL")

    def test_review_schema_limits_findings_and_summary(self):
        temp_dir, _, flow = self.make_flow(auto_approve=True)
        self.addCleanup(temp_dir.cleanup)
        schema = flow._review_output_schema()
        self.assertEqual(schema["properties"]["findings"]["maxItems"], 5)
        self.assertEqual(schema["properties"]["summary"]["maxLength"], 300)

    def test_activity_events_are_persisted(self):
        temp_dir, store, flow = self.make_flow(auto_approve=True)
        self.addCleanup(temp_dir.cleanup)
        flow.run.add_activity("refactor", "重构 Agent", "reasoning", "正在分析认证边界")
        events = store.events_for_run(flow.run.run_id)
        self.assertEqual(len(events), 1)
        self.assertEqual(events[0]["kind"], "reasoning")
        self.assertEqual(events[0]["content"], "正在分析认证边界")

    def test_model_info_is_persisted_for_run_and_agent(self):
        temp_dir, store, flow = self.make_flow(auto_approve=True)
        self.addCleanup(temp_dir.cleanup)
        flow.run.begin_agent("refactor", "重构 Agent", "turn/start", "refactoring")
        flow.run.set_model_info(
            "refactor",
            {
                "model": "gpt-5.6-luna",
                "model_provider": "custom",
                "reasoning_effort": "medium",
            },
        )
        run = store.get_run(flow.run.run_id)
        session = store.get_session(flow.run.run_id, "refactor")
        self.assertEqual(run["model"], "gpt-5.6-luna")
        self.assertEqual(run["model_provider"], "custom")
        self.assertEqual(session["model"], "gpt-5.6-luna")
        self.assertEqual(session["reasoning_effort"], "medium")

    def test_review_collector_uses_completed_final_message(self):
        temp_dir, _, flow = self.make_flow(auto_approve=True)
        self.addCleanup(temp_dir.cleanup)
        collector = ["旧的流式片段"]
        sink = flow._notification_sink("review-spec", "规格审查 Agent", collector)
        sink(
            {
                "method": "item/completed",
                "params": {
                    "item": {
                        "type": "agentMessage",
                        "phase": "final_answer",
                        "text": '{"verdict":"PASS","summary":"完成","findings":[]}',
                    }
                },
            }
        )
        self.assertEqual(
            collector,
            ['{"verdict":"PASS","summary":"完成","findings":[]}'],
        )

    def test_review_output_schema_is_structured(self):
        temp_dir, _, flow = self.make_flow(auto_approve=True)
        self.addCleanup(temp_dir.cleanup)
        schema = flow._review_output_schema()
        self.assertEqual(schema["properties"]["verdict"]["enum"], ["PASS", "FAIL"])
        self.assertIn("findings", schema["required"])

    def test_review_prompt_includes_user_supplement(self):
        temp_dir, _, flow = self.make_flow(auto_approve=True)
        self.addCleanup(temp_dir.cleanup)
        flow.state.refactor_prompt = "常见国家映射即可，未命中返回原名。"
        prompt = flow._review_prompt("spec")
        self.assertIn("最新澄清", prompt)
        self.assertIn("常见国家映射即可", prompt)

    def test_manual_approval_commits_only_after_pass(self):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        root = Path(temp_dir.name)
        subprocess.run(["git", "init", "-q"], cwd=root, check=True)
        subprocess.run(["git", "config", "user.name", "Test"], cwd=root, check=True)
        subprocess.run(["git", "config", "user.email", "test@example.com"], cwd=root, check=True)
        (root / "file.txt").write_text("base\n")
        subprocess.run(["git", "add", "file.txt"], cwd=root, check=True)
        subprocess.run(["git", "commit", "-qm", "base"], cwd=root, check=True)
        (root / "file.txt").write_text("changed\n")
        store_dir = tempfile.TemporaryDirectory()
        self.addCleanup(store_dir.cleanup)
        store = StateStore(Path(store_dir.name) / "state.sqlite3")
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
        manager = PipelineManager(store, enforce_preflight=False)
        sha = manager.approve_commit(run_id)
        self.assertTrue(sha)
        self.assertEqual(store.get_run(run_id)["status"], "completed")
        self.assertEqual(store.get_run(run_id)["commit_sha"], sha)

    def test_manager_runs_independent_worktrees_in_parallel(self):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        root = Path(temp_dir.name)
        store = StateStore(root / "state.sqlite3")
        manager = PipelineManager(store, max_concurrency=2, enforce_preflight=False)
        release = threading.Event()
        manager._execute = lambda run: release.wait(5)

        def config(issue: int) -> PipelineConfig:
            worktree = root / f"issue-{issue}"
            worktree.mkdir()
            return PipelineConfig(issue, f"T{issue}", "", worktree, "true", "", True, True)

        first = manager.start(config(3))
        second = manager.start(config(4))
        self.assertEqual(manager.running_count(), 2)
        self.assertNotEqual(first.runtime_environment.slot, second.runtime_environment.slot)
        self.assertNotEqual(
            first.runtime_environment.compose_project,
            second.runtime_environment.compose_project,
        )
        with self.assertRaisesRegex(RuntimeError, "并发已满"):
            manager.start(config(5))
        release.set()
        first.thread.join(2)
        second.thread.join(2)

    def test_new_run_recovers_existing_issue_driver_thread(self):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        root = Path(temp_dir.name)
        subprocess.run(["git", "init", "-q"], cwd=root, check=True)
        (root / "README.md").write_text("base\n")
        subprocess.run(["git", "add", "README.md"], cwd=root, check=True)
        subprocess.run(
            [
                "git", "-c", "user.name=Test", "-c", "user.email=test@example.com",
                "commit", "-qm", "base",
            ],
            cwd=root,
            check=True,
        )
        store = StateStore(root / "state.sqlite3")
        previous_run = store.create_run(
            {
                "issue_number": 5,
                "issue_title": "Issue 5",
                "issue_body": "",
                "worktree_path": root,
                "test_command": "true",
                "refactor_prompt": "",
                "auto_approve": True,
                "matt_workflow": True,
            }
        )
        store.update_run(previous_run, status="completed", current_stage="completed")
        store.upsert_session(
            previous_run,
            "refactor",
            "Codex Driver",
            thread_id="persistent-driver-thread",
            turn_id="old-turn",
            status="succeeded",
            role="driver",
            read_only=False,
        )
        manager = PipelineManager(store, enforce_preflight=False)
        release = threading.Event()
        manager._execute = lambda run: release.wait(5)
        run = manager.start(PipelineConfig(5, "Issue 5", "", root, "true", "", True, True))
        recovered = store.get_session(run.run_id, "refactor")
        self.assertEqual(recovered["thread_id"], "persistent-driver-thread")
        self.assertEqual(recovered["role"], "driver")
        release.set()
        run.thread.join(2)

    def test_manager_rejects_duplicate_worktree(self):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        root = Path(temp_dir.name)
        worktree = root / "issue-3"
        worktree.mkdir()
        store = StateStore(root / "state.sqlite3")
        manager = PipelineManager(store, max_concurrency=5, enforce_preflight=False)
        release = threading.Event()
        manager._execute = lambda run: release.wait(5)
        config = PipelineConfig(3, "T03", "", worktree, "true", "", True, True)
        first = manager.start(config)
        with self.assertRaisesRegex(RuntimeError, "worktree 已有流水线"):
            manager.start(config)
        release.set()
        first.thread.join(2)

    def test_manager_rejects_same_resource_lock(self):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        root = Path(temp_dir.name)
        store = StateStore(root / "state.sqlite3")
        manager = PipelineManager(store, max_concurrency=5, enforce_preflight=False)
        release = threading.Event()
        manager._execute = lambda run: release.wait(5)
        first_path = root / "issue-3"
        second_path = root / "issue-4"
        first_path.mkdir()
        second_path.mkdir()
        first = manager.start(
            PipelineConfig(
                3, "T03", "", first_path, "true", "", True, True,
                resource_lock="user",
            )
        )
        with self.assertRaisesRegex(RuntimeError, "资源锁 user"):
            manager.start(
                PipelineConfig(
                    4, "T04", "", second_path, "true", "", True, True,
                    resource_lock="user",
                )
            )
        release.set()
        first.thread.join(2)

    def test_concurrency_limit_is_capped_at_five(self):
        with self.assertRaises(ValueError):
            PipelineManager(
                StateStore(Path(tempfile.mkdtemp()) / "state.sqlite3"),
                6,
                enforce_preflight=False,
            )

    def test_runtime_environment_assigns_isolated_names_and_ports(self):
        first = RuntimeEnvironment.create(
            run_id="abc-123", issue_number=3, worktree_path=Path("/tmp/a"), slot=0
        )
        second = RuntimeEnvironment.create(
            run_id="def-456", issue_number=4, worktree_path=Path("/tmp/b"), slot=1
        )
        self.assertNotEqual(first.compose_project, second.compose_project)
        self.assertNotEqual(
            first.process_env()["AI_BLOG_MYSQL_PORT"],
            second.process_env()["AI_BLOG_MYSQL_PORT"],
        )

    def test_workbench_restores_active_run_from_sqlite_after_manager_reload(self):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        root = Path(temp_dir.name)
        worktree = root / "issue-5"
        worktree.mkdir()
        store = StateStore(root / "state.sqlite3")
        run_id = store.create_run(
            {
                "issue_number": 5,
                "issue_title": "T04",
                "issue_body": "",
                "worktree_path": worktree,
                "test_command": "true",
                "refactor_prompt": "",
                "auto_approve": True,
                "matt_workflow": True,
            }
        )
        store.update_run(run_id, status="refactoring", current_stage="refactoring")
        store.upsert_session(
            run_id,
            "refactor",
            "重构 Agent",
            status="running",
            thread_id="thread-5",
            turn_id="turn-5",
            command="turn/start",
        )
        manager = PipelineManager(store, max_concurrency=2, enforce_preflight=False)
        snapshots = manager.workbench_snapshots()
        self.assertEqual(len(snapshots), 1)
        self.assertEqual(snapshots[0]["run_id"], run_id)
        self.assertTrue(snapshots[0]["running"])
        self.assertFalse(snapshots[0]["attached"])
        self.assertEqual(manager.running_count(), 1)

    def test_session_continue_creates_trackable_workbench_run(self):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        root = Path(temp_dir.name)
        worktree = root / "issue-3"
        worktree.mkdir()
        store = StateStore(root / "state.sqlite3")
        parent_run_id = store.create_run(
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
        store.update_run(parent_run_id, status="completed", current_stage="completed")
        store.upsert_session(
            parent_run_id,
            "refactor",
            "重构 Agent",
            status="succeeded",
            thread_id="thread-3",
            turn_id="turn-3",
            role="driver",
            read_only=False,
        )
        session_id = store.get_session(parent_run_id, "refactor")["id"]
        manager = PipelineManager(store, max_concurrency=2, enforce_preflight=False)
        release = threading.Event()
        manager._call_session_worker = lambda *args: release.wait(5)
        continuation_run_id = manager.call_session(session_id, "继续修复")
        row = store.get_run(continuation_run_id)
        self.assertEqual(row["run_kind"], "session")
        self.assertEqual(row["parent_run_id"], parent_run_id)
        self.assertEqual(row["status"], "session_running")
        self.assertEqual(
            {snapshot["run_id"] for snapshot in manager.workbench_snapshots()},
            {continuation_run_id},
        )
        release.set()
        manager._session_calls[continuation_run_id].join(2)

    def test_mark_interrupted_finishes_running_session(self):
        temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(temp_dir.cleanup)
        root = Path(temp_dir.name)
        store = StateStore(root / "state.sqlite3")
        run_id = store.create_run(
            {
                "issue_number": 5,
                "issue_title": "会话继续",
                "issue_body": "",
                "worktree_path": root,
                "test_command": "",
                "refactor_prompt": "",
                "auto_approve": True,
                "matt_workflow": False,
                "run_kind": "session",
            }
        )
        store.update_run(run_id, status="session_running", current_stage="session_running")
        store.upsert_session(run_id, "session-refactor", "重构 Agent · 继续", status="running")
        manager = PipelineManager(store, enforce_preflight=False)
        manager._session_clients[run_id] = type("Client", (), {"close": lambda self: None})()
        manager.mark_interrupted(run_id)
        self.assertEqual(store.get_run(run_id)["status"], "stopped")
        self.assertEqual(store.get_session(run_id, "session-refactor")["status"], "stopped")

    def test_session_thread_id_is_preserved_when_status_changes(self):
        temp_dir, store, flow = self.make_flow(auto_approve=True)
        self.addCleanup(temp_dir.cleanup)
        run_id = flow.run.run_id
        store.upsert_session(run_id, "refactor", "重构 Agent", thread_id="thread-1", turn_id="turn-1")
        store.upsert_session(run_id, "refactor", "重构 Agent", status="succeeded")
        session = store.get_session(run_id, "refactor")
        self.assertEqual(session["thread_id"], "thread-1")
        self.assertEqual(session["turn_id"], "turn-1")
        self.assertEqual(session["status"], "succeeded")

    def test_archive_session_hides_driver_without_deleting_thread(self):
        temp_dir, store, flow = self.make_flow(auto_approve=True)
        self.addCleanup(temp_dir.cleanup)
        run_id = flow.run.run_id
        store.upsert_session(
            run_id,
            "refactor",
            "Codex Driver",
            thread_id="driver-thread",
            turn_id="turn-1",
            status="succeeded",
            role="driver",
            read_only=False,
        )
        session_id = store.get_session(run_id, "refactor")["id"]
        manager = PipelineManager(store, enforce_preflight=False)
        manager.archive_session(session_id)
        self.assertEqual(manager.list_sessions(), [])
        archived = manager.list_all_driver_sessions()
        self.assertEqual(len(archived), 1)
        self.assertEqual(archived[0]["thread_id"], "driver-thread")
        self.assertEqual(archived[0]["archived"], 1)

    def test_reject_run_preserves_worktree_and_marks_decision(self):
        temp_dir, store, flow = self.make_flow(auto_approve=True)
        self.addCleanup(temp_dir.cleanup)
        run_id = flow.run.run_id
        marker = flow.run.config.worktree_path / "change.txt"
        marker.write_text("keep me\n")
        store.update_run(run_id, status="awaiting_approval", current_stage="awaiting_approval")
        manager = PipelineManager(store, enforce_preflight=False)
        manager.reject_run(run_id)
        self.assertTrue(marker.exists())
        self.assertEqual(store.get_run(run_id)["status"], "rejected")
        self.assertTrue(any("Worktree 和代码保持不变" in line for line in store.logs_for_run(run_id)))

    def test_request_review_repair_invalidates_review_and_uses_original_driver_config(self):
        temp_dir, store, flow = self.make_flow(auto_approve=True)
        self.addCleanup(temp_dir.cleanup)
        run_id = flow.run.run_id
        store.update_run(run_id, status="awaiting_approval", current_stage="awaiting_approval")
        manager = PipelineManager(store, enforce_preflight=False)
        captured = []
        fake_run = type("Run", (), {"run_id": "repair-run"})()
        manager.start = lambda config: captured.append(config) or fake_run
        result = manager.request_review_repair(run_id, "只修复人工指出的问题")
        self.assertEqual(result.run_id, "repair-run")
        self.assertEqual(store.get_run(run_id)["status"], "stale_review")
        self.assertEqual(captured[0].issue_number, flow.run.config.issue_number)
        self.assertEqual(captured[0].worktree_path, flow.run.config.worktree_path)
        self.assertEqual(captured[0].refactor_prompt, "只修复人工指出的问题")

    def test_create_pull_request_pushes_issue_branch_and_persists_pr(self):
        temp_dir, store, flow = self.make_flow(auto_approve=True)
        self.addCleanup(temp_dir.cleanup)
        run_id = flow.run.run_id
        worktree = flow.run.config.worktree_path
        head = subprocess.run(
            ["git", "rev-parse", "HEAD"], cwd=worktree, text=True,
            capture_output=True, check=True,
        ).stdout.strip()
        store.update_run(
            run_id,
            status="completed",
            current_stage="completed",
            commit_sha=head,
        )
        manager = PipelineManager(store, enforce_preflight=False)
        calls: list[list[str]] = []

        def fake_command(args, *, cwd=None, timeout=120):
            calls.append(args)
            if args[:4] == ["git", "remote", "get-url", "origin"]:
                return subprocess.CompletedProcess(args, 0, "https://github.com/Tlfff/ai-blog.git\n", "")
            if args[:3] == ["git", "branch", "--show-current"]:
                return subprocess.CompletedProcess(args, 0, "codex/issue-5\n", "")
            if args[:3] == ["git", "rev-parse", "HEAD"]:
                return subprocess.CompletedProcess(args, 0, head + "\n", "")
            if args[:3] == ["git", "status", "--porcelain"]:
                return subprocess.CompletedProcess(args, 0, "", "")
            if args[:3] == ["git", "push", "--set-upstream"]:
                return subprocess.CompletedProcess(args, 0, "", "")
            if args[:3] == ["gh", "pr", "list"]:
                return subprocess.CompletedProcess(args, 0, "[]", "")
            if args[:3] == ["gh", "pr", "create"]:
                return subprocess.CompletedProcess(args, 0, "https://github.com/Tlfff/ai-blog/pull/99\n", "")
            if args[:3] == ["gh", "pr", "view"]:
                return subprocess.CompletedProcess(args, 0, '{"number":99,"url":"https://github.com/Tlfff/ai-blog/pull/99","state":"OPEN"}', "")
            raise AssertionError(args)

        manager._command = fake_command
        result = manager.create_pull_request(run_id)
        self.assertEqual(result["number"], 99)
        row = store.get_run(run_id)
        self.assertEqual(row["pr_number"], 99)
        self.assertEqual(row["pr_url"], "https://github.com/Tlfff/ai-blog/pull/99")
        self.assertTrue(any(args[:3] == ["git", "push", "--set-upstream"] for args in calls))

    def test_merge_pull_request_requires_clean_github_state_and_marks_merged(self):
        temp_dir, store, flow = self.make_flow(auto_approve=True)
        self.addCleanup(temp_dir.cleanup)
        run_id = flow.run.run_id
        worktree = flow.run.config.worktree_path
        head = subprocess.run(
            ["git", "rev-parse", "HEAD"], cwd=worktree, text=True,
            capture_output=True, check=True,
        ).stdout.strip()
        store.update_run(
            run_id,
            status="completed",
            current_stage="completed",
            commit_sha=head,
            pr_number=99,
            pr_url="https://github.com/Tlfff/ai-blog/pull/99",
        )
        manager = PipelineManager(store, enforce_preflight=False)
        states = iter(
            [
                {
                    "number": 99, "url": "https://github.com/Tlfff/ai-blog/pull/99",
                    "state": "OPEN", "baseRefName": "master", "headRefName": "codex/issue-5",
                    "headRefOid": head, "mergeStateStatus": "CLEAN", "statusCheckRollup": [],
                },
                {
                    "number": 99, "url": "https://github.com/Tlfff/ai-blog/pull/99",
                    "state": "MERGED", "baseRefName": "master", "headRefName": "codex/issue-5",
                    "headRefOid": head, "mergeStateStatus": "CLEAN", "statusCheckRollup": [],
                    "mergedAt": "2026-09-05T10:00:00Z", "mergeCommit": {"oid": "merge-sha"},
                },
            ]
        )
        manager._pr_view = lambda run: next(states)
        manager._repository_slug = lambda worktree: "Tlfff/ai-blog"
        manager._command = lambda args, *, cwd=None, timeout=120: subprocess.CompletedProcess(
            args, 0, head + "\n" if args[:3] == ["git", "rev-parse", "HEAD"] else "merged\n", ""
        )
        result = manager.merge_pull_request(run_id)
        self.assertEqual(result["state"], "MERGED")
        self.assertEqual(store.get_run(run_id)["status"], "merged")
        self.assertEqual(store.get_run(run_id)["merge_commit_sha"], "merge-sha")


if __name__ == "__main__":
    unittest.main()
