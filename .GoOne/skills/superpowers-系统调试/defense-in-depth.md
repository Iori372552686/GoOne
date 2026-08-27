# 纵深防御验证

## 概述

当你修复由无效数据引起的bug时，在一个地方添加验证感觉就足够了。但单个检查可以被不同的代码路径、重构或mock绕过。

**核心原则：** 在数据通过的每一层进行验证。使bug在结构上不可能发生。

## 为什么需要多层

单个验证："我们修复了bug"
多层验证："我们使bug不可能发生"

不同层捕获不同情况：
- 入口验证捕获大多数bug
- 业务逻辑捕获边缘情况
- 环境防护防止特定上下文的危险
- 调试日志在其他层失败时提供帮助

## 四层验证

### 第一层：入口点验证
**目的：** 在API边界拒绝明显无效的输入

```typescript
function createProject(name: string, workingDirectory: string) {
  if (!workingDirectory || workingDirectory.trim() === '') {
    throw new Error('workingDirectory cannot be empty');
  }
  if (!existsSync(workingDirectory)) {
    throw new Error(`workingDirectory does not exist: ${workingDirectory}`);
  }
  if (!statSync(workingDirectory).isDirectory()) {
    throw new Error(`workingDirectory is not a directory: ${workingDirectory}`);
  }
  // ... 继续
}
```

### 第二层：业务逻辑验证
**目的：** 确保数据对此操作有意义

```typescript
function initializeWorkspace(projectDir: string, sessionId: string) {
  if (!projectDir) {
    throw new Error('projectDir required for workspace initialization');
  }
  // ... 继续
}
```

### 第三层：环境防护
**目的：** 防止在特定上下文中执行危险操作

```typescript
async function gitInit(directory: string) {
  // 在测试中，拒绝在临时目录外执行git init
  if (process.env.NODE_ENV === 'test') {
    const normalized = normalize(resolve(directory));
    const tmpDir = normalize(resolve(tmpdir()));

    if (!normalized.startsWith(tmpDir)) {
      throw new Error(
        `Refusing git init outside temp dir during tests: ${directory}`
      );
    }
  }
  // ... 继续
}
```

### 第四层：调试工具
**目的：** 捕获法医分析的上下文

```typescript
async function gitInit(directory: string) {
  const stack = new Error().stack;
  logger.debug('About to git init', {
    directory,
    cwd: process.cwd(),
    stack,
  });
  // ... 继续
}
```

## 应用模式

当你发现bug时：

1. **追踪数据流** - 坏值起源于哪里？在哪里使用？
2. **映射所有检查点** - 列出数据通过的每个点
3. **在每层添加验证** - 入口、业务、环境、调试
4. **测试每层** - 尝试绕过第一层，验证第二层捕获它

## 会话示例

Bug：空的`projectDir`导致在源代码中执行`git init`

**数据流：**
1. 测试设置 → 空字符串
2. `Project.create(name, '')`
3. `WorkspaceManager.createWorkspace('')`
4. `git init` 在 `process.cwd()` 中运行

**添加的四层：**
- 第一层：`Project.create()` 验证非空/存在/可写
- 第二层：`WorkspaceManager` 验证projectDir非空
- 第三层：`WorktreeManager` 在测试中拒绝临时目录外的git init
- 第四层：git init前的堆栈跟踪日志

**结果：** 所有1847个测试通过，bug无法重现

## 关键洞察

所有四层都是必要的。在测试过程中，每层捕获了其他层遗漏的bug：
- 不同的代码路径绕过了入口验证
- Mock绕过了业务逻辑检查
- 不同平台上的边缘情况需要环境防护
- 调试日志识别了结构性误用

**不要停留在一个验证点。** 在每层添加检查。