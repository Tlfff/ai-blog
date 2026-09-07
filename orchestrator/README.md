# 本地博客重构控制台

这是一个本地运行的 CrewOps Web 控制台，用于查看 GitHub Issues、管理 Git worktree，并通过 CrewAI Flow 调度本机 Codex App Server。

## 启动

```bash
cd /Users/staff/code/go/ai-blog/orchestrator
UV_CACHE_DIR=.uv-cache uv run --frozen orchestrator
```

浏览器打开 `http://127.0.0.1:8765`。控制台直接基于 CrewOps 原型 HTML/CSS 实现，不依赖 Streamlit。

首次运行或生成器更新后安装隔离工具链：

```bash
./setup-tools.sh
```

生成器安装到 `orchestrator/.tools/bin`，运行时优先于全局 `$GOPATH/bin`；锁定版本记录在 `tools.lock`。

## UI 验收

- `UI_CONTRACT.md` 记录原型元素、真实数据来源和点击行为。
- `web/e2e_contract.cjs` 使用固定的运行中、已完成、会话和待审查数据逐项点击原型交互，并输出四页截图到 `/tmp/crewops-*-contract.png`。
- Token、缓存命中率和上下文窗口在 App Server 未提供可靠数据时显示“未记录”，不会生成演示数值。

## 当前功能

- 操作台 UI 直接采用 CrewOps 参考原型的信息架构：顶部项目状态栏、左侧工作区导航与最近运行、中部页面内容、右侧节点/会话/决策检查器。
- 提供运行中心、运行历史、Agent 会话和审查队列四个独立页面；审查队列集中展示双轴报告、测试日志、代码 Diff、审查指纹和人工提交决策。
- 从当前仓库的 `origin` 自动识别 GitHub 仓库并读取开放 Issues。
- 显示 Issue 标签、受理人、正文和 `Blocked by` 依赖。
- 查看现有 worktree。
- 在仓库已有基线提交后，为 Issue 创建 `codex/issue-<number>` 分支及独立目录。
- 自动装载当前 Issue、父规格、根/后端 `AGENTS.md`、功能文档、领域文档和相关 Matt skills。
- 按改动自动执行 `make api/config/gen`、格式检查、工单测试、`go vet`、必要的竞态测试和差异检查。
- 每个 Issue 只保留一个可恢复的 Codex Driver Thread；规划、实现、验证失败修复和审查返工都是该 thread 的连续 turn。
- 状态机固定为 `PRECHECK → DRIVER_PLAN → DRIVER_IMPLEMENT → VALIDATE → PARALLEL_REVIEW`；P0/P1 失败后进入 `DRIVER_REPAIR → TARGETED_VALIDATE → PARALLEL_REVIEW`，通过后等待人工批准。
- 持久化流水线日志、阶段状态、Agent 状态和 Codex threadId/turnId 到 `data/orchestrator.sqlite3`。
- “调用/继续”只展示并恢复可写 Driver Thread；Standards/Spec Sidecar 不能被手动用于编码。
- 会话“调用/继续”会创建独立的可追踪运行记录，并在“运行中心”实时展示，完成后进入运行历史。
- 支持最多 5 条流水线并行执行，默认 2 条，可在顶部设置面板手动调整。
- 同一 worktree 和相同资源锁不会并行；仍有开放依赖的 Issue 不能启动。
- 每条运行分配独立 Compose project、端口槽和环境变量；运行环境只在手动确认时清理。
- 页面按“运行中心 / 运行历史 / Agent 会话 / 审查队列”四个工作区组织，左侧记录可直接切换中央内容和右侧检查器。
- 运行中心组织所有活动任务；等待批准、审查失效、已批准和已拒绝记录进入审查队列统一处理。
- App Server 返回的实际模型、Provider 和推理强度会保存到 SQLite，并显示在活动任务和当前 Agent 卡片中；旧记录可能显示“未记录”。
- 页面显示带 Agent 节点的进度轨道、当前 Agent、当前命令和结构化双轴审查报告。
- Agent 动态和会话消息展示 Codex 提供的推理摘要、计划和动作，不展示隐藏思维链。
- Standards/Spec 是短生命周期只读 Sidecar，每轮使用独立 thread，仅返回结构化结果；只有 P0/P1 会完整回传原 Driver，最多自动返工 2 轮。
- 双轴审查使用精确 ReviewPack：Standards 只逐项检查手写代码/测试/配置源，生成文件只检查来源、可重复生成和无关漂移；Spec 只读取当前验收标准和匹配的规格章节。每轴最多 5 项，只有 P0/P1 阻断，P2 和代码异味作为 Advisory。
- 变更清单同时包含已跟踪、已修改和未跟踪文件；双轴通过时保存 HEAD、完整文件清单、diff hash 和组合指纹，批准前任一项变化都会取消提交并要求重新验证。
- 创建 worktree 前先更新 `origin`；已有 `codex/issue-N` 分支时恢复使用，不再固定重复创建分支。
- 启动前预检后端生成工具，缺少工具时在页面列出安装命令并禁止消耗模型额度。
- PRECHECK 校验锁定的 Proto/gRPC 生成器版本；验证只定向生成当前变更的 Proto 和相关 Wire 入口，并自动恢复与当前 Issue 无关的生成漂移。
- 定向生成完成后，控制器只对本轮允许的 `pb.go`、gRPC/HTTP 生成文件和 `wire_gen.go` 执行一次确定性 `gofmt -w`，再对手写变更执行只读 `gofmt -l`，避免把生成器格式差异回传给 Driver。
- 首次验证通过后，原 Driver 会在 Sidecar 审查前主动核对验收标准、原子性、Outbox/Consumer 闭环、Adapter 测试和强制注释，再执行一次确定性验证。
- 返工后的 Sidecar 只复核上一轮既有 P0/P1 及返工直接引入的回归，避免每轮重新开放整个问题空间。

## 执行说明

1. 选择具体实现 Issue，不要选择总规格 Issue #1。
2. 如果没有 worktree，先点击“创建独立 Worktree”。
3. 填写测试命令，默认是 `cd backend && go test ./...`。
4. 勾选自动批准和额度确认后启动流水线；默认并发 2，最大并发 5，同一 worktree 或资源锁不会并发写入。
5. 双轴审查通过后检查 diff，点击“批准提交”；提交完成后可以“提交分支并创建 PR”，再由人工点击“合并 PR 到远程 master”。

## 并行与 Docker

每条流水线都会获得唯一的 `COMPOSE_PROJECT_NAME`，并注入 `AI_BLOG_*_PORT` 端口变量。当前仓库暂时没有 Compose 文件，因此控制台不会自动创建数据库容器；后续 Compose 文件应使用这些变量映射宿主机端口。PR 合并不会自动删除 Worktree、容器和数据库，运行历史中的“清理 Compose 环境”需要手动点击，勾选删除 volumes 后测试数据才会被删除。

## App Server 和会话

控制器会按需启动 `codex app-server --listen stdio://`，通过 JSON-RPC 调用 `thread/start`、`thread/resume`、`turn/start`、`turn/interrupt` 和 `thread/delete`。App Server 进程可以在一次流水线后退出，thread 的标识和控制器状态由 SQLite 保存；下次调用时重新启动 App Server 并恢复 thread。

当前使用 Codex 配置文件中的模型和 provider，控制器不会覆盖 `--model`。如果通过 CCSwitch 切换模型，切换后新启动的 turn 会读取新的配置。
