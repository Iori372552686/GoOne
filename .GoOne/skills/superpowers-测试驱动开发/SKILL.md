---
name: superpowers-测试驱动开发
description: 在编写实现代码之前，实施任何功能或bug修复时使用
---

# 测试驱动开发 (TDD)

## 概述

先写测试。看着它失败。写最小的代码来通过。

**核心原则：** 如果你没有看到测试失败，你不知道它是否测试了正确的东西。

**违反规则的字面含义就是违反规则的精神。**

## 何时使用

**始终：**
- 新功能
- Bug修复
- 重构
- 行为更改

**例外（询问你的人类伙伴）：**
- 一次性原型
- 生成的代码
- 配置文件

认为"跳过TDD就这一次"？停止。那是合理化。

## 铁律

```
没有失败测试，不得编写生产代码
```

在测试之前写代码？删除它。重新开始。

**没有例外：**
- 不要保留它作为"参考"
- 不要在写测试时"适应"它
- 不要看它
- 删除意味着删除

从测试重新实现。就这样。

## 红-绿-重构

```dot
digraph tdd_cycle {
    rankdir=LR;
    red [label="RED\n写失败测试", shape=box, style=filled, fillcolor="#ffcccc"];
    verify_red [label="验证失败\n正确", shape=diamond];
    green [label="GREEN\n最小代码", shape=box, style=filled, fillcolor="#ccffcc"];
    verify_green [label="验证通过\n全部通过", shape=diamond];
    refactor [label="REFACTOR\n清理", shape=box, style=filled, fillcolor="#ccccff"];
    next [label="下一步", shape=ellipse];

    red -> verify_red;
    verify_red -> green [label="是"];
    verify_red -> red [label="错误\n失败"];
    green -> verify_green;
    verify_green -> refactor [label="是"];
    verify_green -> green [label="否"];
    refactor -> verify_green [label="保持\n通过"];
    verify_green -> next;
    next -> red;
}
```

### RED - 写失败测试

写一个最小的测试来显示应该发生什么。

<好>
```typescript
test('重试失败操作3次', async () => {
  let attempts = 0;
  const operation = () => {
    attempts++;
    if (attempts < 3) throw new Error('fail');
    return 'success';
  };

  const result = await retryOperation(operation);

  expect(result).toBe('success');
  expect(attempts).toBe(3);
});
```
名称清晰，测试真实行为，一件事
</好>

<坏>
```typescript
test('retry works', async () => {
  const mock = jest.fn()
    .mockRejectedValueOnce(new Error())
    .mockRejectedValueOnce(new Error())
    .mockResolvedValueOnce('success');
  await retryOperation(mock);
  expect(mock).toHaveBeenCalledTimes(3);
});
```
名称模糊，测试mock而不是代码
</坏>

**要求：**
- 一个行为
- 名称清晰
- 真实代码（除非不可避免，否则不使用mock）

### 验证RED - 看着它失败

**强制性。永远不要跳过。**

```bash
npm test path/to/test.test.ts
```

确认：
- 测试失败（不是错误）
- 失败消息符合预期
- 失败是因为功能缺失（不是拼写错误）

**测试通过？** 你正在测试现有行为。修复测试。

**测试错误？** 修复错误，重新运行直到正确失败。

### GREEN - 最小代码

写最简单的代码来通过测试。

<好>
```typescript
async function retryOperation<T>(fn: () => Promise<T>): Promise<T> {
  for (let i = 0; i < 3; i++) {
    try {
      return await fn();
    } catch (e) {
      if (i === 2) throw e;
    }
  }
  throw new Error('unreachable');
}
```
刚好通过
</好>

<坏>
```typescript
async function retryOperation<T>(
  fn: () => Promise<T>,
  options?: {
    maxRetries?: number;
    backoff?: 'linear' | 'exponential';
    onRetry?: (attempt: number) => void;
  }
): Promise<T> {
  // YAGNI
}
```
过度设计
</坏>

不要添加功能、重构其他代码或"改进"超出测试范围。

### 验证GREEN - 看着它通过

**强制性。**

```bash
npm test path/to/test.test.ts
```

确认：
- 测试通过
- 其他测试仍然通过
- 输出干净（无错误、警告）

**测试失败？** 修复代码，不是测试。

**其他测试失败？** 现在修复。

### REFACTOR - 清理

仅在绿色之后：
- 删除重复
- 改进名称
- 提取辅助函数

保持测试通过。不要添加行为。

### 重复

下一个失败测试用于下一个功能。

## 良好测试

| 质量 | 好 | 坏 |
|---------|------|-----|
| **最小** | 一件事。名称中有"and"？拆分它。 | `test('validates email and domain and whitespace')` |
| **清晰** | 名称描述行为 | `test('test1')` |
| **显示意图** | 演示所需的API | 模糊代码应该做什么 |

## 为什么顺序重要

**"我会在之后写测试来验证它有效"**

在代码之后写的测试立即通过。立即通过证明不了什么：
- 可能测试错误的东西
- 可能测试实现，而不是行为
- 可能遗漏你忘记的边缘情况
- 你从未看到它捕获bug

测试优先迫使你看到测试失败，证明它实际上测试了某些东西。

**"我已经手动测试了所有边缘情况"**

手动测试是临时的。你认为你测试了一切，但：
- 没有记录你测试了什么
- 代码更改时无法重新运行
- 在压力下容易忘记情况
- "我尝试时它工作" ≠ 全面

自动化测试是系统性的。它们每次都以相同方式运行。

**"删除X小时的工作是浪费的"**

沉没成本谬误。时间已经过去了。你现在的选择：
- 删除并使用TDD重写（再花X小时，高信心）
- 保留它并在之后添加测试（30分钟，低信心，可能有bug）

"浪费"是保留你不能信任的代码。没有真实测试的工作代码是技术债务。

**"TDD是教条的，务实意味着适应"**

TDD IS务实：
- 在提交前找到bug（比之后调试更快）
- 防止回归（测试立即捕获中断）
- 记录行为（测试显示如何使用代码）
- 支持重构（自由更改，测试捕获中断）

"务实"捷径 = 在生产中调试 = 更慢。

**"之后测试实现相同的目标 - 这是精神不是仪式"**

不。之后测试回答"这做什么？"。测试优先回答"这应该做什么？"

之后测试受你的实现偏见影响。你测试你构建的东西，而不是需要的东西。你验证记住的边缘情况，而不是发现的边缘情况。

测试优先迫使在实施前发现边缘情况。之后测试验证你记住了一切（你没有）。

30分钟的事后测试 ≠ TDD。你获得覆盖率，失去测试工作的证明。

## 常见合理化

| 借口 | 现实 |
|--------|---------|
| "太简单不需要测试" | 简单代码会中断。测试需要30秒。 |
| "我会在之后测试" | 测试立即通过证明不了什么。 |
| "之后测试实现相同目标" | 之后测试 = "这做什么？" 测试优先 = "这应该做什么？" |
| "已经手动测试" | 临时 ≠ 系统。没有记录，无法重新运行。 |
| "删除X小时是浪费的" | 沉没成本谬误。保留未验证的代码是技术债务。 |
| "保留作为参考，先写测试" | 你会适应它。那是事后测试。删除意味着删除。 |
| "需要先探索" | 好的。扔掉探索，从TDD开始。 |
| "测试困难 = 设计不清晰" | 倾听测试。难以测试 = 难以使用。 |
| "TDD会减慢我的速度" | TDD比调试更快。务实 = 测试优先。 |
| "手动测试更快" | 手动不证明边缘情况。每次更改都要重新测试。 |
| "现有代码没有测试" | 你正在改进它。为现有代码添加测试。 |

## 危险信号 - 停止并重新开始

- 代码在测试之前
- 测试在实现之后
- 测试立即通过
- 无法解释为什么测试失败
- 测试"稍后"添加
- 合理化"就这一次"
- "我已经手动测试过了"
- "之后测试实现相同的目的"
- "这是精神不是仪式"
- "保留作为参考"或"适应现有代码"
- "已经花了X小时，删除是浪费的"
- "TDD是教条的，我是务实的"
- "这不同因为..."

**所有这些都意味着：删除代码。使用TDD重新开始。**

## 示例：Bug修复

**Bug：** 接受空邮箱

**RED**
```typescript
test('拒绝空邮箱', async () => {
  const result = await submitForm({ email: '' });
  expect(result.error).toBe('Email required');
});
```

**验证RED**
```bash
$ npm test
FAIL: expected 'Email required', got undefined
```

**GREEN**
```typescript
function submitForm(data: FormData) {
  if (!data.email?.trim()) {
    return { error: 'Email required' };
  }
  // ...
}
```

**验证GREEN**
```bash
$ npm test
PASS
```

**REFACTOR**
如果需要，为多个字段提取验证。

## 验证清单

在标记工作完成之前：

- [ ] 每个新函数/方法都有测试
- [ ] 在实施前观看每个测试失败
- [ ] 每个测试因预期原因失败（功能缺失，不是拼写错误）
- [ ] 写最小代码来通过每个测试
- [ ] 所有测试通过
- [ ] 输出干净（无错误、警告）
- [ ] 测试使用真实代码（仅在不可避免时使用mock）
- [ ] 覆盖边缘情况和错误

无法勾选所有框？你跳过了TDD。重新开始。

## 卡住时

| 问题 | 解决方案 |
|---------|----------|
| 不知道如何测试 | 写期望的API。先写断言。询问你的人类伙伴。 |
| 测试太复杂 | 设计太复杂。简化接口。 |
| 必须mock一切 | 代码耦合太强。使用依赖注入。 |
| 测试设置庞大 | 提取辅助函数。仍然复杂？简化设计。 |

## 调试集成

发现bug？写失败测试重现它。遵循TDD循环。测试证明修复并防止回归。

修复bug必须有测试。

## 测试反模式

添加mock或测试工具时，请阅读@testing-anti-patterns.md以避免常见陷阱：
- 测试mock行为而不是真实行为
- 向生产类添加仅测试方法
- 在不理解依赖关系的情况下mock

## 最终规则

```
生产代码 → 测试存在并首先失败
否则 → 不是TDD
```

未经你的人类伙伴许可，没有例外。