# 根本原因追踪

## 概述

Bug通常在调用堆栈深处显现（git init在错误目录，文件创建在错误位置，数据库用错误路径打开）。你的本能是在错误出现的地方修复，但那只是治疗症状。

**核心原则：** 向后追踪调用链，直到找到原始触发器，然后在源头修复。

## 何时使用

```dot
digraph when_to_use {
    "Bug appears deep in stack?" [shape=diamond];
    "Can trace backwards?" [shape=diamond];
    "Fix at symptom point" [shape=box];
    "Trace to original trigger" [shape=box];
    "BETTER: Also add defense-in-depth" [shape=box];

    "Bug appears deep in stack?" -> "Can trace backwards?" [label="yes"];
    "Can trace backwards?" -> "Trace to original trigger" [label="yes"];
    "Can trace backwards?" -> "Fix at symptom point" [label="no - dead end"];
    "Trace to original trigger" -> "BETTER: Also add defense-in-depth";
}
```

**使用场景：**
- 错误发生在执行深处（不是入口点）
- 堆栈跟踪显示长调用链
- 不清楚无效数据的来源
- 需要找到哪个测试/代码触发了问题

## 追踪过程

### 1. 观察症状
```
Error: git init failed in /Users/jesse/project/packages/core
```

### 2. 找到直接原因
**什么代码直接导致了这个问题？**
```typescript
await execFileAsync('git', ['init'], { cwd: projectDir });
```

### 3. 问：谁调用了这个？
```typescript
WorktreeManager.createSessionWorktree(projectDir, sessionId)
  → called by Session.initializeWorkspace()
  → called by Session.create()
  → called by test at Project.create()
```

### 4. 继续向上追踪
**传递了什么值？**
- `projectDir = ''` (空字符串！)
- 空字符串作为`cwd`解析为`process.cwd()`
- 那是源代码目录！

### 5. 找到原始触发器
**空字符串来自哪里？**
```typescript
const context = setupCoreTest(); // 返回 { tempDir: '' }
Project.create('name', context.tempDir); // 在beforeEach之前访问！
```

## 添加堆栈跟踪

当你无法手动追踪时，添加工具：

```typescript
// 在有问题的操作之前
async function gitInit(directory: string) {
  const stack = new Error().stack;
  console.error('DEBUG git init:', {
    directory,
    cwd: process.cwd(),
    nodeEnv: process.env.NODE_ENV,
    stack,
  });

  await execFileAsync('git', ['init'], { cwd: directory });
}
```

**关键：** 在测试中使用`console.error()`（不是logger - 可能不显示）

**运行并捕获：**
```bash
npm test 2>&1 | grep 'DEBUG git init'
```

**分析堆栈跟踪：**
- 查找测试文件名
- 找到触发调用的行号
- 识别模式（相同测试？相同参数？）

## 找到哪个测试导致污染

如果在测试期间出现问题但不知道是哪个测试：

使用此目录中的二分脚本 `find-polluter.sh`：

```bash
./find-polluter.sh '.git' 'src/**/*.test.ts'
```

逐个运行测试，在第一个污染者处停止。请参阅脚本了解用法。

## 真实示例：空projectDir

**症状：** `.git` 在 `packages/core/`（源代码）中创建

**追踪链：**
1. `git init` 在 `process.cwd()` 中运行 ← 空cwd参数
2. WorktreeManager使用空projectDir调用
3. Session.create()传递空字符串
4. 测试在beforeEach之前访问`context.tempDir`
5. setupCoreTest()最初返回 `{ tempDir: '' }`

**根本原因：** 顶层变量初始化访问空值

**修复：** 将tempDir改为getter，如果在beforeEach之前访问则抛出错误

**还添加了纵深防御：**
- 第一层：Project.create()验证目录
- 第二层：WorkspaceManager验证非空
- 第三层：NODE_ENV防护拒绝临时目录外的git init
- 第四层：git init前的堆栈跟踪日志

## 关键原则

```dot
digraph principle {
    "Found immediate cause" [shape=ellipse];
    "Can trace one level up?" [shape=diamond];
    "Trace backwards" [shape=box];
    "Is this the source?" [shape=diamond];
    "Fix at source" [shape=box];
    "Add validation at each layer" [shape=box];
    "Bug impossible" [shape=doublecircle];
    "NEVER fix just the symptom" [shape=octagon, style=filled, fillcolor=red, fontcolor=white];

    "Found immediate cause" -> "Can trace one level up?";
    "Can trace one level up?" -> "Trace backwards" [label="yes"];
    "Can trace one level up?" -> "NEVER fix just the symptom" [label="no"];
    "Trace backwards" -> "Is this the source?";
    "Is this the source?" -> "Trace backwards" [label="no - keeps going"];
    "Is this the source?" -> "Fix at source" [label="yes"];
    "Fix at source" -> "Add validation at each layer";
    "Add validation at each layer" -> "Bug impossible";
}
```

**永远不要只修复错误出现的地方。** 向后追踪找到原始触发器。

## 堆栈跟踪技巧

**在测试中：** 使用`console.error()`而不是logger - logger可能被抑制
**在操作前：** 在危险操作之前记录，而不是失败之后
**包含上下文：** 目录、cwd、环境变量、时间戳
**捕获堆栈：** `new Error().stack`显示完整调用链

## 实际影响

来自调试会话（2025-10-03）：
- 通过5层追踪找到根本原因
- 在源头修复（getter验证）
- 添加了4层防御
- 1847个测试通过，零污染