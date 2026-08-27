# 子代理驱动开发技能

## 概述

此技能用于将复杂任务分解为独立的子任务，每个子任务由专门的子代理处理，提高专注度、可审查性和执行质量。

## 技能名称

**superpowers-子代理驱动开发**

## 核心功能

- 分解复杂任务为独立子任务
- 为每个任务调度专门的子代理
- 在任务之间进行两阶段审查
- 确保规范合规和代码质量

## 文件结构

```
superpowers-子代理驱动开发/
├── SKILL.md                     # 主技能文档
├── code-quality-reviewer-prompt.md   # 代码质量审查者提示模板
├── implementer-prompt.md        # 实现者提示模板
└── spec-reviewer-prompt.md      # 规范合规审查者提示模板
```

## 文件说明

| 文件 | 说明 |
|------|------|
| `SKILL.md` | 核心技能文档，包含子代理调度工作流程 |
| `code-quality-reviewer-prompt.md` | 代码质量审查者子代理的提示模板 |
| `implementer-prompt.md` | 实现者子代理的提示模板 |
| `spec-reviewer-prompt.md` | 规范合规审查者子代理的提示模板 |

## 使用场景

- 执行复杂的多阶段任务
- 需要严格审查的任务
- 团队协作场景
- 涉及多个技术栈的任务

## 相关技能

- superpowers-编写计划 - 编写计划
- superpowers-执行计划 - 执行计划
- superpowers-请求代码审查 - 请求代码审查