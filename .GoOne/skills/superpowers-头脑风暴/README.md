# 头脑风暴技能

## 概述

此技能用于在任何创造性工作之前探索用户意图、需求和设计。通过自然协作对话，帮助将想法转化为完整的设计和规范。

## 技能名称

**superpowers-头脑风暴**

## 核心功能

- 了解项目背景和用户需求
- 通过提问完善想法
- 展示设计方案并获得用户批准
- 创建可视化原型和规格文档

## 文件结构

```
superpowers-头脑风暴/
├── SKILL.md                 # 主技能文档
├── spec-document-reviewer-prompt.md  # 规范文档审查者提示模板
├── visual-companion.md      # 可视化辅助指南
└── scripts/                 # 可视化工具脚本目录
    ├── frame-template.html  # 框架模板
    ├── helper.js            # 客户端辅助脚本
    ├── server.cjs           # 服务器端脚本
    ├── start-server.sh      # 启动脚本
    └── stop-server.sh       # 停止脚本
```

## 文件说明

| 文件 | 说明 |
|------|------|
| `SKILL.md` | 核心技能文档，包含工作流程和使用说明 |
| `spec-document-reviewer-prompt.md` | 用于调度规范文档审查者子代理的提示模板 |
| `visual-companion.md` | 基于浏览器的可视化头脑风暴辅助工具指南 |
| `scripts/` | 包含可视化服务器相关脚本 |

## 使用场景

- 创建新功能前的需求分析
- 构建组件前的设计探索
- 添加功能前的方案讨论
- 修改行为前的影响评估

## 相关技能

- superpowers-编写计划 - 编写实施计划
- superpowers-执行计划 - 执行计划