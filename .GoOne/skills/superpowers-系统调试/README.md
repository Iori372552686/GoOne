# 系统调试技能

## 概述

此技能用于系统地调试技术问题，确保在尝试修复之前找到根本原因，避免症状修复和随机调试。

## 技能名称

**superpowers-系统调试**

## 核心功能

- 根本原因调查
- 模式分析
- 假设测试
- 实施修复验证

## 文件结构

```
superpowers-系统调试/
├── SKILL.md                 # 主技能文档
├── CREATION-LOG.md          # 创建日志
├── condition-based-waiting.md    # 基于条件的等待技术
├── defense-in-depth.md       # 纵深防御验证
├── root-cause-tracing.md     # 根本原因追踪
├── test-academic.md          # 学术测试场景
├── test-pressure-1.md        # 压力测试1：紧急生产修复
├── test-pressure-2.md        # 压力测试2：沉没成本+疲惫
├── test-pressure-3.md        # 压力测试3：权威+社会压力
└── scripts/                  # 辅助脚本目录
    └── find-polluter.sh      # 查找污染者脚本
```

## 文件说明

| 文件 | 说明 |
|------|------|
| `SKILL.md` | 核心技能文档，包含四阶段调试流程 |
| `CREATION-LOG.md` | 技能创建日志和验证记录 |
| `condition-based-waiting.md` | 基于条件的等待技术指南 |
| `defense-in-depth.md` | 纵深防御验证方法 |
| `root-cause-tracing.md` | 根本原因追踪技术 |
| `test-academic.md` | 学术测试场景（无压力） |
| `test-pressure-*.md` | 压力测试场景（时间压力、沉没成本、权威压力） |
| `scripts/find-polluter.sh` | 查找测试污染者的辅助脚本 |

## 使用场景

- 测试失败时
- 生产环境中的bug
- 意外行为
- 性能问题
- 构建失败
- 集成问题

## 相关技能

- superpowers-测试驱动开发 - 测试驱动开发
- superpowers-完成前验证 - 完成前验证