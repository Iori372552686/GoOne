# 测试反模式

**加载此参考时：** 编写或更改测试、添加mock，或想向生产代码添加仅测试方法时。

## 概述

测试必须验证真实行为，而不是mock行为。Mock是隔离的手段，不是测试的对象。

**核心原则：** 测试代码做什么，而不是mock做什么。

**遵循严格的TDD可防止这些反模式。**

## 铁律

```
1. 永远不要测试mock行为
2. 永远不要向生产类添加仅测试方法
3. 永远不要在不理解依赖的情况下使用mock
```

## 反模式1：测试Mock行为

**违规：**
```typescript
// ❌ BAD: Testing that the mock exists
test('renders sidebar', () => {
  render(<Page />);
  expect(screen.getByTestId('sidebar-mock')).toBeInTheDocument();
});
```

**为什么这是错误的：**
- 您正在验证mock是否工作，而不是组件是否工作
- 测试在mock存在时通过，不存在时失败
- 无法说明真实行为

**你的人类伙伴的纠正：** "我们在测试mock的行为吗？"

**修复：**
```typescript
// ✅ GOOD: Test real component or don't mock it
test('renders sidebar', () => {
  render(<Page />);  // Don't mock sidebar
  expect(screen.getByRole('navigation')).toBeInTheDocument();
});

// OR if sidebar must be mocked for isolation:
// Don't assert on the mock - test Page's behavior with sidebar present
```

### 门函数

```
在对任何mock元素进行断言之前：
  问："我在测试真实组件行为还是只是mock存在？"

  如果测试mock存在：
    停止 - 删除断言或取消mock组件

  改为测试真实行为
```

## 反模式2：生产代码中的仅测试方法

**违规：**
```typescript
// ❌ BAD: destroy() only used in tests
class Session {
  async destroy() {  // Looks like production API!
    await this._workspaceManager?.destroyWorkspace(this.id);
    // ... cleanup
  }
}

// In tests
afterEach(() => session.destroy());
```

**为什么这是错误的：**
- 生产类被仅测试代码污染
- 在生产中意外调用时很危险
- 违反YAGNI和关注点分离
- 混淆对象生命周期与实体生命周期

**修复：**
```typescript
// ✅ GOOD: Test utilities handle test cleanup
// Session has no destroy() - it's stateless in production

// In test-utils/
export async function cleanupSession(session: Session) {
  const workspace = session.getWorkspaceInfo();
  if (workspace) {
    await workspaceManager.destroyWorkspace(workspace.id);
  }
}

// In tests
afterEach(() => cleanupSession(session));
```

### 门函数

```
在向生产类添加任何方法之前：
  问："这只被测试使用吗？"

  如果是：
    停止 - 不要添加它
    改为放在测试工具中

  问："这个类拥有这个资源的生命周期吗？"

  如果不是：
    停止 - 这个方法放错类了
```

## 反模式3：在不理解的情况下Mock

**违规：**
```typescript
// ❌ BAD: Mock breaks test logic
test('detects duplicate server', () => {
  // Mock prevents config write that test depends on!
  vi.mock('ToolCatalog', () => ({
    discoverAndCacheTools: vi.fn().mockResolvedValue(undefined)
  }));

  await addServer(config);
  await addServer(config);  // Should throw - but won't!
});
```

**为什么这是错误的：**
- Mock方法有测试依赖的副作用（写入配置）
- "为了安全"过度mock破坏了实际行为
- 测试因错误原因通过或神秘失败

**修复：**
```typescript
// ✅ GOOD: Mock at correct level
test('detects duplicate server', () => {
  // Mock the slow part, preserve behavior test needs
  vi.mock('MCPServerManager'); // Just mock slow server startup

  await addServer(config);  // Config written
  await addServer(config);  // Duplicate detected ✓
});
```

### 门函数

```
在mock任何方法之前：
  停止 - 先不要mock

  1. 问："真实方法有什么副作用？"
  2. 问："这个测试依赖这些副作用中的任何一个吗？"
  3. 问："我完全理解这个测试需要什么吗？"

  如果依赖副作用：
    在较低级别mock（实际的慢速/外部操作）
    或使用保留必要行为的测试替身
    不是测试依赖的高级方法

  如果不确定测试依赖什么：
    首先使用真实实现运行测试
    观察实际需要发生什么
    然后在正确级别添加最小mock

  危险信号：
    - "我会mock这个以确保安全"
    - "这可能很慢，最好mock它"
    - 在不理解依赖链的情况下进行mock
```

## 反模式4：不完整的Mock

**违规：**
```typescript
// ❌ BAD: Partial mock - only fields you think you need
const mockResponse = {
  status: 'success',
  data: { userId: '123', name: 'Alice' }
  // Missing: metadata that downstream code uses
};

// Later: breaks when code accesses response.metadata.requestId
```

**为什么这是错误的：**
- **部分mock隐藏结构假设** - 您只mock了您知道的字段
- **下游代码可能依赖您未包含的字段** - 静默失败
- **测试通过但集成失败** - Mock不完整，真实API完整
- **虚假信心** - 测试无法证明真实行为

**铁律：** Mock完整的数据结构，就像它在现实中存在的那样，而不仅仅是您即时测试使用的字段。

**修复：**
```typescript
// ✅ GOOD: Mirror real API completeness
const mockResponse = {
  status: 'success',
  data: { userId: '123', name: 'Alice' },
  metadata: { requestId: 'req-789', timestamp: 1234567890 }
  // All fields real API returns
};
```

### 门函数

```
在创建mock响应之前：
  检查："真实API响应包含哪些字段？"

  操作：
    1. 检查文档/示例中的实际API响应
    2. 包含系统可能在下游消费的所有字段
    3. 验证mock完全匹配真实响应模式

  关键：
    如果您要创建mock，您必须理解整个结构
    当代码依赖省略的字段时，部分mock会静默失败

  如果不确定：包含所有文档字段
```

## 反模式5：集成测试事后考虑

**违规：**
```
✅ Implementation complete
❌ No tests written
"Ready for testing"
```

**为什么这是错误的：**
- 测试是实现的一部分，不是可选的后续工作
- TDD会捕获这一点
- 没有测试不能声称完成

**修复：**
```
TDD cycle:
1. Write failing test
2. Implement to pass
3. Refactor
4. THEN claim complete
```

## 当Mock变得过于复杂时

**警告信号：**
- Mock设置比测试逻辑长
- Mock一切以使测试通过
- Mock缺少真实组件拥有的方法
- Mock更改时测试失败

**你的人类伙伴的问题：** "我们需要在这里使用mock吗？"

**考虑：** 使用真实组件的集成测试通常比复杂mock更简单

## TDD防止这些反模式

**为什么TDD有帮助：**
1. **先写测试** → 迫使您思考实际测试什么
2. **观察失败** → 确认测试测试真实行为，而非mock
3. **最小实现** → 没有仅测试方法潜入
4. **真实依赖** → 在mock之前您看到测试实际需要什么

**如果您正在测试mock行为，您违反了TDD** - 您在没有先观察测试对真实代码失败的情况下添加了mock。

## 快速参考

| 反模式 | 修复 |
|--------------|-----|
| 对mock元素断言 | 测试真实组件或取消mock |
| 生产中的仅测试方法 | 移到测试工具 |
| 在不理解的情况下Mock | 先理解依赖，最小化mock |
| 不完整的mock | 完全镜像真实API |
| 测试事后考虑 | TDD - 先测试 |
| 过度复杂的mock | 考虑集成测试 |

## 危险信号

- 断言检查`*-mock`测试ID
- 方法只在测试文件中调用
- Mock设置占测试的>50%
- 删除mock时测试失败
- 无法解释为什么需要mock
- "为了安全"而mock

## 底线

**Mock是隔离的工具，不是测试的对象。**

如果TDD揭示您正在测试mock行为，您就错了。

修复：测试真实行为或质疑为什么要mock。