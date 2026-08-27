# 技能编写最佳实践

> 学习如何编写Claude可以发现和成功使用的有效技能。

优秀的技能简洁、结构良好，并经过实际使用测试。本指南提供实用的编写决策，帮助您编写Claude可以发现和有效使用的技能。

有关技能工作原理的概念背景，请参阅[技能概述](/en/docs/agents-and-tools/agent-skills/overview)。

## 核心原则

### 简洁是关键

[上下文窗口](https://platform.claude.com/docs/en/build-with-claude/context-windows)是公共资源。您的技能与Claude需要知道的其他所有内容共享上下文窗口，包括：

* 系统提示
* 对话历史
* 其他技能的元数据
* 您的实际请求

技能中的每个token并不立即产生成本。启动时，只预加载所有技能的元数据（名称和描述）。Claude仅在技能相关时才读取SKILL.md，并且仅在需要时才读取其他文件。然而，SKILL.md中的简洁性仍然很重要：一旦Claude加载它，每个token都会与对话历史和其他上下文竞争。

**默认假设**：Claude已经非常聪明

只添加Claude还没有的上下文。对每条信息提出质疑：

* "Claude真的需要这个解释吗？"
* "我可以假设Claude知道这个吗？"
* "这段话值得它的token成本吗？"

**好示例：简洁**（约50个token）：

```markdown
## 提取PDF文本

使用pdfplumber进行文本提取：

```python
import pdfplumber

with pdfplumber.open("file.pdf") as pdf:
    text = pdf.pages[0].extract_text()
```
```

**坏示例：过于冗长**（约150个token）：

```markdown
## 提取PDF文本

PDF（便携式文档格式）文件是一种常见的文件格式，包含文本、图像和其他内容。要从PDF中提取文本，您需要使用库。有许多库可用于PDF处理，但我们建议使用pdfplumber，因为它易于使用且能很好地处理大多数情况。首先，您需要使用pip安装它。然后您可以使用下面的代码...
```

简洁版本假设Claude知道PDF是什么以及库如何工作。

### 设置适当的自由度

将具体程度与任务的脆弱性和可变性相匹配。

**高自由度**（基于文本的指令）：

使用场景：

* 多种方法都有效
* 决策取决于上下文
* 启发式方法指导方法

示例：

```markdown
## 代码审查流程

1. 分析代码结构和组织
2. 检查潜在的bug或边缘情况
3. 建议提高可读性和可维护性的改进
4. 验证是否符合项目约定
```

**中等自由度**（带参数的伪代码或脚本）：

使用场景：

* 存在首选模式
* 允许一些变化
* 配置影响行为

示例：

```markdown
## 生成报告

使用此模板并根据需要自定义：

```python
def generate_report(data, format="markdown", include_charts=True):
    # 处理数据
    # 以指定格式生成输出
    # 可选包含可视化
```
```

**低自由度**（特定脚本，很少或没有参数）：

使用场景：

* 操作脆弱且容易出错
* 一致性至关重要
* 必须遵循特定顺序

示例：

```markdown
## 数据库迁移

运行此脚本：

```bash
python scripts/migrate.py --verify --backup
```

不要修改命令或添加额外标志。
```

**类比**：将Claude想象成探索路径的机器人：

* **狭窄的桥梁，两侧是悬崖**：只有一条安全的前进道路。提供特定的护栏和精确的指令（低自由度）。示例：必须按精确顺序运行的数据库迁移。
* **没有危险的开阔场地**：许多路径通向成功。给出大致方向并相信Claude找到最佳路线（高自由度）。示例：代码审查，其中上下文决定最佳方法。

### 使用所有计划使用的模型进行测试

技能作为模型的补充，因此有效性取决于底层模型。使用所有计划使用的模型测试您的技能。

**按模型测试考虑：**

* **Claude Haiku**（快速、经济）：技能是否提供足够的指导？
* **Claude Sonnet**（平衡）：技能是否清晰高效？
* **Claude Opus**（强大的推理）：技能是否避免过度解释？

对Opus完美工作的内容可能需要为Haiku提供更多细节。如果您计划跨多个模型使用技能，请旨在使用适用于所有模型的指令。

## 技能结构

<Note>
  **YAML Frontmatter**：SKILL.md frontmatter需要两个字段：

  * `name` - 技能的人类可读名称（最多64个字符）
  * `description` - 技能作用和使用时机的单行描述（最多1024个字符）

  有关完整的技能结构详细信息，请参阅[技能概述](/en/docs/agents-and-tools/agent-skills/overview#skill-structure)。
</Note>

### 命名约定

使用一致的命名模式使技能更容易引用和讨论。我们建议对技能名称使用**动名词形式**（动词+ing），因为这清楚地描述了技能提供的活动或能力。

**良好命名示例（动名词形式）**：

* "Processing PDFs"
* "Analyzing spreadsheets"
* "Managing databases"
* "Testing code"
* "Writing documentation"

**可接受的替代方案**：

* 名词短语："PDF Processing"、"Spreadsheet Analysis"
* 面向动作："Process PDFs"、"Analyze Spreadsheets"

**避免**：

* 模糊名称："Helper"、"Utils"、"Tools"
* 过于通用："Documents"、"Data"、"Files"
* 技能集合中不一致的模式

一致的命名使：

* 在文档和对话中引用技能更容易
* 一眼就能理解技能的作用
* 组织和搜索多个技能
* 维护专业、连贯的技能库

### 编写有效的描述

`description`字段启用技能发现，应包括技能的作用和使用时机。

<Warning>
  **始终使用第三人称写作**。描述被注入到系统提示中，不一致的视角可能导致发现问题。

  * **Good:** "Processes Excel files and generates reports"
  * **Avoid:** "I can help you process Excel files"
  * **Avoid:** "You can use this to process Excel files"
</Warning>

**具体并包含关键术语**。包括技能的作用和使用它的特定触发器/上下文。

每个技能恰好有一个描述字段。描述对于技能选择至关重要：Claude使用它从可能100+可用技能中选择正确的技能。您的描述必须提供足够的细节，让Claude知道何时选择此技能，而SKILL.md的其余部分提供实现细节。

有效示例：

**PDF处理技能**：

```yaml
description: Extract text and tables from PDF files, fill forms, merge documents. Use when working with PDF files or when the user mentions PDFs, forms, or document extraction.
```

**Excel分析技能**：

```yaml
description: Analyze Excel spreadsheets, create pivot tables, generate charts. Use when analyzing Excel files, spreadsheets, tabular data, or .xlsx files.
```

**Git提交助手技能**：

```yaml
description: Generate descriptive commit messages by analyzing git diffs. Use when the user asks for help writing commit messages or reviewing staged changes.
```

避免这样的模糊描述：

```yaml
description: Helps with documents
```

```yaml
description: Processes data
```

```yaml
description: Does stuff with files
```

### 渐进式披露模式

SKILL.md作为概述，在需要时指向详细材料，就像入职指南中的目录一样。有关渐进式披露如何工作的解释，请参阅概述中的[技能如何工作](/en/docs/agents-and-tools/agent-skills/overview#how-skills-work)。

**实用指导**：

* 将SKILL.md正文保持在500行以下以获得最佳性能
* 接近此限制时将内容拆分为单独的文件
* 使用以下模式有效组织指令、代码和资源

#### 可视化概述：从简单到复杂

基本技能从仅包含元数据和指令的SKILL.md文件开始：

<img src="https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-simple-file.png?fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=87782ff239b297d9a9e8e1b72ed72db9" alt="Simple SKILL.md file showing YAML frontmatter and markdown body" data-og-width="2048" width="2048" data-og-height="1153" height="1153" data-path="images/agent-skills-simple-file.png" data-optimize="true" data-opv="3" srcset="https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-simple-file.png?w=280&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=c61cc33b6f5855809907f7fda94cd80e 280w, https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-simple-file.png?w=560&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=90d2c0c1c76b36e8d485f49e0810dbfd 560w, https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-simple-file.png?w=840&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=ad17d231ac7b0bea7e5b4d58fb4aeabb 840w, https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-simple-file.png?w=1100&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=f5d0a7a3c668435bb0aee9a3a8f8c329 1100w, https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-simple-file.png?w=1650&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=0e927c1af9de5799cfe557d12249f6e6 1650w, https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-simple-file.png?w=2500&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=46bbb1a51dd4c8202a470ac8c80a893d 2500w" />

随着技能的增长，您可以捆绑Claude仅在需要时加载的额外内容：

<img src="https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-bundling-content.png?fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=a5e0aa41e3d53985a7e3e43668a33ea3" alt="Bundling additional reference files like reference.md and forms.md." data-og-width="2048" width="2048" data-og-height="1327" height="1327" data-path="images/agent-skills-bundling-content.png" data-optimize="true" data-opv="3" srcset="https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-bundling-content.png?w=280&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=f8a0e73783e99b4a643d79eac86b70a2 280w, https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-bundling-content.png?w=560&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=dc510a2a9d3f14359416b706f067904a 560w, https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-bundling-content.png?w=840&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=82cd6286c966303f7dd914c28170e385 840w, https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-bundling-content.png?w=1100&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=56f3be36c77e4fe4b523df209a6824c6 1100w, https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-bundling-content.png?w=1650&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=d22b5161b2075656417d56f41a74f3dd 1650w, https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-bundling-content.png?w=2500&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=3dd4bdd6850ffcc96c6c45fcb0acd6eb 2500w" />

完整的技能目录结构可能如下所示：

```
pdf/
├── SKILL.md              # 主要指令（触发时加载）
├── FORMS.md              # 表单填写指南（按需加载）
├── reference.md          # API参考（按需加载）
├── examples.md           # 使用示例（按需加载）
└── scripts/
    ├── analyze_form.py   # 实用脚本（执行，不加载）
    ├── fill_form.py      # 表单填写脚本
    └── validate.py       # 验证脚本
```

#### 模式1：高级指南与参考

```markdown
---
name: PDF Processing
description: Extracts text and tables from PDF files, fills forms, and merges documents. Use when working with PDF files or when the user mentions PDFs, forms, or document extraction.
---

# PDF Processing

## 快速开始

使用pdfplumber提取文本：
```python
import pdfplumber
with pdfplumber.open("file.pdf") as pdf:
    text = pdf.pages[0].extract_text()
```

## 高级功能

**表单填写**：请参阅[FORMS.md](FORMS.md)获取完整指南
**API参考**：请参阅[REFERENCE.md](REFERENCE.md)获取所有方法
**示例**：请参阅[EXAMPLES.md](EXAMPLES.md)获取常见模式
```

Claude仅在需要时加载FORMS.md、REFERENCE.md或EXAMPLES.md。

#### 模式2：领域特定组织

对于具有多个领域的技能，按领域组织内容以避免加载无关上下文。当用户询问销售指标时，Claude只需要读取销售相关的模式，而不是财务或营销数据。这保持token使用量低且上下文集中。

```
bigquery-skill/
├── SKILL.md (概述和导航)
└── reference/
    ├── finance.md (收入、计费指标)
    ├── sales.md (机会、管道)
    ├── product.md (API使用、功能)
    └── marketing.md (活动、归因)
```

```markdown
# BigQuery Data Analysis

## 可用数据集

**Finance**: Revenue, ARR, billing → See [reference/finance.md](reference/finance.md)
**Sales**: Opportunities, pipeline, accounts → See [reference/sales.md](reference/sales.md)
**Product**: API usage, features, adoption → See [reference/product.md](reference/product.md)
**Marketing**: Campaigns, attribution, email → See [reference/marketing.md](reference/marketing.md)

## 快速搜索

使用grep查找特定指标：

```bash
grep -i "revenue" reference/finance.md
grep -i "pipeline" reference/sales.md
grep -i "api usage" reference/product.md
```
```

#### 模式3：条件细节

显示基本内容，链接到高级内容：

```markdown
# DOCX Processing

## 创建文档

使用docx-js创建新文档。请参阅[DOCX-JS.md](DOCX-JS.md)。

## 编辑文档

对于简单编辑，直接修改XML。

**对于跟踪更改**：请参阅[REDLINING.md](REDLINING.md)
**对于OOXML详细信息**：请参阅[OOXML.md](OOXML.md)
```

Claude仅在用户需要这些功能时才读取REDLINING.md或OOXML.md。

### 避免深层嵌套引用

当从其他引用文件引用文件时，Claude可能会部分读取文件。遇到嵌套引用时，Claude可能使用`head -100`等命令预览内容而不是读取整个文件，导致信息不完整。

**保持引用从SKILL.md开始只有一层深**。所有引用文件应直接从SKILL.md链接，确保Claude在需要时读取完整文件。

**坏示例：太深**：

```markdown
# SKILL.md
See [advanced.md](advanced.md)...

# advanced.md
See [details.md](details.md)...

# details.md
Here's the actual information...
```

**好示例：一层深**：

```markdown
# SKILL.md

**Basic usage**: [instructions in SKILL.md]
**Advanced features**: See [advanced.md](advanced.md)
**API reference**: See [reference.md](reference.md)
**Examples**: See [examples.md](examples.md)
```

### 使用目录结构较长的参考文件

对于超过100行的参考文件，在顶部包含目录。这确保Claude即使在部分读取预览时也能看到可用信息的完整范围。

**示例**：

```markdown
# API Reference

## Contents
- Authentication and setup
- Core methods (create, read, update, delete)
- Advanced features (batch operations, webhooks)
- Error handling patterns
- Code examples

## Authentication and setup
...

## Core methods
...
```

然后Claude可以根据需要读取完整文件或跳转到特定部分。

有关此基于文件系统的架构如何实现渐进式披露的详细信息，请参阅下面高级部分中的[运行时环境](#runtime-environment)部分。

## 工作流和反馈循环

### 对复杂任务使用工作流

将复杂操作分解为清晰的顺序步骤。对于特别复杂的工作流，提供一个检查清单，Claude可以复制到其响应中并在进展过程中勾选。

**示例1：研究综合工作流**（无代码技能）：

```markdown
## 研究综合工作流

复制此检查清单并跟踪您的进度：

```
Research Progress:
- [ ] Step 1: Read all source documents
- [ ] Step 2: Identify key themes
- [ ] Step 3: Cross-reference claims
- [ ] Step 4: Create structured summary
- [ ] Step 5: Verify citations
```

**Step 1: Read all source documents**

Review each document in the `sources/` directory. Note the main arguments and supporting evidence.

**Step 2: Identify key themes**

Look for patterns across sources. What themes appear repeatedly? Where do sources agree or disagree?

**Step 3: Cross-reference claims**

For each major claim, verify it appears in the source material. Note which source supports each point.

**Step 4: Create structured summary**

Organize findings by theme. Include:
- Main claim
- Supporting evidence from sources
- Conflicting viewpoints (if any)

**Step 5: Verify citations**

Check that every claim references the correct source document. If citations are incomplete, return to Step 3.
```

此示例展示了工作流如何应用于不需要代码的分析任务。检查清单模式适用于任何复杂的多步骤过程。

**示例2：PDF表单填写工作流**（带代码技能）：

```markdown
## PDF表单填写工作流

复制此检查清单并在完成时勾选项目：

```
Task Progress:
- [ ] Step 1: Analyze the form (run analyze_form.py)
- [ ] Step 2: Create field mapping (edit fields.json)
- [ ] Step 3: Validate mapping (run validate_fields.py)
- [ ] Step 4: Fill the form (run fill_form.py)
- [ ] Step 5: Verify output (run verify_output.py)
```

**Step 1: Analyze the form**

Run: `python scripts/analyze_form.py input.pdf`

This extracts form fields and their locations, saving to `fields.json`.

**Step 2: Create field mapping**

Edit `fields.json` to add values for each field.

**Step 3: Validate mapping**

Run: `python scripts/validate_fields.py fields.json`

Fix any validation errors before continuing.

**Step 4: Fill the form**

Run: `python scripts/fill_form.py input.pdf fields.json output.pdf`

**Step 5: Verify output**

Run: `python scripts/verify_output.py output.pdf`

If verification fails, return to Step 2.
```

清晰的步骤防止Claude跳过关键验证。检查清单帮助Claude和您跟踪多步骤工作流的进度。

### 实施反馈循环

**常见模式**：运行验证器→修复错误→重复

此模式大大提高输出质量。

**示例1：样式指南合规**（无代码技能）：

```markdown
## 内容审查流程

1. Draft your content following the guidelines in STYLE_GUIDE.md
2. Review against the checklist:
   - Check terminology consistency
   - Verify examples follow the standard format
   - Confirm all required sections are present
3. If issues found:
   - Note each issue with specific section reference
   - Revise the content
   - Review the checklist again
4. Only proceed when all requirements are met
5. Finalize and save the document
```

这展示了使用参考文档而非脚本的验证循环模式。"验证器"是STYLE_GUIDE.md，Claude通过读取和比较来执行检查。

**示例2：文档编辑过程**（带代码技能）：

```markdown
## 文档编辑过程

1. Make your edits to `word/document.xml`
2. **Validate immediately**: `python ooxml/scripts/validate.py unpacked_dir/`
3. If validation fails:
   - Review the error message carefully
   - Fix the issues in the XML
   - Run validation again
4. **Only proceed when validation passes**
5. Rebuild: `python ooxml/scripts/pack.py unpacked_dir/ output.docx`
6. Test the output document
```

验证循环及早捕获错误。

## 内容指南

### 避免时间敏感信息

不要包含会过时的信息：

**坏示例：时间敏感**（将会变错）：

```markdown
If you're doing this before August 2025, use the old API.
After August 2025, use the new API.
```

**好示例**（使用"旧模式"部分）：

```markdown
## Current method

Use the v2 API endpoint: `api.example.com/v2/messages`

## Old patterns

<details>
<summary>Legacy v1 API (deprecated 2025-08)</summary>

The v1 API used: `api.example.com/v1/messages`

This endpoint is no longer supported.
</details>
```

旧模式部分提供历史背景而不干扰主要内容。

### 使用一致的术语

选择一个术语并在整个技能中使用：

**Good - Consistent**：

* Always "API endpoint"
* Always "field"
* Always "extract"

**Bad - Inconsistent**：

* Mix "API endpoint", "URL", "API route", "path"
* Mix "field", "box", "element", "control"
* Mix "extract", "pull", "get", "retrieve"

一致性帮助Claude理解和遵循指令。

## 常见模式

### 模板模式

为输出格式提供模板。根据您的需求匹配严格程度。

**对于严格要求**（如API响应或数据格式）：

```markdown
## Report structure

ALWAYS use this exact template structure:

```markdown
# [Analysis Title]

## Executive summary
[One-paragraph overview of key findings]

## Key findings
- Finding 1 with supporting data
- Finding 2 with supporting data
- Finding 3 with supporting data

## Recommendations
1. Specific actionable recommendation
2. Specific actionable recommendation
```
```

**对于灵活指导**（当适应有用时）：

```markdown
## Report structure

Here is a sensible default format, but use your best judgment based on the analysis:

```markdown
# [Analysis Title]

## Executive summary
[Overview]

## Key findings
[Adapt sections based on what you discover]

## Recommendations
[Tailor to the specific context]
```

Adjust sections as needed for the specific analysis type.
```

### 示例模式

对于输出质量取决于查看示例的技能，像常规提示一样提供输入/输出对：

```markdown
## Commit message format

Generate commit messages following these examples:

**Example 1:**
Input: Added user authentication with JWT tokens
Output:
```
feat(auth): implement JWT-based authentication

Add login endpoint and token validation middleware
```

**Example 2:**
Input: Fixed bug where dates displayed incorrectly in reports
Output:
```
fix(reports): correct date formatting in timezone conversion

Use UTC timestamps consistently across report generation
```

**Example 3:**
Input: Updated dependencies and refactored error handling
Output:
```
chore: update dependencies and refactor error handling

- Upgrade lodash to 4.17.21
- Standardize error response format across endpoints
```

Follow this style: type(scope): brief description, then detailed explanation.
```

示例帮助Claude比仅描述更清楚地理解所需的样式和详细程度。

### 条件工作流模式

引导Claude通过决策点：

```markdown
## Document modification workflow

1. Determine the modification type:

   **Creating new content?** → Follow "Creation workflow" below
   **Editing existing content?** → Follow "Editing workflow" below

2. Creation workflow:
   - Use docx-js library
   - Build document from scratch
   - Export to .docx format

3. Editing workflow:
   - Unpack existing document
   - Modify XML directly
   - Validate after each change
   - Repack when complete
```

<Tip>
  If workflows become large or complicated with many steps, consider pushing them into separate files and tell Claude to read the appropriate file based on the task at hand.
</Tip>

## 评估和迭代

### 先构建评估

**在编写大量文档之前创建评估。** 这确保您的技能解决实际问题，而不是记录想象中的问题。

**评估驱动开发：**

1. **识别差距**：在没有技能的情况下运行Claude处理代表性任务。记录特定失败或缺失的上下文
2. **创建评估**：构建三个测试这些差距的场景
3. **建立基线**：测量Claude在没有技能情况下的性能
4. **编写最小指令**：创建足够的内容来解决差距并通过评估
5. **迭代**：执行评估，与基线比较，并改进

此方法确保您解决实际问题，而不是预测可能永远不会出现的需求。

**评估结构**：

```json
{
  "skills": ["pdf-processing"],
  "query": "Extract all text from this PDF file and save it to output.txt",
  "files": ["test-files/document.pdf"],
  "expected_behavior": [
    "Successfully reads the PDF file using an appropriate PDF processing library or command-line tool",
    "Extracts text content from all pages in the document without missing any pages",
    "Saves the extracted text to a file named output.txt in a clear, readable format"
  ]
}
```

<Note>
  此示例演示了一个带有简单测试规则的数据驱动评估。我们目前不提供运行这些评估的内置方式。用户可以创建自己的评估系统。评估是衡量技能有效性的真实来源。
</Note>

### 与Claude迭代开发技能

最有效的技能开发过程涉及Claude本身。与Claude的一个实例（"Claude A"）一起创建将由其他实例（"Claude B"）使用的技能。Claude A帮助您设计和改进指令，而Claude B在实际任务中测试它们。这之所以有效，是因为Claude模型既了解如何编写有效的代理指令，也了解代理需要什么信息。

**创建新技能：**

1. **在没有技能的情况下完成任务**：使用普通提示与Claude A解决问题。在工作过程中，您会自然地提供上下文、解释偏好和分享程序知识。注意您反复提供的信息。

2. **识别可重用模式**：完成任务后，确定您提供的哪些上下文对类似的未来任务有用。

   **示例**：如果您完成了BigQuery分析，您可能提供了表名、字段定义、过滤规则（如"始终排除测试账户"）和常见查询模式。

3. **让Claude A创建技能**："创建一个技能，捕获我们刚刚使用的这个BigQuery分析模式。包括表模式、命名约定和过滤测试账户的规则。"

   <Tip>
     Claude模型本地理解技能格式和结构。您不需要特殊的系统提示或"编写技能"技能来让Claude帮助创建技能。只需让Claude创建一个技能，它将生成具有适当frontmatter和正文内容的正确结构的SKILL.md内容。
   </Tip>

4. **审查简洁性**：检查Claude A是否添加了不必要的解释。询问："删除关于胜率含义的解释 - Claude已经知道。"

5. **改进信息架构**：让Claude A更有效地组织内容。例如："组织这个，使表模式在单独的参考文件中。我们以后可能会添加更多表。"

6. **在类似任务上测试**：使用加载了技能的Claude B（一个新实例）在相关用例上使用该技能。观察Claude B是否找到正确的信息、正确应用规则并成功处理任务。

7. **基于观察迭代**：如果Claude B遇到困难或遗漏了某些内容，请返回Claude A并提供具体信息："当Claude使用此技能时，它忘记按日期过滤第四季度的数据。我们应该添加一个关于日期过滤模式的部分吗？"

**改进现有技能：**

当改进技能时，同样的分层模式继续。您在以下之间交替：

* **与Claude A合作**（帮助改进技能的专家）
* **与Claude B测试**（使用技能执行实际工作的代理）
* **观察Claude B的行为**并将见解带回Claude A

1. **在实际工作流中使用技能**：给加载了技能的Claude B实际任务，而不是测试场景

2. **观察Claude B的行为**：注意它在哪里遇到困难、成功或做出意外选择

   **示例观察**："当我让Claude B提供区域销售报告时，它写了查询但忘记过滤测试账户，尽管技能提到了这个规则。"

3. **返回Claude A进行改进**：分享当前的SKILL.md并描述您观察到的内容。询问："我注意到Claude B在我要求区域报告时忘记过滤测试账户。技能提到了过滤，但可能不够突出？"

4. **审查Claude A的建议**：Claude A可能建议重新组织以使规则更突出，使用更强的语言如"MUST filter"而不是"always filter"，或重构工作流部分。

5. **应用并测试更改**：用Claude A的改进更新技能，然后在类似请求上再次用Claude B测试

6. **基于使用情况重复**：随着您遇到新场景，继续此观察-改进-测试循环。每次迭代都基于实际代理行为而非假设来改进技能。

**收集团队反馈：**

1. 与队友分享技能并观察他们的使用情况
2. 询问：技能是否按预期激活？指令是否清晰？缺少什么？
3. 纳入反馈以解决您自己使用模式中的盲点

**为什么这种方法有效**：Claude A理解代理需求，您提供领域专业知识，Claude B通过实际使用揭示差距，迭代改进基于观察到的行为而非假设的技能。

### 观察Claude如何导航技能

在迭代技能时，注意Claude在实践中如何实际使用它们。注意：

* **意外的探索路径**：Claude是否以您未预期的顺序读取文件？这可能表明您的结构不如您想象的那么直观
* **遗漏的连接**：Claude是否未能遵循指向重要文件的引用？您的链接可能需要更明确或突出
* **过度依赖某些部分**：如果Claude反复读取同一文件，考虑该内容是否应该在主SKILL.md中
* **忽略的内容**：如果Claude从未访问捆绑文件，它可能是不必要的或在主要指令中信号不佳

基于这些观察而非假设进行迭代。技能元数据中的"name"和"description"尤其关键。Claude在决定是否响应当前任务触发技能时使用这些。确保它们清楚地描述技能的作用和使用时机。

## 要避免的反模式

### 避免Windows风格路径

始终在文件路径中使用正斜杠，即使在Windows上：

* ✓ **Good**: `scripts/helper.py`, `reference/guide.md`
* ✗ **Avoid**: `scripts\helper.py`, `reference\guide.md`

Unix风格路径在所有平台上都有效，而Windows风格路径在Unix系统上会导致错误。

### 避免提供太多选项

除非必要，否则不要呈现多种方法：

```markdown
**Bad example: Too many choices** (confusing):
"You can use pypdf, or pdfplumber, or PyMuPDF, or pdf2image, or..."

**Good example: Provide a default** (with escape hatch):
"Use pdfplumber for text extraction:
```python
import pdfplumber
```

For scanned PDFs requiring OCR, use pdf2image with pytesseract instead."
```

## 高级：带可执行代码的技能

下面的部分重点介绍包含可执行脚本的技能。如果您的技能只使用markdown指令，请跳至[有效技能的检查清单](#checklist-for-effective-skills)。

### 解决问题，不要推给Claude

为技能编写脚本时，处理错误条件而不是推给Claude。

**好示例：显式处理错误**：

```python
def process_file(path):
    """Process a file, creating it if it doesn't exist."""
    try:
        with open(path) as f:
            return f.read()
    except FileNotFoundError:
        # Create file with default content instead of failing
        print(f"File {path} not found, creating default")
        with open(path, 'w') as f:
            f.write('')
        return ''
    except PermissionError:
        # Provide alternative instead of failing
        print(f"Cannot access {path}, using default")
        return ''
```

**坏示例：推给Claude**：

```python
def process_file(path):
    # Just fail and let Claude figure it out
    return open(path).read()
```

配置参数也应该有理由并记录，以避免"魔法常量"（Ousterhout定律）。如果您不知道正确的值，Claude如何确定它？

**好示例：自文档化**：

```python
# HTTP requests typically complete within 30 seconds
# Longer timeout accounts for slow connections
REQUEST_TIMEOUT = 30

# Three retries balances reliability vs speed
# Most intermittent failures resolve by the second retry
MAX_RETRIES = 3
```

**坏示例：魔法数字**：

```python
TIMEOUT = 47  # Why 47?
RETRIES = 5   # Why 5?
```

### 提供实用脚本

即使Claude可以编写脚本，预制脚本也有优势：

**实用脚本的好处**：

* 比生成的代码更可靠
* 节省token（无需在上下文中包含代码）
* 节省时间（无需代码生成）
* 确保跨使用的一致性

<img src="https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-executable-scripts.png?fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=4bbc45f2c2e0bee9f2f0d5da669bad00" alt="Bundling executable scripts alongside instruction files" data-og-width="2048" width="2048" data-og-height="1154" height="1154" data-path="images/agent-skills-executable-scripts.png" data-optimize="true" data-opv="3" srcset="https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-executable-scripts.png?w=280&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=9a04e6535a8467bfeea492e517de389f 280w, https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-executable-scripts.png?w=560&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=e49333ad90141af17c0d7651cca7216b 560w, https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-executable-scripts.png?w=840&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=954265a5df52223d6572b6214168c428 840w, https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-executable-scripts.png?w=1100&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=2ff7a2d8f2a83ee8af132b29f10150fd 1100w, https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-executable-scripts.png?w=1650&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=0301a6c8b3ee879497cc5b5483177c90 1650w, https://mintcdn.com/anthropic-claude-docs/4Bny2bjzuGBK7o00/images/agent-skills-executable-scripts.png?w=2500&fit=max&auto=format&n=4Bny2bjzuGBK7o00&q=85&s=0301a6c8b3ee879497cc5b5483177c90 2500w" />

上图显示可执行脚本如何与指令文件一起工作。指令文件（forms.md）引用脚本，Claude可以执行它而无需将其内容加载到上下文中。

**重要区别**：在您的指令中明确说明Claude应该：

* **执行脚本**（最常见）："Run `analyze_form.py` to extract fields"
* **作为参考读取**（用于复杂逻辑）："See `analyze_form.py` for the field extraction algorithm"

对于大多数实用脚本，执行是首选的，因为它更可靠和高效。有关脚本执行如何工作的详细信息，请参阅下面的[运行时环境](#runtime-environment)部分。

**示例**：

```markdown
## Utility scripts

**analyze_form.py**: Extract all form fields from PDF

```bash
python scripts/analyze_form.py input.pdf > fields.json
```

Output format:
```json
{
  "field_name": {"type": "text", "x": 100, "y": 200},
  "signature": {"type": "sig", "x": 150, "y": 500}
}
```

**validate_boxes.py**: Check for overlapping bounding boxes

```bash
python scripts/validate_boxes.py fields.json
# Returns: "OK" or lists conflicts
```

**fill_form.py**: Apply field values to PDF

```bash
python scripts/fill_form.py input.pdf fields.json output.pdf
```
```

### 使用视觉分析

当输入可以渲染为图像时，让Claude分析它们：

```markdown
## Form layout analysis

1. Convert PDF to images:
   ```bash
   python scripts/pdf_to_images.py form.pdf
   ```

2. Analyze each page image to identify form fields
3. Claude can see field locations and types visually
```

<Note>
  In this example, you'd need to write the `pdf_to_images.py` script.
</Note>

Claude的视觉能力有助于理解布局和结构。

### 创建可验证的中间输出

当Claude执行复杂、开放式任务时，它可能会犯错。"计划-验证-执行"模式通过让Claude首先以结构化格式创建计划，然后在执行前用脚本验证该计划，来及早捕获错误。

**示例**：想象让Claude根据电子表格更新PDF中的50个表单字段。如果没有验证，Claude可能引用不存在的字段、创建冲突值、遗漏必填字段或错误应用更新。

**解决方案**：使用上面显示的工作流模式（PDF表单填写），但在应用更改之前添加中间的`changes.json`文件进行验证。工作流变为：分析→**创建计划文件**→**验证计划**→执行→验证。

**为什么此模式有效：**

* **及早捕获错误**：验证在应用更改之前发现问题
* **机器可验证**：脚本提供客观验证
* **可逆规划**：Claude可以在不接触原始文件的情况下迭代计划
* **清晰调试**：错误消息指向特定问题

**何时使用**：批量操作、破坏性更改、复杂验证规则、高风险操作。

**实现提示**：使验证脚本详细，包含特定错误消息，如"Field 'signature\_date' not found. Available fields: customer\_name, order\_total, signature\_date\_signed"，以帮助Claude修复问题。

### 包依赖

技能在代码执行环境中运行，具有特定于平台的限制：

* **claude.ai**：可以从npm和PyPI安装包，并从GitHub仓库拉取
* **Anthropic API**：没有网络访问，无法运行时安装包

在SKILL.md中列出所需包，并在[代码执行工具文档](/en/docs/agents-and-tools/tool-use/code-execution-tool)中验证它们是否可用。

### 运行时环境

技能在具有文件系统访问、bash命令和代码执行能力的代码执行环境中运行。有关此架构的概念性解释，请参阅概述中的[技能架构](/en/docs/agents-and-tools/agent-skills/overview#the-skills-architecture)。

**这如何影响您的编写：**

**Claude如何访问技能：**

1. **元数据预加载**：启动时，所有技能的YAML frontmatter中的名称和描述被加载到系统提示中
2. **文件按需读取**：Claude在需要时使用bash Read工具从文件系统访问SKILL.md和其他文件
3. **脚本高效执行**：实用脚本可以通过bash执行，而无需将其完整内容加载到上下文中。只有脚本的输出消耗token
4. **大文件无上下文惩罚**：参考文件、数据或文档在实际读取之前不消耗上下文token

* **文件路径很重要**：Claude像文件系统一样导航您的技能目录。使用正斜杠（`reference/guide.md`），不要使用反斜杠
* **描述性命名文件**：使用指示内容的名称：`form_validation_rules.md`，而不是`doc2.md`
* **组织发现**：按领域或功能结构目录
  * Good: `reference/finance.md`, `reference/sales.md`
  * Bad: `docs/file1.md`, `docs/file2.md`
* **捆绑综合资源**：包含完整的API文档、大量示例、大型数据集；访问前无上下文惩罚
* **确定性操作首选脚本**：编写`validate_form.py`而不是让Claude生成验证代码
* **明确执行意图**：
  * "Run `analyze_form.py` to extract fields" (execute)
  * "See `analyze_form.py` for the extraction algorithm" (read as reference)
* **测试文件访问模式**：通过实际请求测试验证Claude可以导航您的目录结构

**示例：**

```
bigquery-skill/
├── SKILL.md (overview, points to reference files)
└── reference/
    ├── finance.md (revenue metrics)
    ├── sales.md (pipeline data)
    └── product.md (usage analytics)
```

当用户询问收入时，Claude读取SKILL.md，看到对`reference/finance.md`的引用，并调用bash仅读取该文件。sales.md和product.md文件保留在文件系统上，在需要之前消耗零上下文token。这种基于文件系统的模型是实现渐进式披露的原因。Claude可以导航并选择性地加载每个任务恰好需要的内容。

有关技术架构的完整详细信息，请参阅技能概述中的[技能如何工作](/en/docs/agents-and-tools/agent-skills/overview#how-skills-work)。

### MCP工具引用

如果您的技能使用MCP（模型上下文协议）工具，请始终使用完全限定的工具名称，以避免"工具未找到"错误。

**格式**：`ServerName:tool_name`

**示例**：

```markdown
Use the BigQuery:bigquery_schema tool to retrieve table schemas.
Use the GitHub:create_issue tool to create issues.
```

其中：

* `BigQuery` 和 `GitHub` 是MCP服务器名称
* `bigquery_schema` 和 `create_issue` 是这些服务器中的工具名称

没有服务器前缀，Claude可能无法找到工具，尤其是当有多个MCP服务器可用时。

### 避免假设安装了工具

不要假设包可用：

```markdown
**Bad example: Assumes installation**:
"Use the pdf library to process the file."

**Good example: Explicit about dependencies**:
"Install required package: `pip install pypdf`

Then use it:
```python
from pypdf import PdfReader
reader = PdfReader("file.pdf")
```"
```

## 技术说明

### YAML frontmatter要求

SKILL.md frontmatter需要`name`（最多64个字符）和`description`（最多1024个字符）字段。有关完整的结构详细信息，请参阅[技能概述](/en/docs/agents-and-tools/agent-skills/overview#skill-structure)。

### Token预算

将SKILL.md正文保持在500行以下以获得最佳性能。如果您的内容超过此限制，请使用前面描述的渐进式披露模式将其拆分为单独的文件。有关架构详细信息，请参阅[技能概述](/en/docs/agents-and-tools/agent-skills/overview#how-skills-work)。

## 有效技能的检查清单

在共享技能之前，验证：

### 核心质量

* [ ] 描述具体并包含关键术语
* [ ] 描述包括技能的作用和使用时机
* [ ] SKILL.md正文少于500行
* [ ] 额外细节在单独文件中（如果需要）
* [ ] 没有时间敏感信息（或在"旧模式"部分）
* [ ] 整个技能使用一致的术语
* [ ] 示例具体，不抽象
* [ ] 文件引用只有一层深
* [ ] 适当使用渐进式披露
* [ ] 工作流步骤清晰

### 代码和脚本

* [ ] 脚本解决问题而不是推给Claude
* [ ] 错误处理明确且有帮助
* [ ] 没有"魔法常量"（所有值都有理由）
* [ ] 所需包列在指令中并验证可用
* [ ] 脚本有清晰的文档
* [ ] 没有Windows风格路径（所有正斜杠）
* [ ] 关键操作有验证/验证步骤
* [ ] 质量关键任务包含反馈循环

### 测试

* [ ] 至少创建三个评估
* [ ] 使用Haiku、Sonnet和Opus测试
* [ ] 使用实际使用场景测试
* [ ] 纳入团队反馈（如适用）

## 下一步

<CardGroup cols={2}>
  <Card title="Get started with Agent Skills" icon="rocket" href="/en/docs/agents-and-tools/agent-skills/quickstart">
    Create your first Skill
  </Card>

  <Card title="Use Skills in Claude Code" icon="terminal" href="/en/docs/claude-code/skills">
    Create and manage Skills in Claude Code
  </Card>

  <Card title="Use Skills with the API" icon="code" href="/en/api/skills-guide">
    Upload and use Skills programmatically
  </Card>
</CardGroup>