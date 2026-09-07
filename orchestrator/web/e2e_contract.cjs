const { chromium } = require("playwright");
const assert = require("node:assert/strict");

const baseRun = {
  id: "run-003", issue_number: 3, issue_title: "用户认证与权限模型",
  issue_body: "实现 JWT、权限校验和刷新令牌。", worktree_path: "/tmp/issue-3",
  test_command: "cd backend && go test ./...", refactor_prompt: "", auto_approve: 1,
  matt_workflow: 1, status: "completed", current_stage: "completed",
  created_at: "2026-09-04T01:12:04+00:00", updated_at: "2026-09-04T01:39:12+00:00",
  finished_at: "2026-09-04T01:39:12+00:00", run_kind: "pipeline",
  resource_lock: "user", model: "gpt-5.6-sol", reasoning_effort: "high",
  commit_sha: "a4e31c9", reviewed_fingerprint: "fp-3", reviewed_diff_hash: "hash-3",
};
const reviewRun = {
  ...baseRun, id: "run-008", issue_number: 8, issue_title: "文章编辑与草稿自动保存",
  issue_body: "冲突时允许恢复草稿。", worktree_path: "/tmp/issue-8", status: "awaiting_approval",
  current_stage: "awaiting_approval", resource_lock: "article", commit_sha: null,
  created_at: "2026-09-04T02:10:00+00:00", updated_at: "2026-09-04T02:38:00+00:00",
  finished_at: "2026-09-04T02:38:00+00:00", reviewed_fingerprint: "fp-8",
  reviewed_diff_hash: "hash-8",
};
const activeSnapshot = {
  run_id: "run-005", status: "driver_implement", running: true, attached: true,
  started_at: "2026-09-04T00:22:00+00:00", finished_at: null,
  current_agent: "refactor", current_command: "go test ./internal/biz/...",
  config: {
    issue_number: 5, issue_title: "创建文章与正文图片", issue_body: "上传正文图片并支持失败回滚。",
    worktree_path: "/tmp/issue-5", test_command: "cd backend && go test ./...",
    refactor_prompt: "", auto_approve: true, matt_workflow: true,
    max_review_cycles: 2, resource_lock: "article",
  },
  model_info: { model: "gpt-5.6-sol", reasoning_effort: "high", model_provider: "openai" },
  agents: { refactor: { label: "Codex Driver", status: "running", thread_id: "thread-5", role: "driver", read_only: false, model: "gpt-5.6-sol", reasoning_effort: "high" } },
  activities: [
    { agent_key: "refactor", agent_label: "Codex Driver", kind: "plan", content: "检查依赖\n补充回滚测试\n实现修复", created_at: "2026-09-04T00:30:00+00:00" },
    { agent_key: "refactor", agent_label: "Codex Driver", kind: "action", content: "执行 go test ./internal/biz/...", created_at: "2026-09-04T00:34:02+00:00" },
    { agent_key: "refactor", agent_label: "Codex Driver", kind: "commentary", content: "正在补充失败回滚测试。", created_at: "2026-09-04T00:35:09+00:00" },
  ],
  logs: ["[08:34:02] 启动测试：go test ./internal/biz/...", "[08:34:11] PASS 8.43s"],
  validation_results: [{ name: "工单测试", command: "go test ./...", exit_code: 0 }],
  review_reports: {},
};
activeSnapshot.activities = Array.from({ length: 36 }, (_, index) => ({
  agent_key: "refactor",
  agent_label: "Codex Driver",
  kind: index % 3 === 0 ? "action" : "reasoning",
  content: `持续执行事件 ${index + 1}：检查当前修改并等待最新结果。`,
  created_at: `2026-09-04T00:${String(20 + Math.floor(index / 60)).padStart(2, "0")}:${String(index % 60).padStart(2, "0")}+00:00`,
}));
activeSnapshot.logs = Array.from({ length: 72 }, (_, index) => `[00:${String(Math.floor(index / 60)).padStart(2, "0")}:${String(index % 60).padStart(2, "0")}] ${index % 4 === 0 ? "启动验证命令" : "INFO Driver 正在处理最新事件"}`);
const driverSession = {
  id: 51, run_id: "run-005", issue_number: 5, issue_title: "创建文章与正文图片",
  worktree_path: "/tmp/issue-5", agent_key: "refactor", label: "Codex Driver",
  thread_id: "thread-5", turn_id: "turn-2", status: "running", run_status: "driver_implement",
  role: "driver", read_only: 0, archived: 0, model: "gpt-5.6-sol", reasoning_effort: "high",
  updated_at: "2026-09-04T00:35:09+00:00",
};
const driverSession3 = {
  ...driverSession, id: 31, run_id: "run-003", issue_number: 3,
  issue_title: "用户认证与权限模型", worktree_path: "/tmp/issue-3",
  thread_id: "thread-3", status: "succeeded", run_status: "completed",
};
const reviewSessions = [
  { id: 81, run_id: "run-008", agent_key: "review-standards", label: "Standards", role: "review_sidecar", read_only: 1, status: "succeeded", created_at: "2026-09-04T02:30:00+00:00", updated_at: "2026-09-04T02:32:21+00:00", report: JSON.stringify({ verdict: "PASS", summary: "代码结构和事务边界符合项目规范。", findings: [] }) },
  { id: 82, run_id: "run-008", agent_key: "review-spec", label: "Spec Review", role: "review_sidecar", read_only: 1, status: "succeeded", created_at: "2026-09-04T02:30:00+00:00", updated_at: "2026-09-04T02:32:48+00:00", report: JSON.stringify({ verdict: "PASS", summary: "核心需求已覆盖，有一项恢复路径建议。", findings: [{ severity: "P2", title: "冲突后缺少恢复草稿入口", location: "editor/store.ts:184", evidence: "验收要求可恢复", recommendation: "增加恢复入口" }] }) },
  { ...driverSession, id: 83, run_id: "run-008", status: "succeeded" },
];
const commonEvents = activeSnapshot.activities;
const detail = (run, sessions = [], turns = []) => ({
  run, sessions, turns, events: commonEvents,
  logs: activeSnapshot.logs,
  stats: { files: run.id === "run-008" ? 26 : 18, untracked: 1, additions: 624, deletions: 181, paths: ["backend/a.go", "backend/a_test.go"], generatedPaths: ["backend/api/a.pb.go"], head: "abc", diffHash: "hash" },
});
const details = {
  "run-005": detail({ ...baseRun, id: "run-005", issue_number: 5, issue_title: "创建文章与正文图片", status: "driver_implement", finished_at: null }, [driverSession], [{ role: "driver", turn_kind: "driver_plan", status: "succeeded" }, { role: "driver", turn_kind: "driver_implement", status: "running" }]),
  "run-003": detail(baseRun, [{ ...driverSession, id: 31, run_id: "run-003", issue_number: 3, issue_title: baseRun.issue_title, status: "succeeded" }], [{ role: "driver", turn_kind: "driver_plan", status: "succeeded" }]),
  "run-008": detail(reviewRun, reviewSessions, [{ role: "driver", turn_kind: "driver_implement", status: "succeeded" }]),
};
const bootstrap = {
  repository: "Tlfff/ai-blog", maxConcurrency: 2, running: 1,
  active: [activeSnapshot], runs: [reviewRun, baseRun],
  sessions: [driverSession, driverSession3], archivedSessions: [], issues: [
    { number: 5, title: "创建文章与正文图片", body: "Blocked by #3", labels: [{ name: "ready-for-agent" }] },
    { number: 8, title: "文章编辑与草稿自动保存", body: "冲突时允许恢复草稿", labels: [] },
  ],
  worktrees: { 5: { path: "/tmp/issue-5", branch: "agent/issue-5" }, 8: { path: "/tmp/issue-8", branch: "agent/issue-8" } },
  reviewRuns: [reviewRun], reviewPending: 1,
};

async function main() {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
  const actions = [];
  let prCreated = false;
  let prMerged = false;
  await page.route("**/api/**", async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    if (path === "/api/bootstrap") return route.fulfill({ json: bootstrap });
    if (/^\/api\/runs\/[^/]+$/.test(path)) {
      const id = path.split("/")[3];
      const payload = structuredClone(details[id]);
      if (id === "run-003" && prCreated) {
        payload.run.pr_number = 99;
        payload.run.pr_url = "https://github.com/Tlfff/ai-blog/pull/99";
        payload.pullRequest = { number: 99, url: payload.run.pr_url, state: prMerged ? "MERGED" : "OPEN", mergeCommit: prMerged ? { oid: "merge-sha" } : null };
      }
      return route.fulfill({ json: payload });
    }
    if (/\/diff$/.test(path)) return route.fulfill({ json: { diff: "diff --git a/a.go b/a.go\n+new line" } });
    if (/\/report$/.test(path)) return route.fulfill({ body: JSON.stringify(details["run-003"]), headers: { "Content-Type": "application/json", "Content-Disposition": "attachment; filename=report.json" } });
    if (path === "/api/sessions/51") return route.fulfill({ json: { session: driverSession, runs: [details["run-005"].run], events: commonEvents, logs: activeSnapshot.logs, turns: details["run-005"].turns, activeRunId: "run-005" } });
    if (path === "/api/sessions/31") return route.fulfill({ json: { session: driverSession3, runs: [baseRun], events: commonEvents, logs: activeSnapshot.logs, turns: details["run-003"].turns, activeRunId: null } });
    actions.push({ path, method: route.request().method() });
    if (path.endsWith("/create-pr")) { prCreated = true; return route.fulfill({ json: { pullRequest: { number: 99, url: "https://github.com/Tlfff/ai-blog/pull/99", state: "OPEN" } } }); }
    if (path.endsWith("/merge-pr")) { prMerged = true; return route.fulfill({ json: { pullRequest: { number: 99, url: "https://github.com/Tlfff/ai-blog/pull/99", state: "MERGED", mergeCommit: { oid: "merge-sha" } } } }); }
    return route.fulfill({ json: path.endsWith("/approve") ? { sha: "newsha" } : path === "/api/runs" || path.endsWith("/rerun") || path.endsWith("/repair") || path.endsWith("/continue") ? { runId: "new-run" } : { ok: true, message: "ok" } });
  });
  await page.goto("http://127.0.0.1:8765", { waitUntil: "networkidle" });
  assert.equal(await page.locator(".shell").evaluate((node) => getComputedStyle(node).gridTemplateColumns), "252px 868px 320px");
  assert.equal(await page.locator("h1").textContent(), "创建文章与正文图片");
  const feeds = page.locator('[data-scroll="activity"]');
  const logs = page.locator('[data-scroll="logs"]');
  assert.equal(await feeds.evaluate((node) => node.clientHeight), 244);
  assert.equal(await logs.evaluate((node) => node.clientHeight), 244);
  assert.equal(await feeds.evaluate((node) => node.scrollHeight > node.clientHeight), true);
  assert.equal(await logs.evaluate((node) => node.scrollHeight > node.clientHeight), true);
  await feeds.evaluate((node) => { node.scrollTop = node.scrollHeight; node.dispatchEvent(new Event("scroll")); });
  await logs.evaluate((node) => { node.scrollTop = node.scrollHeight; node.dispatchEvent(new Event("scroll")); });
  await page.locator("#toggle-current-agent").click();
  await page.locator('[data-scroll="activity"]').waitFor();
  await page.waitForTimeout(100);
  assert.equal(await page.locator('[data-scroll="activity"]').evaluate((node) => node.scrollHeight - node.scrollTop - node.clientHeight < 42), true);
  assert.equal(await page.locator('[data-scroll="logs"]').evaluate((node) => node.scrollHeight - node.scrollTop - node.clientHeight < 42), true);
  await page.locator('[data-scroll="activity"]').evaluate((node) => { node.scrollTop = 0; node.dispatchEvent(new Event("scroll")); });
  await page.locator('[data-scroll="logs"]').evaluate((node) => { node.scrollTop = 0; node.dispatchEvent(new Event("scroll")); });
  await page.locator("#toggle-current-agent").click();
  await page.locator('[data-scroll="activity"]').waitFor();
  await page.waitForTimeout(100);
  assert.equal(await page.locator('[data-scroll="activity"]').evaluate((node) => node.scrollTop < 5), true);
  assert.equal(await page.locator('[data-scroll="logs"]').evaluate((node) => node.scrollTop < 5), true);
  await page.screenshot({ path: "/tmp/crewops-run-contract.png", fullPage: true });
  await page.setViewportSize({ width: 1440, height: 900 });

  await page.locator("#view-issue").click();
  await page.locator(".modal-close").click();
  await page.locator("#dependency-action").click();
  await page.locator(".modal-close").click();
  await page.locator('[data-run-tab="changes"]').click();
  await page.locator("#run-diff").waitFor();
  await page.locator('[data-run-tab="artifacts"]').click();
  await page.getByText("backend/api/a.pb.go", { exact: true }).waitFor();
  assert.match(await page.locator("#page-main").innerText(), /a\.pb\.go/);
  await page.locator("#toggle-current-agent").click();
  const runLogDownload = page.waitForEvent("download");
  await page.locator('[data-run-tab="logs"]').click();
  await page.locator("#download-run-log").click();
  await runLogDownload;
  await page.locator("#stop-run").click();

  await page.locator('[data-page="history"]').click();
  await page.locator('#workspace-nav [data-page="history"].active').waitFor({ timeout: 15000 });
  await page.locator("#export-report").waitFor({ timeout: 15000 });
  await page.waitForTimeout(2800);
  await page.screenshot({ path: "/tmp/crewops-history-contract.png", fullPage: true });
  const download = page.waitForEvent("download");
  await page.locator("#export-report").click();
  await download;
  await page.locator('[data-history-tab="changes"]').click();
  await page.locator("#history-diff").waitFor();
  await page.locator('[data-history-tab="logs"]').click();
  await page.locator('[data-history-tab="artifacts"]').click();
  await page.locator('[data-history-tab="summary"]').click();
  await page.locator("#compare-run").click();
  await page.locator(".modal-close").click();
  await page.locator("#cleanup-run").click();
  await page.locator("#create-pr").click();
  await page.locator("#open-pr").waitFor();
  await page.locator("#open-pr").click();
  await page.locator("#merge-pr").click();
  await page.locator("#confirm-ok").click();
  await page.locator("#rerun").click();
  await page.locator('[data-page="history"]').click();
  await page.locator("#open-driver-session").waitFor();
  await page.locator("#open-driver-session").click();
  await page.locator(".sessionbar").waitFor();
  await page.waitForTimeout(2800);
  await page.screenshot({ path: "/tmp/crewops-session-contract.png", fullPage: true });

  await page.locator('[data-session-tab="plan"]').click();
  await page.locator('[data-session-tab="tools"]').click();
  await page.locator('[data-session-tab="context"]').click();
  assert.match(await page.locator("#stop-session").getAttribute("class"), /disabled/);
  await page.locator('[data-record-id="51"]').click();
  await page.locator("#stop-session:not(.disabled)").waitFor();
  await page.locator("#stop-session").click();
  await page.locator("#send-session").click();
  await page.locator('[data-page="sessions"]').click();
  await page.locator('[data-record-id="51"]').click();
  await page.locator("#new-linked-run").click();
  await page.locator('[data-page="sessions"]').click();
  await page.locator('[data-record-id="51"]').click();
  await page.locator("#archive-session").click();
  await page.locator('[data-page="sessions"]').click();
  await page.locator('[data-record-id="51"]').click();
  await page.locator("#delete-session").click();
  await page.locator("#confirm-ok").click();

  await page.locator("#settings-button").click();
  await page.locator("#settings-save").click();
  await page.locator("#global-search").click();
  await page.locator("#global-search-input").fill("用户认证");
  await page.locator(".modal-close").click();
  await page.locator("#new-run-button").click();
  await page.locator(".modal-close").click();

  await page.locator('[data-page="review"]').click();
  await page.locator(".reviewhero").waitFor();
  await page.waitForTimeout(2800);
  await page.screenshot({ path: "/tmp/crewops-review-contract.png", fullPage: true });
  await page.locator("#show-diff").click();
  await page.locator("#review-diff").waitFor();
  await page.locator("#open-worktree").click();
  await page.locator("#only-open").click();
  await page.locator('[data-review-tab="tests"]').click();
  await page.locator('[data-review-tab="conclusion"]').click();
  await page.locator("#request-repair").click();
  await page.locator("#confirm-repair").click();
  await page.locator('[data-page="review"]').click();
  await page.locator("#approve-risk").waitFor();
  await page.locator("#approve-risk").click();
  await page.locator("#reject-run").click();
  await page.locator("#confirm-ok").click();

  assert(actions.some((item) => item.path.endsWith("/stop")));
  assert(actions.some((item) => item.path === "/api/settings"));
  assert(actions.some((item) => item.path.endsWith("/cleanup")));
  assert(actions.some((item) => item.path.endsWith("/archive")));
  assert(actions.some((item) => item.path.endsWith("/delete")));
  assert(actions.some((item) => item.path.endsWith("/continue")));
  assert(actions.some((item) => item.path.endsWith("/open")));
  assert(actions.some((item) => item.path.endsWith("/approve")));
  assert(actions.some((item) => item.path.endsWith("/create-pr")));
  assert(actions.some((item) => item.path.endsWith("/merge-pr")));
  assert(actions.some((item) => item.path.endsWith("/rerun")));
  assert(actions.some((item) => item.path.endsWith("/repair")));
  assert(actions.some((item) => item.path.endsWith("/reject")));
  await page.screenshot({ path: "/tmp/crewops-contract.png", fullPage: true });
  await browser.close();
  console.log("e2e-contract-ok", actions.length);
}

main().catch((error) => { console.error(error); process.exit(1); });
