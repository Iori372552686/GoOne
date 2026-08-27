---
name: "G1-项目配置"
description: "GoOne 项目集中配置管理。当任何 G1-* 技能需要获取项目配置信息（vault_path、prd_dir、analysis_output_dir 等），或在配置读取、路径解析、Obsidian 知识库操作前必须先调用本技能。"
---

# G1-项目配置

## 概述

本技能是 **GoOne 项目的集中配置管理中心**，所有 G1-* 技能通过调用本技能获取项目配置信息，而非直接读取配置文件。后续将扩展 Git 版本管理配置、部署目标设定等功能。

> **定位边界**：
> - ✅ 集中管理 `.GoOne/conf.json` 的读取和解析
> - ✅ 向其他技能提供 vault_path、prd_dir、analysis_output_dir 等配置字段
> - ✅ 定义 Obsidian 知识库路径规则、技术产出目录规则
> - ✅ 后续扩展：Git 仓库配置、部署目标等
> - ❌ 不替代各技能自身的业务逻辑和流程
> - ❌ 不修改 `.GoOne/conf.json` 文件内容（只读）

---

## 配置文件位置

所有项目配置统一存放在项目根目录下的 `.GoOne/conf.json`：

```
{project_root}/.GoOne/conf.json
```

配置格式为 JSON：

```json
{
  "vault_path": "../Knowledge",
  "prd_dir": "策划案",
  "analysis_output_dir": "技术目录/需求分析"
}
```

如果 `conf.json` 不存在，向用户询问 Obsidian Vault 路径后自动创建。

> **注意区分两类配置**：
> - `.GoOne/conf.json`：**AI 工作流配置**（Obsidian 知识库、策划/技术文档目录），供 G1-* 技能使用。
> - `etc/config/server_conf_*.yaml`：**服务运行时配置**（端口、DB、bus 等），由 `module/conf` 加载。两者互不干扰，本技能只管前者。

---

## 配置字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `vault_path` | string | 是 | Obsidian Vault 的相对/绝对路径（相对于项目根，或绝对路径）。知识库的根目录。 |
| `prd_dir` | string | 是 | 策划需求文档所在目录（相对 Vault 根目录），如 `策划案` |
| `analysis_output_dir` | string | 是 | 技术分析与需求分析输出目录（相对 Vault 根目录），如 `技术目录/需求分析` |

### 字段用途速查

| 字段 | 读取命令 | 写入命令 |
|------|---------|---------|
| `vault_path` | 用于定位 Obsidian Vault 根目录，配合 obsidian-cli 的 `vault=` 或直接拼路径 | - |
| `prd_dir` | `obsidian vault="{vault_path}" read path="{prd_dir}/{文档名称}"` | - |
| `analysis_output_dir` | `obsidian vault="{vault_path}" read path="{analysis_output_dir}/{需求名称}/..."` | `obsidian vault="{vault_path}" create name="{analysis_output_dir}/{需求名称}/..."` |

> 说明：`vault_path` 指向 Vault 在文件系统中的位置；obsidian-cli 命令里的 `vault=` 参数通常填 **Vault 名称**（即 `vault_path` 的最后一段目录名，如 `../Knowledge` → Vault 名 `Knowledge`）。若 obsidian-cli 用法有差异，以实际工具行为为准。

---

## 输出目录规则

所有技术类输出统一遵循以下目录规则：

```
{analysis_output_dir}/{需求名称}/
```

例如 `analysis_output_dir` = `技术目录/需求分析` 时，产出路径为 `技术目录/需求分析/{需求名称}/需求分析报告.md`。

技术设计类文档（非需求分析）可放在平级的 `技术目录/{需求名称}/` 下，按各技能约定。

---

## 如何调用本技能

其他 G1-* 技能在需要获取项目配置时，按以下方式引用：

### 第一步：调用本技能获取配置

在技能流程的"读取知识库配置"步骤中，先调用本技能：

> 调用 `G1-项目配置` 技能获取项目配置：
> - `vault_path` = Obsidian Vault 路径（定位知识库根目录）
> - `prd_dir` = 策划需求文档目录
> - `analysis_output_dir` = 技术分析输出目录
>
> 所有技术产出路径均为 `{analysis_output_dir}/{需求名称}/...`。

### 第二步：在 obsidian-cli 命令中使用配置变量

```bash
# 读取策划案文档
obsidian vault="{vault_path}" read path="{prd_dir}/{文档名称}"

# 读取技术文档
obsidian vault="{vault_path}" read path="{analysis_output_dir}/{需求名称}/{文档名称}"

# 创建技术文档
obsidian vault="{vault_path}" create name="{analysis_output_dir}/{需求名称}/{文档名称}" content="{内容}"
```

如果 `conf.json` 不存在，向用户询问 Obsidian Vault 路径后自动创建 `.GoOne/conf.json`。

---

## 扩展预留

### 版本管理配置（规划中）

GoOne 使用 **Git** 作为版本控制系统（当前分支策略见仓库 `AGENTS.md` 与 `CONTRIBUTING.md`）。未来可在 `conf.json` 扩展：

```json
{
  "vault_path": "../Knowledge",
  "prd_dir": "策划案",
  "analysis_output_dir": "技术目录/需求分析",
  "vcs": {
    "type": "git",
    "main_branch": "master",
    "dev_branch": "dev"
  }
}
```

### 部署目标配置（规划中）

```json
{
  "deploy": {
    "envs": ["dev1", "dev2", "dev_local"],
    "roles": ["connsvr", "mainsvr", "infosvr", "mysqlsvr", "roomcentersvr", "websvr"]
  }
}
```

扩展说明：

| 预留字段 | 说明 |
|---------|------|
| `vcs.type` | 版本控制系统类型（GoOne 固定为 `git`） |
| `deploy.envs` | 部署环境列表，对应 `deploy/playbook_dev/<env>.yml` |
| `deploy.roles` | 部署角色集合，对应 `deploy/roles/<svc>` |
