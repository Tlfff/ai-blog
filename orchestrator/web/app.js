const state = {
  page: "run",
  data: null,
  selected: { run: null, history: null, sessions: null, review: null },
  search: "",
  filters: { history: "all", sessions: "all", review: "pending" },
  tabs: { runLog: "logs", history: "summary", session: "messages", review: "findings" },
  onlyCurrentAgent: true,
  onlyOpenFindings: false,
  navigationVersion: 0,
  scroll: { activity: null, logs: null },
  renderVersion: 0,
};

const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];
const escapeHtml = (value) => String(value ?? "").replace(/[&<>"']/g, (char) => ({
  "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#039;",
})[char]);
const statusLabel = {
  queued: "排队中", precheck: "环境预检", driver_plan: "Driver 规划中",
  driver_implement: "Driver 实现中", driver_verify: "Driver 自检中", validate: "确定性验证中",
  driver_repair: "Driver 返工中", targeted_validate: "针对性验证中",
  parallel_review: "双轴审查中", session_running: "会话继续中",
  awaiting_approval: "等待人工决策", stale_review: "审查已失效",
  stopping: "正在停止", stopped: "已停止", completed: "已提交", merged: "已合并到 master",
  failed: "需要人工处理", rejected: "已拒绝",
};
const icons = {
  run: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none"><path d="M4 6h16M4 12h10M4 18h7" stroke="currentColor" stroke-width="1.8" stroke-linecap="round"/></svg>',
  history: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none"><circle cx="12" cy="12" r="8" stroke="currentColor" stroke-width="1.7"/><path d="M12 7v5l3 2" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/></svg>',
  sessions: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none"><path d="M7 17 4 20v-4a7 7 0 1 1 3 1Z" stroke="currentColor" stroke-width="1.7" stroke-linejoin="round"/></svg>',
  review: '<svg width="16" height="16" viewBox="0 0 24 24" fill="none"><path d="M5 7h14M7 12h10M9 17h6" stroke="currentColor" stroke-width="1.7" stroke-linecap="round"/></svg>',
};

async function request(path, options = {}) {
  const response = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  const type = response.headers.get("content-type") || "";
  const body = type.includes("application/json") ? await response.json() : await response.text();
  if (!response.ok) throw new Error(body.error || body || "请求失败");
  return body;
}
const post = (path, payload = {}) => request(path, { method: "POST", body: JSON.stringify(payload) });

function notify(message, error = false) {
  const toast = $("#toast");
  toast.textContent = message;
  toast.style.background = error ? "#d94c59" : "#172033";
  toast.classList.add("show");
  window.setTimeout(() => toast.classList.remove("show"), 2600);
}

function formatDuration(start, end) {
  if (!start) return "—";
  const seconds = Math.max(0, Math.floor((new Date(end || Date.now()) - new Date(start)) / 1000));
  if (seconds >= 3600) return `${Math.floor(seconds / 3600)}h ${String(Math.floor(seconds % 3600 / 60)).padStart(2, "0")}m`;
  if (seconds >= 60) return `${Math.floor(seconds / 60)}m ${String(seconds % 60).padStart(2, "0")}s`;
  return `${seconds}s`;
}
function shortTime(value) { return value ? String(value).slice(11, 19) : "—"; }
function shortDate(value) { return value ? String(value).slice(5, 16).replace("T", " ") : "—"; }
function runCode(run) {
  const raw = String(run.id || run.run_id);
  return `RUN-${raw.replace(/^run-/i, "").slice(0, 6).toUpperCase()}`;
}
function issueFor(number) { return state.data.issues.find((item) => Number(item.number) === Number(number)); }
function issueUrl(number) { return `https://github.com/${state.data.repository}/issues/${number}`; }
function statusDot(status) {
  if (["completed", "awaiting_approval"].includes(status)) return "done";
  if (["failed", "rejected"].includes(status)) return "failed";
  if (["stopped", "stale_review"].includes(status)) return "waiting";
  return "running";
}
function toolCount(detail) {
  return (detail.events || []).filter((event) => event.kind === "action").length
    + (detail.logs || []).filter((line) => /启动|命令|CMD|执行/.test(line)).length;
}
function driverTurns(detail) { return (detail.turns || []).filter((turn) => turn.role === "driver"); }
function terminalRuns() {
  return state.data.runs.filter((run) => ![
    "queued", "precheck", "driver_plan", "driver_implement", "validate",
    "driver_repair", "targeted_validate", "parallel_review", "stopping",
    "session_running", "awaiting_approval", "stale_review",
  ].includes(run.status));
}
function reviewRuns() { return state.data.reviewRuns || []; }

function showModal(title, content, onMount) {
  const layer = $("#modal-layer");
  $("#modal").innerHTML = `<div class="modal-head"><strong>${escapeHtml(title)}</strong><button class="modal-close" aria-label="关闭">×</button></div><div class="modal-body">${content}</div>`;
  layer.hidden = false;
  $(".modal-close").onclick = closeModal;
  layer.onclick = (event) => { if (event.target === layer) closeModal(); };
  if (onMount) onMount($("#modal"));
}
function closeModal() { $("#modal-layer").hidden = true; }

function statCard(label, value, icon = "◷") {
  return `<div class="stat"><div class="stat-icon">${icon}</div><div><div class="stat-label">${escapeHtml(label)}</div><div class="stat-value">${escapeHtml(value)}</div></div></div>`;
}

function renderTopbar() {
  $("#project-name").textContent = state.data.repository;
  $("#global-status").textContent = `服务正常 · 并发 ${state.data.running} / ${state.data.maxConcurrency}`;
}

function renderNavigation() {
  const counts = {
    run: state.data.active.length,
    history: terminalRuns().length,
    sessions: state.data.sessions.length,
    review: state.data.reviewPending,
  };
  const entries = [
    ["run", "运行中心"], ["history", "运行历史"],
    ["sessions", "Agent 会话"], ["review", "审查队列"],
  ];
  $("#workspace-nav").innerHTML = entries.map(([key, label]) => `
    <div class="nav-item ${state.page === key ? "active" : ""}" data-page="${key}">
      ${icons[key]}${label}<span class="badge">${counts[key]}</span>
    </div>`).join("");
  $$('[data-page]').forEach((item) => {
    item.onclick = () => {
      state.page = item.dataset.page;
      state.navigationVersion += 1;
      state.search = "";
      $("#sidebar-search").value = "";
      render();
    };
  });
}

function sidebarConfig() {
  if (state.page === "run") return { title: "最近运行", rows: state.data.active.length ? state.data.active : state.data.runs.slice(0, 8), filters: [] };
  if (state.page === "history") return {
    title: "历史记录", rows: terminalRuns(),
    filters: [["all", "全部"], ["completed", "成功"], ["failed", "失败"], ["stopped", "已停止"], ["rejected", "已拒绝"]],
  };
  if (state.page === "sessions") return {
    title: "会话列表", rows: [...state.data.sessions, ...state.data.archivedSessions],
    filters: [["all", "全部"], ["running", "运行中"], ["continuable", "可继续"], ["archived", "已归档"]],
  };
  return {
    title: "等待决策", rows: reviewRuns(),
    filters: [["pending", "待处理"], ["repair", "返工中"], ["approved", "已批准"], ["rejected", "已拒绝"], ["all", "全部"]],
  };
}

function filteredSidebarRows(config) {
  const query = state.search.trim().toLowerCase();
  let rows = config.rows;
  const filter = state.filters[state.page];
  if (state.page === "history" && filter !== "all") rows = rows.filter((row) => row.status === filter);
  if (state.page === "sessions") {
    if (filter === "running") rows = rows.filter((row) => ["running", "session_running"].includes(row.status) || row.run_status === "session_running");
    if (filter === "continuable") rows = rows.filter((row) => !row.archived && row.thread_id);
    if (filter === "archived") rows = rows.filter((row) => row.archived);
    if (filter === "all") rows = rows.filter((row) => !row.archived);
  }
  if (state.page === "review") {
    if (filter === "pending") rows = rows.filter((row) => ["awaiting_approval", "stale_review"].includes(row.status));
    if (filter === "repair") rows = rows.filter((row) => row.status === "stale_review");
    if (filter === "approved") rows = rows.filter((row) => row.status === "completed");
    if (filter === "rejected") rows = rows.filter((row) => row.status === "rejected");
  }
  if (query) rows = rows.filter((row) => `${row.issue_number || row.config?.issue_number} ${row.issue_title || row.config?.issue_title} ${row.id || row.run_id} ${row.thread_id || ""} ${row.worktree_path || row.config?.worktree_path || ""}`.toLowerCase().includes(query));
  return rows;
}

function renderSidebar() {
  const config = sidebarConfig();
  $("#sidebar-section-title").textContent = config.title;
  const currentFilter = state.filters[state.page];
  $("#sidebar-filters").innerHTML = config.filters.map(([key, label]) => `<button class="filter ${currentFilter === key ? "on" : ""}" data-filter="${key}">${label}</button>`).join("");
  $$('[data-filter]').forEach((button) => {
    button.onclick = () => { state.filters[state.page] = button.dataset.filter; renderSidebar(); renderPage(); };
  });
  const rows = filteredSidebarRows(config);
  const selected = state.selected[state.page];
  $("#sidebar-list").innerHTML = rows.map((row, index) => {
    const id = String(row.id || row.run_id);
    const issue = row.issue_number || row.config?.issue_number;
    const title = row.issue_title || row.config?.issue_title || row.label || "";
    const meta = state.page === "sessions" ? String(row.thread_id || "").slice(0, 12) : (row.resource_lock || row.config?.resource_lock || "");
    const isActive = selected ? selected === id : index === 0;
    const code = state.page === "sessions" ? `THREAD-${id.slice(0, 6).toUpperCase()}` : state.page === "review" ? runCode({ id }).replace("RUN", "REVIEW") : runCode({ id });
    return `<div class="run-card ${isActive ? "active" : ""}" data-record-id="${escapeHtml(id)}">
      <div class="run-top"><span class="status-dot ${statusDot(row.status)}"></span><span class="run-id">#${issue} · ${code}</span><span class="run-state">${escapeHtml(row.archived ? "已归档" : statusLabel[row.status] || row.status)}</span></div>
      <div class="run-name">${escapeHtml(title)}</div><div class="run-meta"><span>${escapeHtml(meta)}</span><span>${shortDate(row.updated_at)}</span></div>
    </div>`;
  }).join("") || '<div class="empty">暂无记录</div>';
  $$('[data-record-id]').forEach((card) => {
    card.onclick = () => { state.selected[state.page] = card.dataset.recordId; render(); };
  });
  if (!state.selected[state.page] && rows.length) state.selected[state.page] = String(rows[0].id || rows[0].run_id);
}

async function loadRun(id) { return request(`/api/runs/${id}`); }
async function loadSession(id) { return request(`/api/sessions/${id}`); }

function coalesceEvents(events) {
  const result = [];
  for (const event of events || []) {
    if (!event.content?.trim()) continue;
    const previous = result.at(-1);
    if (previous && previous.agent_key === event.agent_key && previous.kind === event.kind && ["reasoning", "commentary"].includes(event.kind)) {
      previous.content += event.content;
      previous.created_at = event.created_at;
    } else result.push({ ...event });
  }
  return result;
}

function eventRows(events, limit = 8) {
  return coalesceEvents(events).slice(-limit).map((event) => {
    const iconClass = event.kind === "action" ? "tool" : event.kind === "plan" ? "ok" : "";
    const icon = event.kind === "action" ? "›_" : event.kind === "plan" ? "✓" : "•";
    return `<div class="event"><div class="event-icon ${iconClass}">${icon}</div><div><div class="event-title">${escapeHtml(event.agent_label || "Agent")} · ${escapeHtml(event.kind)}</div><div class="event-body">${escapeHtml(event.content)}</div></div><div class="event-time">${shortTime(event.created_at)}</div></div>`;
  }).join("") || '<div class="empty">等待 Agent 动态…</div>';
}

function flowIndex(status) {
  return ({ queued: 0, precheck: 0, driver_plan: 1, driver_implement: 2, driver_verify: 2, validate: 3, driver_repair: 2, targeted_validate: 3, parallel_review: 4, awaiting_approval: 5, completed: 6 })[status] ?? 0;
}
function flowHtml(status, detail) {
  const current = flowIndex(status);
  const names = ["创建 Worktree", "Driver 规划", "Driver 实现", "验证矩阵", "双轴审查", "人工批准"];
  const driver = driverTurns(detail);
  const durations = ["已就绪", driver.length ? `${driver.length} turns` : "等待", driver.length ? `${driver.length} turns` : "等待", (detail.sessions || []).some((item) => ["test", "retest"].includes(item.agent_key)) ? "已执行" : "等待", (detail.sessions || []).some((item) => item.agent_key.startsWith("review-")) ? "已执行" : "等待", status === "completed" ? "已完成" : "等待"];
  return `<section class="panel flow-panel"><div class="panel-head"><div class="panel-title">执行流程</div><div class="panel-sub">固定流水线 · 自动返工最多 2 轮</div><div class="panel-action" id="dependency-action">查看依赖图</div></div><div class="flow"><span class="flow-progress" style="width:${Math.min(82, current * 16.4)}%"></span>${names.map((name, index) => `<div class="stage ${index < current ? "done" : index === current && current < 6 ? "active" : ""}"><div class="stage-node">${index < current ? "✓" : String(index + 1).padStart(2, "0")}</div><div class="stage-name">${name}</div><div class="stage-status">${index < current ? durations[index] : index === current && current < 6 ? "进行中" : durations[index]}</div></div>`).join("")}<span class="branch-note">失败时进入 Driver 返工 → 针对性验证</span></div></section>`;
}

function logLines(logs) {
  return (logs || []).slice(-100).map((line) => {
    const time = line.match(/^\[([^\]]+)\]/)?.[1] || "";
    const kind = /失败|ERROR/.test(line) ? "WARN" : /退出码：0|PASS|通过/.test(line) ? "PASS" : /启动|命令/.test(line) ? "CMD" : "INFO";
    const klass = kind === "PASS" ? "ok" : kind === "CMD" ? "cmd" : kind === "WARN" ? "warn" : "";
    return `<div class="log-line"><span class="time">${escapeHtml(time)}</span><span class="log-type ${klass}">${kind}</span><span class="cmdtext">${escapeHtml(line.replace(/^\[[^\]]+\]\s*/, ""))}</span></div>`;
  }).join("") || '<div class="empty">等待日志…</div>';
}

function fileList(paths, empty = "没有文件") {
  return paths?.length ? `<div class="activity">${paths.map((path) => `<div class="event"><div class="event-icon tool">↗</div><div><div class="event-title"><code>${escapeHtml(path)}</code></div></div></div>`).join("")}</div>` : `<div class="empty">${empty}</div>`;
}

function issueModal(issueNumber) {
  const issue = issueFor(issueNumber);
  showModal(`Issue #${issueNumber}`, issue ? `<div class="issue-row"><span class="issue-num">ISSUE #${issue.number}</span></div><h1>${escapeHtml(issue.title)}</h1><div class="task-box">${escapeHtml(issue.body || "无正文")}</div><div class="modal-actions"><button class="secondary-btn" id="open-github-issue">在 GitHub 打开 ↗</button></div>` : '<div class="empty">未找到 Issue 数据</div>', () => {
    const button = $("#open-github-issue");
    if (button) button.onclick = () => window.open(issueUrl(issueNumber), "_blank", "noopener");
  });
}

function dependencyModal(issueNumber) {
  const issue = issueFor(issueNumber);
  const matches = [...String(issue?.body || "").matchAll(/#(\d+)/g)].map((match) => Number(match[1]));
  showModal("Issue 依赖图", matches.length ? `<div class="activity">${matches.map((number) => `<div class="event"><div class="event-icon tool">#</div><div><div class="event-title">Issue #${number}</div><div class="event-body">${escapeHtml(issueFor(number)?.title || "未加载或已关闭")}</div></div></div>`).join("")}</div>` : '<div class="empty">当前 Issue 没有声明依赖</div>');
}

function renderRunTabs(snapshot, detail) {
  const tab = state.tabs.runLog;
  const stats = detail.stats || {};
  const body = tab === "logs" ? `<div class="logs" data-scroll="logs">${logLines(snapshot.logs)}</div>`
    : tab === "changes" ? `<pre class="diff-view" id="run-diff">加载 Diff…</pre>`
      : fileList(stats.generatedPaths || [], "没有生成产物");
  return `<section class="panel log-panel"><div class="tabs"><div class="tab ${tab === "logs" ? "active" : ""}" data-run-tab="logs">实时日志</div><div class="tab ${tab === "changes" ? "active" : ""}" data-run-tab="changes">代码变更</div><div class="tab ${tab === "artifacts" ? "active" : ""}" data-run-tab="artifacts">产物</div><div class="log-tools"><span id="download-run-log">下载</span></div></div>${body}</section>`;
}

function runInspector(snapshot, detail) {
  const config = snapshot.config;
  const agent = snapshot.agents?.[snapshot.current_agent] || {};
  const plans = coalesceEvents(snapshot.activities).filter((event) => event.kind === "plan");
  const planLines = String(plans.at(-1)?.content || "").split("\n").filter(Boolean);
  const progress = Math.max(8, Math.min(100, (flowIndex(snapshot.status) + 1) * 16));
  return `<div class="inspect-title">节点检查器 <span>${String(flowIndex(snapshot.status) + 1).padStart(2, "0")} / 06</span></div><div class="agent-card"><div class="agent-top"><div class="avatar">DR</div><div><div class="agent-name">${escapeHtml(agent.label || "Codex Driver")}</div><div class="agent-role">${agent.read_only ? "Read-only Sidecar" : "Implementation Driver"}</div></div><div class="pulse"><i></i>${escapeHtml(statusLabel[snapshot.status] || snapshot.status)}</div></div><div class="agent-progress-label"><span>当前计划：${planLines.length ? `${Math.min(planLines.length, 3)} / ${planLines.length}` : "未记录"}</span><span>${progress}%</span></div><div class="progress"><span style="width:${progress}%"></span></div></div><div class="inspect-section"><div class="inspect-label">运行信息</div><div class="kv"><span>模型</span><strong>${escapeHtml(agent.model || snapshot.model_info?.model || "未记录")}</strong></div><div class="kv"><span>推理强度</span><strong>${escapeHtml(agent.reasoning_effort || snapshot.model_info?.reasoning_effort || "未记录")}</strong></div><div class="kv"><span>会话状态</span><strong style="color:var(--green)">${snapshot.running ? "已连接" : "已结束"}</strong></div><div class="kv"><span>threadId</span><strong>${escapeHtml(String(agent.thread_id || "未记录").slice(0, 18))}</strong></div><div class="kv"><span>Worktree</span><strong>${escapeHtml(config.worktree_path.split("/").pop())}</strong></div><div class="kv"><span>分支</span><strong>${escapeHtml(state.data.worktrees[config.issue_number]?.branch || "未记录")}</strong></div></div><div class="inspect-section"><div class="inspect-label">当前任务</div><div class="task-box">${escapeHtml(snapshot.current_command || plans.at(-1)?.content || "等待调度")}</div></div><div class="inspect-section"><div class="inspect-label">本节点统计</div><div class="kv"><span>耗时</span><strong>${formatDuration(snapshot.started_at, snapshot.finished_at)}</strong></div><div class="kv"><span>Token</span><strong>未记录</strong></div><div class="kv"><span>工具调用</span><strong>${toolCount(detail)}</strong></div><div class="kv"><span>重试</span><strong>${detail.turns.filter((turn) => /repair/.test(turn.turn_kind)).length}</strong></div></div><div class="approval"><strong>安全门禁</strong><br>代码修改完成后仍需通过验证矩阵与双轴审查，最后由人工批准提交。</div><div class="footer-note">CrewOps · ${new Date().toISOString().slice(0, 10)}</div>`;
}

function nearBottom(element) {
  return element.scrollHeight - element.scrollTop - element.clientHeight < 42;
}

function rememberLiveScroll() {
  for (const key of ["activity", "logs"]) {
    const element = document.querySelector(`[data-scroll="${key}"]`);
    if (!element) continue;
    state.scroll[key] = { top: element.scrollTop, follow: nearBottom(element) };
  }
}

function restoreLiveScroll() {
  for (const key of ["activity", "logs"]) {
    const element = document.querySelector(`[data-scroll="${key}"]`);
    if (!element) continue;
    const saved = state.scroll[key] || { top: 0, follow: true };
    const update = () => {
      if (saved.follow) element.scrollTop = element.scrollHeight;
      else element.scrollTop = Math.min(saved.top, Math.max(0, element.scrollHeight - element.clientHeight));
    };
    element.addEventListener("scroll", () => {
      state.scroll[key] = { top: element.scrollTop, follow: nearBottom(element) };
    }, { passive: true });
    requestAnimationFrame(() => requestAnimationFrame(update));
  }
}

async function renderRunPage(version) {
  rememberLiveScroll();
  const rows = filteredSidebarRows(sidebarConfig());
  const snapshot = rows.find((row) => String(row.run_id || row.id) === state.selected.run) || state.data.active[0];
  if (!snapshot || !snapshot.run_id) {
    $("#page-main").innerHTML = `<div class="breadcrumb">运行中心</div><div class="issue-head"><div class="issue-main"><div class="issue-row"><span class="issue-num">WORKSPACE</span></div><h1>运行中心</h1><div class="issue-desc">当前没有活动运行。点击右上角“启动新工单”创建独立 Driver。</div></div></div><div class="stats">${statCard("活动运行", "0", "◷")}${statCard("Token 消耗", "未记录", "⌁")}${statCard("命令 / 工具调用", "0", "›_")}${statCard("代码变更", "0 files", "≋")}</div><div class="notice green">一个 Issue 只会拥有一个可写 Driver Thread；不同 Worktree 可以并行。</div>`;
    $("#page-inspector").innerHTML = '<div class="inspect-title">节点检查器 <span>实时</span></div><div class="notice amber">选择 Issue 并启动后，这里会显示当前 Driver、模型、Worktree 和命令。</div>';
    return;
  }
  const detail = await loadRun(snapshot.run_id);
  if (version !== state.renderVersion || state.page !== "run") return;
  const config = snapshot.config;
  const currentEvents = state.onlyCurrentAgent && snapshot.current_agent ? snapshot.activities.filter((event) => event.agent_key === snapshot.current_agent) : snapshot.activities;
  $("#page-main").innerHTML = `<div class="breadcrumb">运行中心 / ${runCode(snapshot)}</div><div class="issue-head"><div class="issue-main"><div class="issue-row"><span class="issue-num">ISSUE #${config.issue_number}</span><span class="issue-tag">${escapeHtml(statusLabel[snapshot.status] || snapshot.status)}</span></div><h1>${escapeHtml(config.issue_title)}</h1><div class="issue-desc">一个 Issue · 一个独立 Worktree · 一个持续可恢复 Driver Thread</div></div><div class="head-actions"><button class="secondary-btn" id="view-issue">查看 Issue ↗</button><button class="danger-btn ${snapshot.running ? "" : "disabled"}" id="stop-run">停止运行</button></div></div><div class="stats">${statCard("运行时长", formatDuration(snapshot.started_at, snapshot.finished_at), "◷")}${statCard("Token 消耗", "未记录", "⌁")}${statCard("命令 / 工具调用", `${toolCount(detail)} 次`, "›_")}${statCard("代码变更", `${detail.stats.files} files`, "≋")}</div>${flowHtml(snapshot.status, detail)}<div class="work-grid"><section class="panel"><div class="panel-head"><div class="panel-title">Agent 动态</div><div class="panel-sub">结构化事件流</div><div class="panel-action" id="toggle-current-agent">${state.onlyCurrentAgent ? "查看全部 Agent" : "仅看当前 Agent"}</div></div><div class="activity activity-scroll" data-scroll="activity">${eventRows(currentEvents)}</div></section>${renderRunTabs(snapshot, detail)}</div>`;
  $("#page-inspector").innerHTML = runInspector(snapshot, detail);
  restoreLiveScroll();
  $("#view-issue").onclick = () => issueModal(config.issue_number);
  $("#stop-run").onclick = async () => { if (!snapshot.running) return; await post(`/api/runs/${snapshot.run_id}/stop`); notify("已请求停止运行"); await refresh(); };
  $("#dependency-action").onclick = () => dependencyModal(config.issue_number);
  $("#toggle-current-agent").onclick = () => { state.onlyCurrentAgent = !state.onlyCurrentAgent; renderPage(); };
  $$('[data-run-tab]').forEach((tab) => tab.onclick = () => { state.tabs.runLog = tab.dataset.runTab; renderPage(); });
  $("#download-run-log").onclick = () => downloadText(`${runCode(snapshot)}-logs.txt`, (snapshot.logs || []).join("\n"));
  if (state.tabs.runLog === "changes") {
    const diff = await request(`/api/runs/${snapshot.run_id}/diff`);
    $("#run-diff").textContent = diff.diff || "无差异";
  }
}

function downloadText(filename, content, type = "text/plain") {
  const link = document.createElement("a");
  link.href = URL.createObjectURL(new Blob([content], { type }));
  link.download = filename;
  link.click();
  URL.revokeObjectURL(link.href);
}

function downloadUrl(url) {
  const link = document.createElement("a");
  link.href = url;
  link.download = "";
  document.body.appendChild(link);
  link.click();
  link.remove();
}

function historyInspector(run, detail) {
  const pr = detail.pullRequest || (run.pr_number ? { number: run.pr_number, url: run.pr_url, state: "OPEN" } : null);
  const prState = String(pr?.state || "").toUpperCase();
  const prActions = run.status === "completed" && !pr
    ? '<button class="widebtn primary" id="create-pr">提交分支并创建 PR</button>'
    : prState === "OPEN"
      ? `<button class="widebtn secondary" id="open-pr">打开 PR #${pr.number}</button><button class="widebtn primary" id="merge-pr">合并 PR 到远程 master</button>`
      : prState === "MERGED" || run.status === "merged"
        ? `<div class="notice green"><strong>PR 已合并到远程 master</strong><br><code>${escapeHtml(run.merge_commit_sha || "已合并")}</code></div>`
        : run.status === "completed" ? '<div class="notice amber">当前运行还没有 PR；请先创建 PR。</div>' : '';
  return `<div class="ititle">运行详情 <span>只读快照</span></div><div class="section"><div class="scap">关联信息</div><div class="kv"><span>Issue</span><strong>#${run.issue_number} ${escapeHtml(run.issue_title)}</strong></div><div class="kv"><span>Worktree</span><strong>${escapeHtml(run.worktree_path.split("/").pop())}</strong></div><div class="kv"><span>分支</span><strong>${escapeHtml(state.data.worktrees[run.issue_number]?.branch || "未记录")}</strong></div><div class="kv"><span>触发方式</span><strong>${run.run_kind === "session" ? "会话继续" : "手动启动"}</strong></div><div class="kv"><span>运行模型</span><strong>${escapeHtml(run.model || "未记录")}</strong></div></div><div class="section"><div class="scap">代码结果</div><div class="kv"><span>变更文件</span><strong>${detail.stats.files}</strong></div><div class="kv"><span>新增 / 删除</span><strong style="color:var(--green)">+${detail.stats.additions}</strong><strong style="color:var(--red)">−${detail.stats.deletions}</strong></div><div class="kv"><span>最终提交</span><strong><code>${escapeHtml(run.commit_sha || "未提交")}</code></strong></div><div class="kv"><span>PR</span><strong>${pr ? `#${pr.number}` : "未创建"}</strong></div><div class="kv"><span>合并状态</span><strong>${escapeHtml(statusLabel[run.status] || "未合并")}</strong></div></div>${prActions}<div class="notice green"><strong>完整运行快照</strong><br>日志、Agent 事件和审查结论均已持久化，可用于回放和审计。</div><button class="widebtn secondary" id="open-driver-session">打开对应 Agent 会话</button><button class="widebtn secondary" id="compare-run">比较另一次运行</button><div class="section"><div class="scap">环境清理</div><label class="checkline"><input type="checkbox" id="remove-volumes">同时删除数据库 volumes</label><button class="widebtn danger" id="cleanup-run">清理 Compose 环境</button></div><div class="footer-note">CrewOps · ${new Date().toISOString().slice(0, 10)}</div>`;
}

function historyAgentTable(detail) {
  return `<section class="panel summarybox"><div class="panel-head"><div class="panel-title">Agent 与阶段统计</div><div class="panel-action" id="download-csv">下载 CSV</div></div><div class="agenttable"><div class="atablehead"><span>执行单元</span><span>角色</span><span>结果</span><span>耗时</span></div>${detail.sessions.map((session) => `<div class="atablerow"><div class="agentcell"><i class="miniavatar">${session.read_only ? "RO" : "DR"}</i>${escapeHtml(session.label)}</div><span>${escapeHtml(session.role)}</span><span class="good">${escapeHtml(session.status)}</span><span>${session.created_at && session.updated_at ? formatDuration(session.created_at, session.updated_at) : "—"}</span></div>`).join("")}</div></section>`;
}

function historySummary(detail) {
  const events = coalesceEvents(detail.events).slice(-3);
  return `<div class="summarylist">${events.map((event) => `<div class="sumrow"><div class="sumicon">✓</div><div><div class="sumtitle">${escapeHtml(event.agent_label)} · ${escapeHtml(event.kind)}</div><div class="sumdesc">${escapeHtml(event.content.slice(0, 180))}</div></div><div class="sumtime">${shortTime(event.created_at)}</div></div>`).join("") || '<div class="empty">没有运行摘要</div>'}</div>`;
}

function historyTabContent(detail) {
  if (state.tabs.history === "summary") return historySummary(detail);
  if (state.tabs.history === "logs") return `<div class="logs" style="height:139px">${logLines(detail.logs)}</div>`;
  if (state.tabs.history === "changes") return '<pre class="diff-view" id="history-diff" style="height:139px">加载 Diff…</pre>';
  return fileList(detail.stats.generatedPaths, "没有生成产物");
}

function historyTabBody(detail) {
  return `<div class="summarygrid"><section class="panel summarybox"><div class="tabs"><div class="tab ${state.tabs.history === "summary" ? "active" : ""}" data-history-tab="summary">运行摘要</div><div class="tab ${state.tabs.history === "logs" ? "active" : ""}" data-history-tab="logs">日志</div><div class="tab ${state.tabs.history === "changes" ? "active" : ""}" data-history-tab="changes">代码变更</div><div class="tab ${state.tabs.history === "artifacts" ? "active" : ""}" data-history-tab="artifacts">产物</div></div>${historyTabContent(detail)}</section>${historyAgentTable(detail)}</div>`;
}

async function renderHistoryPage(version) {
  const rows = filteredSidebarRows(sidebarConfig());
  let run = rows.find((item) => String(item.id) === state.selected.history) || rows[0];
  if (!run) { $("#page-main").innerHTML = '<div class="empty">暂无历史运行</div>'; $("#page-inspector").innerHTML = '<div class="ititle">运行详情</div>'; return; }
  state.selected.history = String(run.id);
  const detail = await loadRun(run.id);
  if (version !== state.renderVersion || state.page !== "history") return;
  run = detail.run;
  $("#page-main").innerHTML = `<div class="crumb">运行历史 / ${runCode(run)}</div><div class="header"><div class="hmain"><div class="eyebrow"><span class="tag">${runCode(run)}</span><span class="tag green">${escapeHtml(statusLabel[run.status] || run.status)}</span></div><h1>${escapeHtml(run.issue_title)}</h1><div class="subtitle">Issue #${run.issue_number} · ${escapeHtml(run.worktree_path.split("/").pop())} · ${run.run_kind === "session" ? "会话继续触发" : "由工单执行器触发"}</div></div><div class="actions"><button class="btn" id="export-report">导出报告</button><button class="btn purple" id="rerun">基于此运行重跑</button></div></div><div class="cards">${statCard("开始时间", shortTime(run.created_at), "◷")}${statCard("总耗时", formatDuration(run.created_at, run.finished_at), "◷")}${statCard("Token 消耗", "未记录", "⌁")}${statCard("最终提交", run.commit_sha || "未提交", "✓")}</div>${flowHtml(run.status, detail)}<div id="history-tab-body">${historyTabBody(detail)}</div>`;
  $("#page-inspector").innerHTML = historyInspector(run, detail);
  $("#export-report").onclick = () => downloadUrl(`/api/runs/${run.id}/report`);
  $("#rerun").onclick = async () => { try { const result = await post(`/api/runs/${run.id}/rerun`); notify(`已启动 ${result.runId.slice(0, 8)}`); state.page = "run"; await refresh(); } catch (error) { notify(error.message, true); } };
  $$('[data-history-tab]').forEach((tab) => tab.onclick = () => { state.tabs.history = tab.dataset.historyTab; renderPage(); });
  $("#open-driver-session").onclick = () => {
    const session = state.data.sessions.find((item) => item.run_id === run.id) || state.data.sessions.find((item) => item.issue_number === run.issue_number);
    if (!session) return notify("没有可恢复的 Driver 会话", true);
    state.page = "sessions"; state.selected.sessions = String(session.id); render();
  };
  $("#compare-run").onclick = () => compareRunModal(run, detail);
  $("#cleanup-run").onclick = async () => { try { const result = await post(`/api/runs/${run.id}/cleanup`, { removeVolumes: $("#remove-volumes").checked }); notify(result.message); } catch (error) { notify(error.message, true); } };
  const createPr = $("#create-pr");
  if (createPr) createPr.onclick = async () => { try { const result = await post(`/api/runs/${run.id}/create-pr`); notify(`PR #${result.pullRequest.number} 已创建`); await renderPage(); } catch (error) { notify(error.message, true); } };
  const openPr = $("#open-pr");
  if (openPr) openPr.onclick = () => { if (detail.pullRequest?.url) window.open(detail.pullRequest.url, "_blank", "noopener"); else notify("PR 地址未记录", true); };
  const mergePr = $("#merge-pr");
  if (mergePr) mergePr.onclick = () => confirmModal("合并 PR 到远程 master", "将读取最新 GitHub 检查状态，并使用 gh pr merge --merge 合并；不会删除 Worktree 或关闭 Issue。", async () => { const result = await post(`/api/runs/${run.id}/merge-pr`); closeModal(); notify(`PR 已合并 ${result.pullRequest.mergeCommit?.oid?.slice(0, 8) || "完成"}`); await renderPage(); });
  const csv = $("#download-csv");
  if (csv) csv.onclick = () => downloadText(`${runCode(run)}-agents.csv`, `label,role,status\n${detail.sessions.map((item) => `"${item.label}",${item.role},${item.status}`).join("\n")}`, "text/csv");
  if (state.tabs.history === "changes") {
    const diff = await request(`/api/runs/${run.id}/diff`); $("#history-diff").textContent = diff.diff || "无差异";
  }
}

function compareRunModal(run, detail) {
  const options = terminalRuns().filter((item) => item.id !== run.id);
  showModal("比较另一次运行", `<div class="field"><label>对比运行</label><select id="compare-select">${options.map((item) => `<option value="${item.id}">${escapeHtml(runCode(item))} · #${item.issue_number} ${escapeHtml(item.issue_title)}</option>`).join("")}</select></div><div id="compare-result" class="compare-grid" style="margin-top:14px"><div class="compare-card"><strong>${escapeHtml(runCode(run))}</strong><div class="kv"><span>状态</span><strong>${escapeHtml(statusLabel[run.status] || run.status)}</strong></div><div class="kv"><span>耗时</span><strong>${formatDuration(run.created_at, run.finished_at)}</strong></div><div class="kv"><span>文件</span><strong>${detail.stats.files}</strong></div></div></div>`, (modal) => {
    const select = $("#compare-select", modal);
    const update = async () => {
      const other = options.find((item) => item.id === select.value); if (!other) return;
      const otherDetail = await loadRun(other.id);
      $("#compare-result", modal).innerHTML = `<div class="compare-card"><strong>${escapeHtml(runCode(run))}</strong><div class="kv"><span>状态</span><strong>${escapeHtml(statusLabel[run.status] || run.status)}</strong></div><div class="kv"><span>耗时</span><strong>${formatDuration(run.created_at, run.finished_at)}</strong></div><div class="kv"><span>文件</span><strong>${detail.stats.files}</strong></div></div><div class="compare-card"><strong>${escapeHtml(runCode(other))}</strong><div class="kv"><span>状态</span><strong>${escapeHtml(statusLabel[other.status] || other.status)}</strong></div><div class="kv"><span>耗时</span><strong>${formatDuration(other.created_at, other.finished_at)}</strong></div><div class="kv"><span>文件</span><strong>${otherDetail.stats.files}</strong></div></div>`;
    };
    select.onchange = update; update();
  });
}

function sessionMessages(detail) {
  return coalesceEvents(detail.events).map((event) => `<div class="msg agent"><div class="mavatar">DR</div><div class="bubble"><div class="mhead">Codex Driver <span>${shortTime(event.created_at)} · ${escapeHtml(event.kind)}</span></div>${escapeHtml(event.content)}${event.kind === "action" ? `<div class="toolbox"><div class="toolhead">执行动作 <b>记录</b></div><div class="toolcmd">${escapeHtml(event.content)}</div></div>` : ""}</div></div>`).join("") || '<div class="empty">该 Driver 没有持久化的消息。</div>';
}
function sessionTabBody(detail) {
  if (state.tabs.session === "messages") return sessionMessages(detail);
  if (state.tabs.session === "plan") {
    const plans = detail.events.filter((event) => event.kind === "plan");
    return plans.length ? plans.map((event) => `<div class="msg agent"><div class="mavatar">PL</div><div class="bubble"><div class="mhead">计划更新 <span>${shortTime(event.created_at)}</span></div>${escapeHtml(event.content)}</div></div>`).join("") : '<div class="empty">没有持久化的计划。</div>';
  }
  if (state.tabs.session === "tools") {
    const actions = detail.events.filter((event) => event.kind === "action");
    return actions.length ? actions.map((event) => `<div class="msg agent"><div class="mavatar">›_</div><div class="bubble"><div class="mhead">工具调用 <span>${shortTime(event.created_at)}</span></div><div class="toolbox"><div class="toolcmd">${escapeHtml(event.content)}</div></div></div></div>`).join("") : '<div class="empty">没有持久化的工具调用。</div>';
  }
  return `<div class="empty">上下文 Token、缓存命中率和模型窗口未由 App Server 持久化，因此不伪造数值。</div>`;
}

function sessionInspector(detail) {
  const session = detail.session;
  return `<div class="ititle">会话检查器 <span>${detail.activeRunId ? "实时" : "可恢复"}</span></div><div class="section"><div class="scap">上下文使用</div><div class="context"><div class="barlabel"><span>已使用：未记录</span><span>—</span></div><div class="bar"><span style="width:0"></span></div></div><div class="kv"><span>模型窗口</span><strong>未记录</strong></div><div class="kv"><span>缓存命中</span><strong>未记录</strong></div><div class="kv"><span>推理 Token</span><strong>未记录</strong></div></div><div class="section"><div class="scap">会话属性</div><div class="kv"><span>Agent</span><strong>Codex Driver</strong></div><div class="kv"><span>模型</span><strong>${escapeHtml(session.model || "未记录")}</strong></div><div class="kv"><span>推理强度</span><strong>${escapeHtml(session.reasoning_effort || "未记录")}</strong></div><div class="kv"><span>Worktree</span><strong>${escapeHtml(session.worktree_path.split("/").pop())}</strong></div><div class="kv"><span>会话状态</span><strong style="color:var(--green)">${detail.activeRunId ? "running" : "continuable"}</strong></div></div><div class="section"><div class="scap">关联运行</div>${detail.runs.slice(-4).map((run) => `<div class="linked"><div class="linkedtop">${runCode(run)} <span>${escapeHtml(statusLabel[run.status] || run.status)}</span></div><div class="linkedbody">${escapeHtml(run.issue_title)} · ${shortDate(run.updated_at)}</div></div>`).join("")}</div><div class="notice amber"><strong>状态说明</strong><br>Pipeline 与 Driver Thread 是两层状态：流水线结束后，会话仍可被继续调用。</div><button class="widebtn secondary" id="new-linked-run">新建关联运行</button><button class="widebtn secondary" id="archive-session">归档会话</button><button class="widebtn danger" id="delete-session">删除会话</button><div class="footer-note">CrewOps · ${new Date().toISOString().slice(0, 10)}</div>`;
}

async function renderSessionsPage(version) {
  const rows = filteredSidebarRows(sidebarConfig());
  const session = rows.find((item) => String(item.id) === state.selected.sessions) || rows[0];
  if (!session) { $("#page-main").innerHTML = '<div class="empty">暂无 Driver 会话</div>'; $("#page-inspector").innerHTML = '<div class="ititle">会话检查器</div>'; return; }
  state.selected.sessions = String(session.id);
  const detail = await loadSession(session.id);
  if (version !== state.renderVersion || state.page !== "sessions") return;
  $("#page-main").innerHTML = `<div class="crumb">Agent 会话 / ${escapeHtml(String(session.thread_id).slice(0, 12))}</div><div class="sessionbar"><div class="bigavatar">DR</div><div><div class="sessionname">Codex Driver · 会话继续</div><div class="sessionmeta">Issue #${session.issue_number} · ${detail.runs.map(runCode).join(" · ")} · Thread ${escapeHtml(session.thread_id)}</div></div><div class="live"><i></i>${detail.activeRunId ? "正在执行" : "可继续"}</div><button class="btn red ${detail.activeRunId ? "" : "disabled"}" id="stop-session">停止调用</button></div><section class="panel conversation"><div class="tabs"><div class="tab ${state.tabs.session === "messages" ? "active" : ""}" data-session-tab="messages">消息与执行</div><div class="tab ${state.tabs.session === "plan" ? "active" : ""}" data-session-tab="plan">计划</div><div class="tab ${state.tabs.session === "tools" ? "active" : ""}" data-session-tab="tools">工具调用 ${detail.events.filter((event) => event.kind === "action").length}</div><div class="tab ${state.tabs.session === "context" ? "active" : ""}" data-session-tab="context">上下文</div></div><div class="messages">${sessionTabBody(detail)}</div><div class="composer"><textarea class="input" id="session-prompt" placeholder="输入继续指令…">请读取当前 worktree 和 git diff，继续完成尚未完成的工作；不要提交或合并。</textarea><button class="send" id="send-session">发送调用</button></div></section>`;
  $("#page-inspector").innerHTML = sessionInspector(detail);
  $$('[data-session-tab]').forEach((tab) => tab.onclick = () => { state.tabs.session = tab.dataset.sessionTab; renderPage(); });
  const continueSession = async () => { try { const result = await post(`/api/sessions/${session.id}/continue`, { prompt: $("#session-prompt").value.trim() }); notify(`已启动 ${result.runId.slice(0, 8)}`); state.page = "run"; await refresh(); } catch (error) { notify(error.message, true); } };
  $("#send-session").onclick = continueSession;
  $("#new-linked-run").onclick = continueSession;
  $("#stop-session").onclick = async () => { if (!detail.activeRunId) return; try { await post(`/api/sessions/${session.id}/stop`); notify("已停止会话调用"); await refresh(); } catch (error) { notify(error.message, true); } };
  $("#archive-session").onclick = async () => { try { await post(`/api/sessions/${session.id}/archive`); notify("会话已归档"); await refresh(); } catch (error) { notify(error.message, true); } };
  $("#delete-session").onclick = () => confirmModal("删除会话", "将删除 Codex Thread 和本地会话记录，但不会删除 Worktree。", async () => { await post(`/api/sessions/${session.id}/delete`); notify("会话已删除"); closeModal(); await refresh(); });
}

function parseReports(detail) {
  const reports = {};
  for (const session of detail.sessions || []) {
    if (session.agent_key.startsWith("review-standards") && session.report) reports.standards = safeJson(session.report);
    if (session.agent_key.startsWith("review-spec") && session.report) reports.spec = safeJson(session.report);
  }
  return reports;
}
function safeJson(value) { try { return JSON.parse(value); } catch { return null; } }
function reportAxis(title, report, session) {
  const verdict = report?.verdict || "UNKNOWN";
  return `<div class="axis"><div class="axistop"><div class="axisname">${title}</div><span class="verdict ${verdict === "PASS" ? "pass" : "fail"}">${verdict === "PASS" ? "PASS" : "CHANGES"}</span></div><div class="axisdesc">${escapeHtml(report?.summary || "没有结构化报告")}</div><div class="axismeta"><span>Agent: ${title.replace(" Review", "")}</span><span>耗时 ${session ? formatDuration(session.created_at, session.updated_at) : "—"}</span><span>Token 未记录</span></div></div>`;
}
function reviewFindings(reports) {
  return ["standards", "spec"].flatMap((axis) => (reports[axis]?.findings || []).map((finding) => ({ ...finding, axis })));
}
function findingsTable(findings) {
  const rows = state.onlyOpenFindings ? findings.filter((finding) => ["P0", "P1", "P2", "Advisory"].includes(finding.severity)) : findings;
  return `<div class="findtable"><div class="fhead"><span>级别</span><span>问题</span><span>位置</span><span>责任方</span><span>状态</span></div>${rows.map((finding) => `<div class="frow"><span class="sev ${finding.severity === "P0" || finding.severity === "P1" ? "p1" : finding.severity === "P2" ? "p2" : "note"}">${escapeHtml(finding.severity)}</span><div class="ftitle">${escapeHtml(finding.title)}</div><div class="floc">${escapeHtml(finding.location)}</div><div class="owner"><i class="miniavatar">DR</i>Driver</div><span class="state ${["P0", "P1"].includes(finding.severity) ? "open" : "fixed"}">${["P0", "P1"].includes(finding.severity) ? "待处理" : "仅展示"}</span></div>`).join("") || '<div class="empty">没有审查发现</div>'}</div>`;
}
function reviewTabBody(run, detail, findings) {
  if (state.tabs.review === "findings") return findingsTable(findings);
  if (state.tabs.review === "tests") return `<div class="logs" style="height:269px">${logLines(detail.logs.filter((line) => /测试|验证|PASS|退出码/.test(line)))}</div>`;
  if (state.tabs.review === "diff") return '<pre class="diff-view" id="review-diff">加载 Diff…</pre>';
  return `<div class="activity">${eventRows(detail.events, 10)}</div>`;
}
function reviewInspector(run, detail, reports, findings) {
  const blocking = findings.filter((finding) => ["P0", "P1"].includes(finding.severity));
  const advisory = findings.filter((finding) => ["P2", "Advisory"].includes(finding.severity));
  const fresh = run.status === "awaiting_approval";
  return `<div class="ititle">人工决策 <span>${runCode(run).replace("RUN", "REVIEW")}</span></div><div class="section"><div class="scap">门禁检查</div><div class="check"><i>✓</i><div><strong>验证矩阵${fresh ? "通过" : "需重跑"}</strong><br><span style="color:#8d96a7">确定性验证结果已保存</span></div></div><div class="check"><i>${reports.standards?.verdict === "PASS" ? "✓" : "!"}</i><div><strong>Standards ${escapeHtml(reports.standards?.verdict || "UNKNOWN")}</strong><br><span style="color:#8d96a7">${escapeHtml(reports.standards?.summary || "无报告")}</span></div></div><div class="check ${reports.spec?.verdict === "PASS" ? "" : "wait"}"><i>${reports.spec?.verdict === "PASS" ? "✓" : "!"}</i><div><strong>Spec ${escapeHtml(reports.spec?.verdict || "UNKNOWN")}</strong><br><span style="color:#8d96a7">${advisory.length} 个 P2/Advisory</span></div></div><div class="check ${fresh ? "" : "wait"}"><i>${fresh ? "✓" : "!"}</i><div><strong>审查${fresh ? "仍然有效" : "已失效"}</strong><br><span style="color:#8d96a7">${fresh ? "审查后代码未发生变化" : "必须重新验证和审查"}</span></div></div></div><div class="section"><div class="scap">变更规模</div><div class="diffbox"><div class="diffstat"><strong>${detail.stats.files}</strong><span>文件</span></div><div class="diffstat"><strong style="color:var(--green)">+${detail.stats.additions}</strong><span>新增</span></div><div class="diffstat"><strong style="color:var(--red)">−${detail.stats.deletions}</strong><span>删除</span></div></div></div><div class="approve"><h3>建议：${blocking.length ? "请求返工" : advisory.length ? "人工判断 P2/Advisory" : "批准提交"}</h3><p>${blocking.length ? "仍有 P0/P1 阻断项，不能批准提交。" : advisory.length ? "P2 和 Advisory 不触发自动返工，可人工接受风险或请求小范围返工。" : "门禁均已通过，可以创建 Issue 分支提交。"}</p></div><button class="widebtn" id="request-repair">请求返工并重新审查</button><button class="widebtn secondary ${fresh && !blocking.length ? "" : "disabled"}" id="approve-risk">接受风险并批准提交</button><button class="widebtn danger" id="reject-run">拒绝本次变更</button><div class="notice amber"><strong>批准后的动作</strong><br>系统将创建提交，但不会自动清理 Worktree、容器或数据库。</div><div class="footer-note">CrewOps · ${new Date().toISOString().slice(0, 10)}</div>`;
}

async function renderReviewPage(version) {
  const rows = filteredSidebarRows(sidebarConfig());
  const run = rows.find((item) => String(item.id) === state.selected.review) || rows[0];
  if (!run) { $("#page-main").innerHTML = '<div class="crumb">审查队列</div><div class="header"><div class="hmain"><div class="eyebrow"><span class="tag">REVIEW QUEUE</span></div><h1>审查队列</h1><div class="subtitle">当前没有等待人工决策或历史审查记录。</div></div></div>'; $("#page-inspector").innerHTML = '<div class="ititle">人工决策 <span>空</span></div><div class="notice amber">验证与双轴审查完成后，门禁、Diff 和批准操作会显示在这里。</div>'; return; }
  state.selected.review = String(run.id);
  const detail = await loadRun(run.id);
  if (version !== state.renderVersion || state.page !== "review") return;
  const reports = parseReports(detail);
  const findings = reviewFindings(reports);
  const standardsSession = detail.sessions.filter((item) => item.agent_key.startsWith("review-standards")).at(-1);
  const specSession = detail.sessions.filter((item) => item.agent_key.startsWith("review-spec")).at(-1);
  $("#page-main").innerHTML = `<div class="crumb">审查队列 / ${runCode(run).replace("RUN", "REVIEW")}</div><div class="header"><div class="hmain"><div class="eyebrow"><span class="tag">ISSUE #${run.issue_number}</span><span class="tag amber">${escapeHtml(statusLabel[run.status] || run.status)}</span></div><h1>${escapeHtml(run.issue_title)}</h1><div class="subtitle">${runCode(run)} · ${detail.stats.files} files changed · 最后验证 ${String(run.updated_at).slice(0, 19)}</div></div><div class="actions"><button class="btn" id="show-diff">查看 Diff</button><button class="btn" id="open-worktree">打开 Worktree</button></div></div><div class="cards queuecards">${statCard("开放问题", `${findings.length} 项`, "!")}${statCard("验证矩阵", run.status === "awaiting_approval" ? "已通过" : "需重跑", "✓")}${statCard("审查新鲜度", run.status === "awaiting_approval" ? "有效" : "已失效", "◷")}</div><div class="reviewhero">${reportAxis("Standards Review", reports.standards, standardsSession)}${reportAxis("Specification Review", reports.spec, specSession)}</div><section class="panel findings"><div class="tabs"><div class="tab ${state.tabs.review === "findings" ? "active" : ""}" data-review-tab="findings">审查发现 ${findings.length}</div><div class="tab ${state.tabs.review === "tests" ? "active" : ""}" data-review-tab="tests">测试报告</div><div class="tab ${state.tabs.review === "diff" ? "active" : ""}" data-review-tab="diff">代码 Diff</div><div class="tab ${state.tabs.review === "conclusion" ? "active" : ""}" data-review-tab="conclusion">Agent 结论</div><div class="plink" id="only-open">${state.onlyOpenFindings ? "显示全部" : "仅看开放项"}</div></div><div id="review-body">${reviewTabBody(run, detail, findings)}</div></section>`;
  $("#page-inspector").innerHTML = reviewInspector(run, detail, reports, findings);
  const switchTab = async (tab) => { state.tabs.review = tab; await renderPage(); };
  $$('[data-review-tab]').forEach((tab) => tab.onclick = () => switchTab(tab.dataset.reviewTab));
  $("#show-diff").onclick = () => switchTab("diff");
  $("#open-worktree").onclick = async () => { try { await post(`/api/runs/${run.id}/open`); notify("已在 Finder 打开 Worktree"); } catch (error) { notify(error.message, true); } };
  $("#only-open").onclick = () => { state.onlyOpenFindings = !state.onlyOpenFindings; renderPage(); };
  $("#request-repair").onclick = () => repairModal(run);
  $("#approve-risk").onclick = async () => { if (run.status !== "awaiting_approval") return; try { const result = await post(`/api/runs/${run.id}/approve`); notify(`已提交 ${result.sha}`); await refresh(); } catch (error) { notify(error.message, true); } };
  $("#reject-run").onclick = () => confirmModal("拒绝本次变更", "只会标记运行已拒绝，Worktree 和代码保持不变。", async () => { await post(`/api/runs/${run.id}/reject`); closeModal(); notify("已拒绝本次变更"); await refresh(); });
  if (state.tabs.review === "diff") { const diff = await request(`/api/runs/${run.id}/diff`); $("#review-diff").textContent = diff.diff || "无差异"; }
}

function repairModal(run) {
  showModal("请求返工并重新审查", `<div class="field full"><label>回传给原 Driver 的人工意见</label><textarea id="repair-prompt">请根据当前人工审查意见做必要的小范围修复；保持现有实现方向，不要扩大范围。完成后重新执行确定性验证和双轴审查。</textarea></div><div class="notice amber">原审查快照会立即失效；新运行继续使用同一个 Driver threadId。</div><div class="modal-actions"><button class="secondary-btn" id="cancel-repair">取消</button><button class="primary-btn" id="confirm-repair">请求返工</button></div>`, () => {
    $("#cancel-repair").onclick = closeModal;
    $("#confirm-repair").onclick = async () => { try { const result = await post(`/api/runs/${run.id}/repair`, { prompt: $("#repair-prompt").value.trim() }); closeModal(); notify(`返工运行 ${result.runId.slice(0, 8)} 已启动`); state.page = "run"; await refresh(); } catch (error) { notify(error.message, true); } };
  });
}

function confirmModal(title, message, confirm) {
  showModal(title, `<div class="task-box">${escapeHtml(message)}</div><div class="modal-actions"><button class="secondary-btn" id="confirm-cancel">取消</button><button class="danger-btn" id="confirm-ok">确认</button></div>`, () => {
    $("#confirm-cancel").onclick = closeModal;
    $("#confirm-ok").onclick = async () => { try { await confirm(); } catch (error) { notify(error.message, true); } };
  });
}

function inferLock(issue) {
  const text = `${issue.title}\n${issue.body || ""}`.toLowerCase();
  const rules = [["user", ["用户", "登录", "认证", "头像"]], ["article", ["文章", "正文", "图片", "草稿"]], ["comment", ["评论", "回复", "点赞"]], ["search", ["搜索", "meili"]], ["notification", ["通知", "kafka"]], ["grpc", ["grpc", "proto", "rpc"]]];
  return rules.find(([, words]) => words.some((word) => text.includes(word)))?.[0] || "shared";
}

function newRunModal() {
  const issues = state.data.issues.filter((issue) => Number(issue.number) !== 1);
  const navigationVersion = state.navigationVersion;
  showModal("启动新工单", `<div class="form-grid"><div class="field full"><label>Issue</label><select id="new-issue">${issues.map((issue) => `<option value="${issue.number}">#${issue.number} · ${escapeHtml(issue.title)}</option>`).join("")}</select></div><div class="field"><label>测试命令</label><input id="new-test" value="cd backend && go test ./..."></div><div class="field"><label>资源锁</label><select id="new-lock">${["user", "article", "comment", "search", "notification", "grpc", "integration", "shared", "none"].map((value) => `<option value="${value}">${value}</option>`).join("")}</select></div><div class="field"><label>自动返工轮次</label><select id="new-cycles"><option>0</option><option>1</option><option selected>2</option></select></div><div class="field"><label>Worktree</label><div class="task-box" id="worktree-status"></div></div><div class="field full"><label>Driver 追加指令</label><textarea id="new-prompt" placeholder="可选"></textarea></div><label class="checkline"><input type="checkbox" id="new-tdd" checked>启用严格 TDD 实现提示</label><label class="checkline"><input type="checkbox" id="new-approve">允许 Codex 自动批准 Worktree 命令</label><label class="checkline"><input type="checkbox" id="new-cost">确认本次运行会消耗模型额度</label></div><div class="modal-actions"><button class="secondary-btn" id="create-worktree">创建 Worktree</button><button class="primary-btn" id="start-run">启动唯一 Driver Thread</button></div>`, () => {
    const issueSelect = $("#new-issue");
    const update = () => {
      const issue = issues.find((item) => Number(item.number) === Number(issueSelect.value));
      const previousIssue = $("#new-prompt").dataset.issueNumber;
      if (previousIssue && previousIssue !== String(issue.number)) {
        $("#new-prompt").value = "";
      }
      $("#new-prompt").dataset.issueNumber = String(issue.number);
      $("#new-lock").value = inferLock(issue);
      const tree = state.data.worktrees[issue.number];
      $("#worktree-status").textContent = tree ? `${tree.path.split("/").pop()} · ${tree.branch}` : "尚未创建";
    };
    $("#new-prompt").value = "";
    issueSelect.onchange = update; update();
    $("#create-worktree").onclick = async () => { try { await post("/api/worktrees", { issueNumber: Number(issueSelect.value) }); notify("Worktree 已创建"); state.data = await request("/api/bootstrap"); update(); } catch (error) { notify(error.message, true); } };
    $("#start-run").onclick = async () => {
      if (!$("#new-approve").checked || !$("#new-cost").checked) return notify("请确认命令批准和模型额度", true);
      try {
        const result = await post("/api/runs", {
          issueNumber: Number(issueSelect.value), testCommand: $("#new-test").value.trim(),
          resourceLock: $("#new-lock").value === "none" ? "" : $("#new-lock").value,
          maxReviewCycles: Number($("#new-cycles").value), prompt: $("#new-prompt").value.trim(),
          autoApprove: true, tdd: $("#new-tdd").checked,
        });
        closeModal(); notify(`运行 ${result.runId.slice(0, 8)} 已启动`); if (state.navigationVersion === navigationVersion) state.page = "run"; await refresh();
      } catch (error) { notify(error.message, true); }
    };
  });
}

function settingsModal() {
  showModal("控制台设置", `<div class="field"><label>最大并发流水线（1–5）</label><input type="number" id="settings-concurrency" min="1" max="5" value="${state.data.maxConcurrency}"></div><div class="notice amber">并发只发生在不同 Issue 的独立 Worktree 之间；同一 Worktree 和资源锁仍不会并行写入。</div><div class="modal-actions"><button class="secondary-btn" id="settings-cancel">取消</button><button class="primary-btn" id="settings-save">保存</button></div>`, () => {
    $("#settings-cancel").onclick = closeModal;
    $("#settings-save").onclick = async () => { try { await post("/api/settings", { maxConcurrency: Number($("#settings-concurrency").value) }); closeModal(); notify("设置已保存"); await refresh(); } catch (error) { notify(error.message, true); } };
  });
}

function searchModal() {
  const all = [
    ...state.data.runs.map((run) => ({ type: "history", id: run.id, label: `${runCode(run)} · #${run.issue_number} ${run.issue_title}` })),
    ...state.data.sessions.map((session) => ({ type: "sessions", id: session.id, label: `Thread ${String(session.thread_id).slice(0, 12)} · #${session.issue_number} ${session.issue_title}` })),
    ...state.data.issues.map((issue) => ({ type: "issue", id: issue.number, label: `Issue #${issue.number} · ${issue.title}` })),
  ];
  showModal("全局搜索", `<div class="field"><input id="global-search-input" placeholder="搜索 Issue、运行、分支或 Thread"></div><div id="global-search-results" class="activity"></div>`, () => {
    const input = $("#global-search-input");
    const update = () => {
      const query = input.value.trim().toLowerCase();
      const rows = all.filter((item) => !query || item.label.toLowerCase().includes(query)).slice(0, 12);
      $("#global-search-results").innerHTML = rows.map((item) => `<div class="event" data-search-type="${item.type}" data-search-id="${item.id}"><div class="event-icon tool">⌕</div><div><div class="event-title">${escapeHtml(item.label)}</div></div></div>`).join("");
      $$('[data-search-type]').forEach((row) => row.onclick = () => {
        if (row.dataset.searchType === "issue") { closeModal(); issueModal(row.dataset.searchId); return; }
        state.page = row.dataset.searchType; state.selected[state.page] = row.dataset.searchId; closeModal(); render();
      });
    };
    input.oninput = update; update(); input.focus();
  });
}

async function renderPage(version = ++state.renderVersion) {
  if (version !== state.renderVersion) return;
  try {
    if (state.page === "run") await renderRunPage(version);
    else if (state.page === "history") await renderHistoryPage(version);
    else if (state.page === "sessions") await renderSessionsPage(version);
    else await renderReviewPage(version);
  } catch (error) {
    $("#page-main").innerHTML = `<div class="notice amber"><strong>页面加载失败</strong><br>${escapeHtml(error.message)}</div>`;
    notify(error.message, true);
  }
}

async function render() {
  const version = ++state.renderVersion;
  renderTopbar(); renderNavigation(); renderSidebar(); await renderPage(version);
}

async function refresh() {
  state.data = await request("/api/bootstrap");
  await render();
}

$("#sidebar-search").oninput = (event) => { state.search = event.target.value; renderSidebar(); renderPage(); };
$("#global-search").onclick = searchModal;
$("#settings-button").onclick = settingsModal;
$("#project-button").onclick = settingsModal;
$("#new-run-button").onclick = newRunModal;

refresh().catch((error) => notify(error.message, true));
window.setInterval(() => { if (state.page === "run" && state.data?.active.length) refresh(); }, 3000);
