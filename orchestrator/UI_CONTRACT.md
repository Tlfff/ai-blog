# CrewOps UI 合同

本文件把参考原型中的可见元素绑定到真实数据与操作。实现和端到端测试以此为准。

## 全局框架

| 元素 | 数据/操作 |
| --- | --- |
| 项目选择器 | 当前 Git `origin` 推断的仓库名；点击打开设置面板 |
| 服务状态 | API 可用状态、活动运行数和并发上限 |
| 搜索 | 过滤当前左侧运行、会话或审查记录 |
| 设置 | 修改并发上限 1–5 |
| 启动新工单 | 打开启动对话框；选择 Issue、测试命令、资源锁、返工轮次和 Driver 指令 |
| 左侧导航 | 运行中心、运行历史、Agent 会话、审查队列，均可切换 |
| 左侧记录 | 点击后切换中央内容和右侧检查器 |

## 运行中心

| 元素 | 数据/操作 |
| --- | --- |
| 查看 Issue | 在浏览器新标签打开 GitHub Issue |
| 停止运行 | 调用 `mark_interrupted`，不删除 Worktree 修改 |
| 四个指标 | 实际耗时；Token 无数据时显示“未记录”；工具调用数；完整变更文件数 |
| 执行流程 | PRECHECK、Driver 规划/实现、验证、并行审查、人工批准 |
| Agent 动态 | 当前节点的结构化事件流；可切换“仅当前 Agent” |
| 日志/代码变更/产物 | 实时日志、Git diff、生成文件/完整变更清单 |
| 节点检查器 | 当前节点、模型、推理强度、threadId、Worktree、分支、任务和节点统计 |

## 运行历史

| 元素 | 数据/操作 |
| --- | --- |
| 搜索与状态筛选 | 客户端过滤历史运行 |
| 导出报告 | 下载当前运行 JSON 快照 |
| 基于此运行重跑 | 使用原配置启动新运行，并恢复同 Issue Driver Thread |
| 执行回放 | 显示各阶段及实际会话/turn 耗时 |
| 摘要/日志/代码变更/产物 | 均绑定真实运行快照 |
| 打开 Agent 会话 | 跳转到当前运行的 Driver 会话 |
| 比较另一次运行 | 选择同 Issue 或任意历史运行并比较状态、耗时和变更规模 |
| 清理 Compose | 仅显式点击后执行，不删除 Worktree |
| 提交分支并创建 PR | 仅在人工批准提交后可用；先确认干净 Worktree、Issue 分支和批准 HEAD，再推送分支并创建目标为远程 `master` 的 PR |
| 打开 PR | 在 GitHub 打开已记录的 PR |
| 合并 PR 到远程 master | 重新读取 PR 状态、目标分支、head SHA 和 GitHub checks，通过后调用 `gh pr merge --merge`；不删除 Worktree、不关闭 Issue |

## Agent 会话

| 元素 | 数据/操作 |
| --- | --- |
| 会话搜索/筛选 | 过滤 Driver 会话；归档会话默认隐藏 |
| 消息与执行 | 只显示同一 Driver Thread 的事件，不混入 Standards/Spec Sidecar |
| 计划/工具调用/上下文 | 分别显示 Driver plan、action/log 和可用元数据；无 Token 数据时显示“未记录” |
| 发送调用 | 恢复原 Driver threadId 创建后续 turn |
| 停止调用 | 仅在关联 continuation 正运行时可用 |
| 新建关联运行 | 等价于发送 Driver continuation |
| 归档会话 | 隐藏会话但保留 threadId 与历史数据 |
| 删除会话 | 删除 Codex Thread 及对应本地会话记录 |

## 审查队列

| 元素 | 数据/操作 |
| --- | --- |
| 搜索与状态筛选 | 过滤待决策、返工中、已批准和已拒绝记录 |
| 查看 Diff | 切换到代码 Diff 标签 |
| 打开 Worktree | 使用 macOS Finder 打开 Worktree |
| Standards/Spec 卡片 | 展示最新只读 Sidecar 结构化报告 |
| 发现/测试/Diff/结论 | 展示真实 findings、验证日志、完整 diff 和 Agent 事件 |
| 仅看开放项 | 过滤 P0/P1/P2/Advisory 中尚未处理的发现 |
| 请求返工并重新审查 | 恢复原 Driver Thread；原审查快照立即失效 |
| 接受风险并批准提交 | 仅在审查快照有效且无 P0/P1 时提交 |
| 拒绝本次变更 | 仅标记运行已拒绝，保留 Worktree 和代码 |

## 明确不伪造的数据

- Token、缓存命中率、上下文窗口和推理 Token：App Server 未提供或未持久化时显示“未记录”。
- 合并状态：只有人工点击“合并 PR 到远程 master”才执行远程合并；不会本地直接 merge，不会自动删除 Worktree 或关闭 Issue。
- 不存在的历史 turn：旧数据只显示已保存的 session 信息，不推测 turn 数量。
