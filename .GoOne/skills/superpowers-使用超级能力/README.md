# 使用超级能力技能

## 概述

此技能用于在开始任何对话时确定如何查找和使用技能，确保在任何响应之前调用适当的技能。

## 技能名称

**superpowers-使用超级能力**

## 核心功能

- 技能发现和选择
- 技能调用流程
- 指令优先级管理
- 平台适配

## 文件结构

```
superpowers-使用超级能力/
├── SKILL.md                 # 主技能文档
└── references/              # 参考文档目录
    ├── codex-tools.md       # Codex工具映射
    ├── copilot-tools.md     # Copilot CLI工具映射
    └── gemini-tools.md      # Gemini CLI工具映射
```

## 文件说明

| 文件 | 说明 |
|------|------|
| `SKILL.md` | 核心技能文档，包含技能使用规则和工作流程 |
| `references/codex-tools.md` | Codex平台的工具映射 |
| `references/copilot-tools.md` | Copilot CLI的工具映射 |
| `references/gemini-tools.md` | Gemini CLI的工具映射 |

## 使用场景

- 开始任何对话时
- 需要查找和使用技能时
- 需要确定技能优先级时

## 相关技能

- 所有其他superpowers技能