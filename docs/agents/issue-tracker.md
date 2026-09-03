# 问题跟踪器：GitHub

本仓库的问题与规格说明统一存放在 GitHub Issues 中。所有操作均使用 `gh` CLI。

## 操作约定

- **创建问题**：`gh issue create --title "..." --body "..."`。正文包含多行内容时使用 heredoc。
- **读取问题**：运行 `gh issue view <编号> --comments`，使用 `jq` 筛选评论，并同时获取标签。
- **列出问题**：运行 `gh issue list --state open --json number,title,body,labels,comments --jq '[.[] | {number, title, body, labels: [.labels[].name], comments: [.comments[].body]}]'`，并根据需要添加 `--label` 和 `--state` 筛选条件。
- **评论问题**：`gh issue comment <编号> --body "..."`
- **添加或移除标签**：`gh issue edit <编号> --add-label "..."` 或 `--remove-label "..."`
- **关闭问题**：`gh issue close <编号> --comment "..."`

仓库信息从 `git remote -v` 推断；在克隆的仓库目录内运行时，`gh` 会自动完成这一过程。

## 是否将拉取请求作为分诊入口

**不将 PR 作为请求入口。** 如果本仓库需要把外部 PR 当作功能请求处理，可将此项改为“是”；`/triage` 会读取该设置。

设置为“是”后，PR 将使用与问题相同的标签和状态，并通过对应的 `gh pr` 命令操作：

- **读取 PR**：使用 `gh pr view <编号> --comments` 查看内容和评论，使用 `gh pr diff <编号>` 查看差异。
- **列出待分诊的外部 PR**：运行 `gh pr list --state open --json number,title,body,labels,author,authorAssociation,comments`，仅保留 `authorAssociation` 为 `CONTRIBUTOR`、`FIRST_TIME_CONTRIBUTOR` 或 `NONE` 的项目，排除 `OWNER`、`MEMBER` 和 `COLLABORATOR`。
- **评论、添加标签或关闭**：使用 `gh pr comment`、`gh pr edit --add-label`、`gh pr edit --remove-label` 和 `gh pr close`。

GitHub 的问题和 PR 共用同一编号空间，因此单独出现的 `#42` 可能指向其中任意一种。先运行 `gh pr view 42`，失败时再运行 `gh issue view 42`。

## 当技能要求“发布到问题跟踪器”时

创建一个 GitHub Issue。

## 当技能要求“获取相关工单”时

运行 `gh issue view <编号> --comments`。

## Wayfinder 导航操作

供 `/wayfinder` 使用。“地图”是一个主问题，其下关联多个作为工单的子问题。

- **地图**：带有 `wayfinder:map` 标签的单个问题，用于保存“笔记”“当前决策”和“待探索区域”等内容。使用 `gh issue create --label wayfinder:map` 创建。
- **子工单**：作为 GitHub 子问题关联到地图的问题，通过子问题 API 和 `gh api` 操作。如果仓库未启用子问题功能，则在地图正文中添加任务列表，并在子工单正文顶部写入 `Part of #<地图编号>`。标签使用 `wayfinder:<类型>`，类型包括 `research`、`prototype`、`grilling` 和 `task`。工单被领取后，将其分配给负责执行的开发者。
- **阻塞关系**：优先使用 GitHub 原生的问题依赖关系，作为规范且在界面中可见的表示。添加依赖边时运行 `gh api --method POST repos/<所有者>/<仓库>/issues/<子工单>/dependencies/blocked_by -F issue_id=<阻塞项数据库ID>`。其中 `<阻塞项数据库ID>` 是阻塞问题的数字数据库 ID，可通过 `gh api repos/<所有者>/<仓库>/issues/<编号> --jq .id` 获取；它不是 `#编号` 或 `node_id`。GitHub 通过 `issue_dependencies_summary.blocked_by` 返回仍处于开放状态的阻塞项数量。如果依赖功能不可用，则在子工单正文顶部使用 `Blocked by: #<编号>, #<编号>`。所有阻塞项关闭后，该工单才视为解除阻塞。
- **前沿查询**：列出地图下仍开放的子问题，可使用 `gh issue list --state open` 并限定到地图的子问题或任务列表。排除存在开放阻塞项或已有受理人的工单，然后按地图中的排列顺序选择第一个。
- **领取**：运行 `gh issue edit <编号> --add-assignee @me`。这是会话中的第一次写操作。
- **解决**：运行 `gh issue comment <编号> --body "<答案>"`，随后运行 `gh issue close <编号>`，最后在地图的“当前决策”中追加上下文指针及链接。
