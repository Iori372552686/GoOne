# 代码质量审查者提示模板

在调度代码质量审查者子代理时使用此模板。

**目的：** 验证实现构建良好（干净、经过测试、可维护）

**仅在规范合规审查通过后调度。**

```
Task tool (superpowers:code-reviewer):
  Use template at requesting-code-review/code-reviewer.md

  WHAT_WAS_IMPLEMENTED: [from implementer's report]
  PLAN_OR_REQUIREMENTS: Task N from [plan-file]
  BASE_SHA: [commit before task]
  HEAD_SHA: [current commit]
  DESCRIPTION: [task summary]
```

**除标准代码质量问题外，审查者还应检查：**
- 每个文件是否有明确的职责和定义良好的接口？
- 单元是否分解以便可以独立理解和测试？
- 实现是否遵循计划中的文件结构？
- 此实现是否创建了已经很大的新文件，或显著增加了现有文件的大小？（不要标记预先存在的文件大小——关注此更改贡献了什么。）

**代码审查者返回：** Strengths, Issues (Critical/Important/Minor), Assessment