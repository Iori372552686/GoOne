# 编写技能

## 概述

此技能用于创建新技能、编辑现有技能或在部署前验证技能工作，遵循测试驱动开发原则。

## 技能名称

**superpowers-编写技能**

## 核心功能

- 技能创建流程（TDD方法）
- 技能结构设计
- 技能测试和验证
- 技能文档编写最佳实践

## 文件结构

```
superpowers-编写技能/
├── SKILL.md                 # 主技能文档
├── anthropic-best-practices.md  # Anthropic官方最佳实践指南
├── graphviz-conventions.dot     # Graphviz样式约定
├── persuasion-principles.md    # 说服原则指南
├── testing-skills-with-subagents.md  # 使用子代理测试技能
├── render-graphs.js          # 图表渲染脚本
└── examples/                 # 示例目录
    └── CLAUDE_MD_TESTING.md  # CLAUDE.md测试示例
```

## 文件说明

| 文件 | 说明 |
|------|------|
| `SKILL.md` | 核心技能文档，包含技能创建流程和检查清单 |
| `anthropic-best-practices.md` | Anthropic官方技能创作最佳实践 |
| `graphviz-conventions.dot` | Graphviz图表样式约定 |
| `persuasion-principles.md` | 技能设计的说服原则指南 |
| `testing-skills-with-subagents.md` | 使用子代理测试技能的方法 |
| `render-graphs.js` | 渲染技能流程图的脚本 |
| `examples/CLAUDE_MD_TESTING.md` | CLAUDE.md技能文档测试示例 |

## 使用场景

- 创建新技能时
- 编辑现有技能时
- 在部署前验证技能工作时
- 需要编写有效的技能文档时

## 相关技能

- superpowers-测试驱动开发 - 测试驱动开发
- superpowers-完成前验证 - 完成前验证