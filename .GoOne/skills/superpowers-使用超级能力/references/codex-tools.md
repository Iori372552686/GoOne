# Codex工具映射

技能使用Claude Code工具名称。当您在技能中遇到这些时，请使用您的平台等效工具：

| 技能引用 | Codex等效 |
|-----------------|------------------|
| `Task`工具 (调度子代理) | `spawn_agent` (请参阅[命名代理调度](#named-agent-dispatch)) |
| 多个`Task`调用 (并行) | 多个`spawn_agent`调用 |
| 任务返回结果 | `wait` |
| 任务自动完成 | `close_agent`释放槽位 |
| `TodoWrite` (任务跟踪) | `update_plan` |
| `Skill`工具 (调用技能) | 技能原生加载 — 只需遵循说明 |
| `Read`, `Write`, `Edit` (文件) | 使用您的原生文件工具 |
| `Bash` (运行命令) | 使用您的原生shell工具 |

## 子代理调度需要多代理支持

添加到您的Codex配置（`~/.codex/config.toml`）：

```toml
[features]
multi_agent = true
```

这启用了`spawn_agent`、`wait`和`close_agent`，用于`dispatching-parallel-agents`和`subagent-driven-development`等技能。

## 命名代理调度

Claude Code技能引用命名代理类型，如`superpowers:code-reviewer`。
Codex没有命名代理注册表 — `spawn_agent`从内置角色（`default`、`explorer`、`worker`）创建通用代理。

当技能说要调度命名代理类型时：

1. 找到代理的提示文件（例如`agents/code-reviewer.md`或技能的本地提示模板，如`code-quality-reviewer-prompt.md`）
2. 读取提示内容
3. 填写任何模板占位符（`{BASE_SHA}`、`{WHAT_WAS_IMPLEMENTED}`等）
4. 使用填充内容作为`message`生成`worker`代理

| 技能指令 | Codex等效 |
|-------------------|------------------|
| `Task tool (superpowers:code-reviewer)` | `spawn_agent(agent_type="worker", message=...)`与`code-reviewer.md`内容 |
| `Task tool (general-purpose)`带内联提示 | `spawn_agent(message=...)`与相同提示 |

### 消息框架

`message`参数是用户级输入，不是系统提示。为最大指令遵守性进行结构化：

```
Your task is to perform the following. Follow the instructions below exactly.

<agent-instructions>
[来自代理的.md文件的填充提示内容]
</agent-instructions>

Execute this now. Output ONLY the structured response following the format
specified in the instructions above.
```

- 使用任务委托框架（"Your task is..."）而非角色框架（"You are..."）
- 将指令包装在XML标签中 — 模型将带标签的块视为权威
- 以显式执行指令结束，防止指令摘要

### 何时可以删除此解决方法

此方法弥补了Codex的插件系统尚未在`plugin.json`中支持`agents`字段的问题。当`RawPluginManifest`获得`agents`字段时，插件可以符号链接到`agents/`（镜像现有的`skills/`符号链接），技能可以直接调度命名代理类型。

## 环境检测

创建工作树或完成分支的技能应在继续之前使用只读git命令检测其环境：

```bash
GIT_DIR=$(cd "$(git rev-parse --git-dir)" 2>/dev/null && pwd -P)
GIT_COMMON=$(cd "$(git rev-parse --git-common-dir)" 2>/dev/null && pwd -P)
BRANCH=$(git branch --show-current)
```

- `GIT_DIR != GIT_COMMON` → 已在链接工作树中（跳过创建）
- `BRANCH`为空 → 分离HEAD（无法从沙箱分支/推送/PR）

请参阅`using-git-worktrees`步骤0和`finishing-a-development-branch`步骤1，了解每个技能如何使用这些信号。

## Codex App完成

当沙箱阻止分支/推送操作时（外部管理的工作树中的分离HEAD），代理提交所有工作并通知用户使用App的原生控件：

- **"Create branch"** — 命名分支，然后通过App UI提交/推送/PR
- **"Hand off to local"** — 将工作转移到用户的本地检出

代理仍然可以运行测试、暂存文件，并输出建议的分支名称、提交消息和PR描述供用户复制。