# Copilot CLI工具映射

技能使用Claude Code工具名称。当您在技能中遇到这些时，请使用您的平台等效工具：

| 技能引用 | Copilot CLI等效 |
|-----------------|----------------------|
| `Read` (文件读取) | `view` |
| `Write` (文件创建) | `create` |
| `Edit` (文件编辑) | `edit` |
| `Bash` (运行命令) | `bash` |
| `Grep` (搜索文件内容) | `grep` |
| `Glob` (按名称搜索文件) | `glob` |
| `Skill`工具 (调用技能) | `skill` |
| `WebFetch` | `web_fetch` |
| `Task`工具 (调度子代理) | `task` (请参阅[代理类型](#agent-types)) |
| 多个`Task`调用 (并行) | 多个`task`调用 |
| 任务状态/输出 | `read_agent`, `list_agents` |
| `TodoWrite` (任务跟踪) | `sql`与内置`todos`表 |
| `WebSearch` | 无等效项 — 使用搜索引擎URL的`web_fetch` |
| `EnterPlanMode` / `ExitPlanMode` | 无等效项 — 保持在主会话中 |

## 代理类型

Copilot CLI的`task`工具接受`agent_type`参数：

| Claude Code代理 | Copilot CLI等效 |
|-------------------|----------------------|
| `general-purpose` | `"general-purpose"` |
| `Explore` | `"explore"` |
| 命名插件代理（例如`superpowers:code-reviewer`） | 从已安装的插件自动发现 |

## 异步shell会话

Copilot CLI支持持久异步shell会话，没有直接的Claude Code等效项：

| 工具 | 用途 |
|------|---------|
| `bash` with `async: true` | 在后台启动长时间运行的命令 |
| `write_bash` | 向运行中的异步会话发送输入 |
| `read_bash` | 从异步会话读取输出 |
| `stop_bash` | 终止异步会话 |
| `list_bash` | 列出所有活动的shell会话 |

## 额外的Copilot CLI工具

| 工具 | 用途 |
|------|---------|
| `store_memory` | 持久化关于代码库的事实以供未来会话使用 |
| `report_intent` | 使用当前意图更新UI状态栏 |
| `sql` | 查询会话的SQLite数据库（待办事项、元数据） |
| `fetch_copilot_cli_documentation` | 查找Copilot CLI文档 |
| GitHub MCP工具 (`github-mcp-server-*`) | 原生GitHub API访问（问题、PR、代码搜索） |