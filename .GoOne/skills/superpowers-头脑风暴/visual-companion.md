# 可视化辅助指南

基于浏览器的可视化头脑风暴辅助工具，用于展示原型、图表和选项。

## 何时使用

每个问题单独决定，而非整个会话。测试标准：**用户通过查看比阅读能更好地理解吗？**

**使用浏览器**当内容本身是视觉性的：

- **UI原型** — 线框图、布局、导航结构、组件设计
- **架构图** — 系统组件、数据流、关系图
- **并排视觉比较** — 比较两个布局、两种配色方案、两种设计方向
- **设计润色** — 当问题涉及外观和感觉、间距、视觉层次时
- **空间关系** — 状态机、流程图、实体关系渲染为图表

**使用终端**当内容是文本或表格形式：

- **需求和范围问题** — "X是什么意思？"、"哪些功能在范围内？"
- **概念性A/B/C选择** — 在文字描述的方法之间选择
- **权衡列表** — 优缺点、比较表
- **技术决策** — API设计、数据建模、架构方法选择
- **澄清问题** — 任何答案是文字而非视觉偏好的问题

关于UI主题的问题并不自动成为视觉问题。"你想要什么样的向导？"是概念性的——使用终端。"这些向导布局中哪个感觉更好？"是视觉性的——使用浏览器。

## 工作原理

服务器监视目录中的HTML文件，并将最新文件提供给浏览器。您将HTML内容写入`screen_dir`，用户在浏览器中看到它，可以点击选择选项。选择记录到`state_dir/events`，您在下一轮读取。

**内容片段与完整文档：** 如果您的HTML文件以`<!DOCTYPE`或`<html`开头，服务器按原样提供（仅注入辅助脚本）。否则，服务器自动将您的内容包装在框架模板中——添加页眉、CSS主题、选择指示器和所有交互基础设施。**默认情况下编写内容片段**。仅当需要完全控制页面时才编写完整文档。

## 启动会话

```bash
# 启动持久化服务器（原型保存到项目）
scripts/start-server.sh --project-dir /path/to/project

# 返回: {"type":"server-started","port":52341,"url":"http://localhost:52341",
#           "screen_dir":"/path/to/project/.superpowers/brainstorm/12345-1706000000/content",
#           "state_dir":"/path/to/project/.superpowers/brainstorm/12345-1706000000/state"}
```

保存响应中的`screen_dir`和`state_dir`。告诉用户打开URL。

**查找连接信息：** 服务器将其启动JSON写入`$STATE_DIR/server-info`。如果您在后台启动服务器且未捕获stdout，请读取该文件以获取URL和端口。使用`--project-dir`时，检查`<project>/.superpowers/brainstorm/`获取会话目录。

**注意：** 将项目根目录作为`--project-dir`传递，以便原型持久化在`.superpowers/brainstorm/`中，并在服务器重启后保留。如果不传递，文件会进入`/tmp`并被清理。提醒用户如果`.superpowers/`尚未在`.gitignore`中，请添加它。

**按平台启动服务器：**

**Claude Code (macOS / Linux)：**
```bash
# 默认模式工作 - 脚本本身在后台运行服务器
scripts/start-server.sh --project-dir /path/to/project
```

**Claude Code (Windows)：**
```bash
# Windows自动检测并使用前台模式，这会阻塞工具调用。
# 在Bash工具调用上使用run_in_background: true，使服务器在会话轮次间存活
scripts/start-server.sh --project-dir /path/to/project
```
通过Bash工具调用时，设置`run_in_background: true`。然后在下一轮读取`$STATE_DIR/server-info`以获取URL和端口。

**Codex：**
```bash
# Codex会收割后台进程。脚本自动检测CODEX_CI并切换到前台模式。正常运行 - 不需要额外标志。
scripts/start-server.sh --project-dir /path/to/project
```

**Gemini CLI：**
```bash
# 使用--foreground并在shell工具调用上设置is_background: true
# 使进程在轮次间存活
scripts/start-server.sh --project-dir /path/to/project --foreground
```

**其他环境：** 服务器必须在会话轮次间保持在后台运行。如果您的环境会收割分离的进程，请使用`--foreground`并使用平台的后台执行机制启动命令。

如果URL从浏览器无法访问（在远程/容器化设置中常见），绑定非回环主机：

```bash
scripts/start-server.sh \
  --project-dir /path/to/project \
  --host 0.0.0.0 \
  --url-host localhost
```

使用`--url-host`控制返回的URL JSON中打印的主机名。

## 循环流程

1. **检查服务器是否存活**，然后**写入HTML**到`screen_dir`中的新文件：
   - 每次写入前，检查`$STATE_DIR/server-info`是否存在。如果不存在（或`$STATE_DIR/server-stopped`存在），服务器已关闭——继续前用`start-server.sh`重启它。服务器在30分钟无活动后自动退出。
   - 使用语义文件名：`platform.html`、`visual-style.html`、`layout.html`
   - **不要重复使用文件名**——每个屏幕获取新文件
   - 使用Write工具——**不要使用cat/heredoc**（会在终端中产生噪音）
   - 服务器自动提供最新文件

2. **告诉用户预期内容并结束回合：**
   - 提醒他们URL（每一步，不只是第一步）
   - 简要总结屏幕上的内容（例如，"显示主页的3个布局选项"）
   - 让他们在终端中响应："看一看，告诉我你的想法。如果愿意，可以点击选择一个选项。"

3. **在下一回合**——用户在终端中响应后：
   - 如果存在，读取`$STATE_DIR/events`——这包含用户的浏览器交互（点击、选择）作为JSON行
   - 将其与用户的终端文本合并以获得完整画面
   - 终端消息是主要反馈；`state_dir/events`提供结构化交互数据

4. **迭代或推进**——如果反馈更改当前屏幕，写入新文件（例如`layout-v2.html`）。只有当前步骤验证后才移动到下一个问题。

5. **返回终端时卸载**——当下一个步骤不需要浏览器时（例如，澄清问题、权衡讨论），推送等待屏幕以清除过时内容：

   ```html
   <!-- 文件名: waiting.html (或 waiting-2.html 等) -->
   <div style="display:flex;align-items:center;justify-content:center;min-height:60vh">
     <p class="subtitle">Continuing in terminal...</p>
   </div>
   ```

   这防止用户在对话继续时盯着已解决的选择。当下一个视觉问题出现时，按常规推送新内容文件。

6. 重复直到完成。

## 编写内容片段

只编写放入页面的内容。服务器自动将其包装在框架模板中（页眉、主题CSS、选择指示器和所有交互基础设施）。

**最小示例：**

```html
<h2>Which layout works better?</h2>
<p class="subtitle">Consider readability and visual hierarchy</p>

<div class="options">
  <div class="option" data-choice="a" onclick="toggleSelect(this)">
    <div class="letter">A</div>
    <div class="content">
      <h3>Single Column</h3>
      <p>Clean, focused reading experience</p>
    </div>
  </div>
  <div class="option" data-choice="b" onclick="toggleSelect(this)">
    <div class="letter">B</div>
    <div class="content">
      <h3>Two Column</h3>
      <p>Sidebar navigation with main content</p>
    </div>
  </div>
</div>
```

就这样。不需要`<html>`、CSS或`<script>`标签。服务器提供所有这些。

## 可用的CSS类

框架模板为您的内容提供这些CSS类：

### Options (A/B/C choices)

```html
<div class="options">
  <div class="option" data-choice="a" onclick="toggleSelect(this)">
    <div class="letter">A</div>
    <div class="content">
      <h3>Title</h3>
      <p>Description</p>
    </div>
  </div>
</div>
```

**多选：** 向容器添加`data-multiselect`以允许用户选择多个选项。每次点击切换项目。指示器栏显示计数。

```html
<div class="options" data-multiselect>
  <!-- 相同的选项标记 - 用户可以选择/取消选择多个 -->
</div>
```

### Cards (visual designs)

```html
<div class="cards">
  <div class="card" data-choice="design1" onclick="toggleSelect(this)">
    <div class="card-image"><!-- mockup content --></div>
    <div class="card-body">
      <h3>Name</h3>
      <p>Description</p>
    </div>
  </div>
</div>
```

### Mockup container

```html
<div class="mockup">
  <div class="mockup-header">Preview: Dashboard Layout</div>
  <div class="mockup-body"><!-- your mockup HTML --></div>
</div>
```

### Split view (side-by-side)

```html
<div class="split">
  <div class="mockup"><!-- left --></div>
  <div class="mockup"><!-- right --></div>
</div>
```

### Pros/Cons

```html
<div class="pros-cons">
  <div class="pros"><h4>Pros</h4><ul><li>Benefit</li></ul></div>
  <div class="cons"><h4>Cons</h4><ul><li>Drawback</li></ul></div>
</div>
```

### Mock elements (wireframe building blocks)

```html
<div class="mock-nav">Logo | Home | About | Contact</div>
<div style="display: flex;">
  <div class="mock-sidebar">Navigation</div>
  <div class="mock-content">Main content area</div>
</div>
<button class="mock-button">Action Button</button>
<input class="mock-input" placeholder="Input field">
<div class="placeholder">Placeholder area</div>
```

### Typography and sections

- `h2` — 页面标题
- `h3` — 章节标题
- `.subtitle` — 标题下方的次要文本
- `.section` — 带底部边距的内容块
- `.label` — 小写大写标签文本

## 浏览器事件格式

用户在浏览器中点击选项时，他们的交互记录到`$STATE_DIR/events`（每行一个JSON对象）。当您推送新屏幕时，文件自动清除。

```jsonl
{"type":"click","choice":"a","text":"Option A - Simple Layout","timestamp":1706000101}
{"type":"click","choice":"c","text":"Option C - Complex Grid","timestamp":1706000108}
{"type":"click","choice":"b","text":"Option B - Hybrid","timestamp":1706000115}
```

完整的事件流显示用户的探索路径——他们可能在确定前点击多个选项。最后的`choice`事件通常是最终选择，但点击模式可以揭示犹豫或值得询问的偏好。

如果`$STATE_DIR/events`不存在，用户没有与浏览器交互——仅使用他们的终端文本。

## 设计提示

- **根据问题调整保真度** — 布局使用线框图，润色问题使用精修版
- **在每页上解释问题** — "哪个布局感觉更专业？"而不只是"选一个"
- **推进前迭代** — 如果反馈更改当前屏幕，编写新版本
- **每屏最多2-4个选项**
- **重要时使用真实内容** — 对于摄影作品集，使用实际图像（Unsplash）。占位内容掩盖设计问题。
- **保持原型简单** — 专注于布局和结构，而非像素完美设计

## 文件命名

- 使用语义名称：`platform.html`、`visual-style.html`、`layout.html`
- 不要重复使用文件名——每个屏幕必须是新文件
- 对于迭代：附加版本后缀，如`layout-v2.html`、`layout-v3.html`
- 服务器按修改时间提供最新文件

## 清理

```bash
scripts/stop-server.sh $SESSION_DIR
```

如果会话使用了`--project-dir`，原型文件会持久化在`.superpowers/brainstorm/`中供以后参考。只有`/tmp`会话会在停止时删除。

## 参考

- 框架模板（CSS参考）：`scripts/frame-template.html`
- 辅助脚本（客户端）：`scripts/helper.js`