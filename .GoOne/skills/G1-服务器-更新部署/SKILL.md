---
name: "G1-服务器-更新部署"
description: "在业务代码开发完成并通过测验后，将最新代码构建为服务器程序、启动验证无报错、提交 Git 更新并生成部署报告，或通过 Ansible 将构建产物发布到目标环境。当用户需要进行构建部署、版本提交、服务器更新发布时调用。"
---

# G1-服务器-更新部署

## 概述

本技能在**业务代码开发完成并通过测验**后，执行从构建到部署的全流程操作。涵盖：通过 `./main.sh build`（Windows 用 `./build.ps1`）将各服务编译到 `build/`、本地拉起 6 服务（connsvr / mainsvr / infosvr / mysqlsvr / roomcentersvr / web_svr）做无报错验证、生成符合规范的 Git commit 并提交、通过 Ansible 将构建产物发布到目标环境（`./main.sh deploy`）、输出部署更新报告。

> **定位边界**：
> - ✅ 自动构建各活动服务到 `build/` 目录
> - ✅ 本地拉起 6 服务进行部署前验证（依赖 MySQL / Redis / ZK(etcd) / RabbitMQ）
> - ✅ 自动收集代码变更清单，生成符合 `docs/STYLE.md` 规范的 Git commit message
> - ✅ 执行 Git 提交更新（dev 分支开发，master 分支主干）
> - ✅ 通过 Ansible 发布到目标环境（init / push / start / stop / restart）
> - ✅ 输出部署更新报告
> - ❌ 不涉及业务编码开发（那是"G1-服务器-业务开发"/"G1-服务器-业务迭代开发v2"技能的职责）
> - ❌ 不涉及业务逻辑测验（那是"G1-服务器-代码测验"/"G1-服务器-模拟测试"技能的职责）
> - ❌ 不审查 PRD 逻辑或技术方案设计
> - ❌ 不修改 GoOne 框架层代码（`lib/service/` 等基础设施）
> - ❌ 不涉及目标机的操作系统初始化以外的运维操作（OS 初始化走 `./main.sh host init`）

## 适用场景

- 业务代码开发完成并通过测验后，准备提交版本更新
- 需要构建最新服务器程序并验证启动无异常
- 需要按 `docs/STYLE.md` 规范生成 Git commit message 并提交
- 多人协作时，需要快速验证合并后的代码能否正常构建和启动
- 发布前进行干净的构建验证
- 需要把构建产物发布到 dev / test 等目标环境

---

## 前置条件

本技能依赖业务代码已开发完成并通过测验。开始前必须确认：

1. 业务代码已通过编译（`go build ./...` 无错误）
2. 业务测验已通过（测验报告状态为"已通过"或"部分通过"，无致命问题遗留）
3. 知识库中 `{analysis_output_dir}/{需求名称}/需求开发报告.md`（或 `迭代开发报告v2.md`）已存在
4. 知识库中 `{analysis_output_dir}/{需求名称}/测试样例-{日期}.md` 已存在（如有测验）
5. Git 命令行工具可用（`git --version` 可正常执行）
6. Git 工作区状态正常（无未解决的合并冲突）
7. 改动 proto 或生成器时已运行 `./main.sh check-genproto` 且通过
8. 如果上述条件不满足，提示用户先完成前置步骤

---

## 项目技术要素速查

在进行更新部署前，必须先了解以下项目核心技术要素：

| 要素 | 说明 | 涉及路径/模块 |
|------|------|-------------|
| **构建入口（跨平台）** | `main.sh build` 构建全部活动服务到 `build/` | `<root>/main.sh`、`<root>/build.sh` |
| **构建入口（Windows）** | PowerShell 本地构建 | `<root>/build.ps1` |
| **构建输出** | 各服务二进制输出到 `build/` | `build/connsvr`、`build/mainsvr`、`build/infosvr`、`build/mysqlsvr`、`build/roomcentersvr`、`build/websvr`、`build/tester`、`build/stress` |
| **本地 6 服务** | connsvr / mainsvr / infosvr / mysqlsvr / roomcentersvr / web_svr | `cmd/<svc>svr/`、`src/<svc>svr/` |
| **中间件依赖** | MySQL（持久化）、Redis（缓存）、ZK/etcd（注册发现）、RabbitMQ（消息总线） | `etc/env/env_docker.yaml`（docker-compose 编排） |
| **中间件启动** | 通过 docker-compose 一键拉起 | `./main.sh docker up --env dev` |
| **服务配置** | 单一 `server_conf.yaml`（含 `connsvr.runtime.listen_port` 等） | `etc/config/server_conf_ide.yaml`（IDE 本地）|
| **生成代码门禁** | proto 生成一致性校验 | `./main.sh check-genproto` |
| **知识库配置** | Obsidian Vault 路径、策划案目录、技术输出目录 | `.GoOne/conf.json` |
| **Git 仓库** | 版本管理（master 主干 / dev 开发分支） | 项目根目录为 Git 工作区 |
| **Ansible 部署** | init / push / start / stop / restart | `<root>/deploy/`（`main.sh deploy` 封装）|
| **部署配置（目标机）** | 目标机统一配置路径 | `/data/GoOne/commconf/server_conf.yaml` |

> 注意：GoOne **不使用 MongoDB / NATS**。持久化走 MySQL（经 mysqlsvr + xorm），消息总线走 RabbitMQ，注册发现走 etcd / ZooKeeper。

---

## 工作流程

### 第一步：获取项目配置

调用 `G1-项目配置` 技能获取项目配置信息：

> - `vault_path` = Obsidian Vault 路径（用于 obsidian-cli 的 `vault=` 参数）
> - `prd_dir` = 策划需求文档目录（相对 vault 根目录）
> - `analysis_output_dir` = 技术输出目录（相对 vault 根目录）
>
> **部署输出目录**固定为 `{analysis_output_dir}/{需求名称}/`。

如果 `.GoOne/conf.json` 不存在，`G1-项目配置` 技能会自动引导创建。

---

### 第二步：收集本次变更信息

在构建之前，必须先收集本次变更的完整信息，用于生成提交日志和部署报告。

#### 2.1 获取 Git 变更状态

```bash
cd <project_root>
git status
```

从 `git status` 输出中解析变更文件清单，分类为：

| 状态标记 | 含义 | 分类 |
|---------|------|------|
| `M ` / ` M` | 已修改（Modified，暂存区 / 工作区） | 修改 |
| `A` | 已添加（Added，暂存区） | 新增 |
| `D` | 已删除（Deleted） | 删除 |
| `??` | 未纳入版本控制 | 待添加 |
| `UU` / `AA` / `DD` | 合并冲突 | **阻塞** — 必须先解决冲突 |
| `R` | 重命名 | 重命名 |

#### 2.2 获取 Git 差异摘要

```bash
# 工作区 vs 暂存区
git diff --stat

# 暂存区 vs HEAD（待提交）
git diff --cached --stat
```

#### 2.3 提取变更模块清单

根据变更文件路径，推断涉及的服务和模块：

| 文件路径模式 | 推断模块 |
|------------|---------|
| `common/game_proto/service/*.proto` | 业务通讯协议（service proto） |
| `api/proto/**` | 框架级 proto（cmd / options） |
| `api/gen/**`、`common/protocol/**` | 生成代码（禁止手改，应与 proto 同步提交） |
| `module/gamedata/repository/**` | 配置表生成代码 |
| `src/connsvr/` | 连接服务 |
| `src/mainsvr/` | 主服务 |
| `src/infosvr/` | 信息服务 |
| `src/mysqlsvr/` | MySQL 服务 |
| `src/roomcentersvr/` | 房间中心服务 |
| `src/web_svr/` | Web 服务 |
| `lib/` | 框架层（改动需谨慎评审） |
| `tools/tester/` | 测试 / 压测工具 |

#### 2.4 读取开发报告（如可用）

通过 obsidian-cli 读取知识库中的开发报告以获取更准确的变更描述：

```bash
obsidian vault="{vault_path}" read path="{analysis_output_dir}/{需求名称}/需求开发报告.md"
# 或迭代开发：
obsidian vault="{vault_path}" read path="{analysis_output_dir}/{需求名称}/迭代开发报告v2.md"
```

从报告中提取：
- 需求名称
- 涉及服务清单
- 代码变更清单

---

### 第三步：自动构建服务器程序

运行构建脚本，将最新代码编译为各服务二进制。

#### 3.1 执行构建脚本

**Linux / macOS / Git-Bash / WSL**（推荐，统一入口）：
```bash
cd <project_root>
./main.sh build
# 或只构建单个服务：
./main.sh build main      # mainsvr
./main.sh build conn      # connsvr
./main.sh build web       # web_svr
```

**Windows PowerShell**（本地构建）：
```powershell
.\build.ps1            # 构建全部
.\build.ps1 main       # 构建单个
```

`build.sh` 实际为对各服务逐一执行 `go build -o build/<svc>`，目标对照（见 `build.sh`）：

| target | 源目录 | 输出 |
|--------|--------|------|
| `conn` / `connsvr` | `cmd/connsvr` | `build/connsvr` |
| `main` / `mainsvr` | `cmd/mainsvr` | `build/mainsvr` |
| `info` / `infosvr` | `cmd/infosvr` | `build/infosvr` |
| `mysql` / `mysqlsvr` | `cmd/mysqlsvr` | `build/mysqlsvr` |
| `roomcenter` / `roomcentersvr` | `cmd/roomcentersvr` | `build/roomcentersvr` |
| `web` / `websvr` / `web_svr` | `cmd/web_svr` | `build/websvr` |
| `tester` | `tools/tester/cmd/tester` | `build/tester` |
| `stress` | `tools/tester/cmd/stress` | `build/stress` |

#### 3.2 构建结果验证

构建完成后，验证以下内容：

```
[√] build/connsvr、build/mainsvr 等二进制文件存在且大小 > 0
[√] 构建过程中无编译错误（exit code = 0）
[√] 若改动 proto：./main.sh check-genproto 通过（api/gen 与 common/protocol 与 proto 一致）
```

**如果构建失败**：
1. 从构建输出中定位错误原因（语法错误 / 类型不匹配 / 缺少依赖）
2. 分类错误类型并给出修复建议
3. 修复后重新执行第三步
4. 最多重试 3 次，超过则输出失败报告并中止

#### 3.3 快速构建（跳过全量）

如果只修改了单个服务的业务代码，可只构建该服务：

```bash
./main.sh build main      # 只构建 mainsvr
```

> ⚠️ 若改动涉及 proto 或生成器，必须额外运行 `./main.sh check-genproto` 确认生成代码一致性，再提交。

---

### 第四步：启动服务器验证

构建成功后，拉起本地 6 服务进行无报错验证，确保程序能正常启动和运行。

#### 4.1 启动前：拉起中间件依赖

GoOne 本地依赖 MySQL / Redis / ZK(etcd) / RabbitMQ，统一通过 docker-compose 编排（见 `etc/env/env_docker.yaml`）：

```bash
cd <project_root>
./main.sh docker up --env dev
# 状态检查：
./main.sh docker status --env dev
```

> 首次运行需准备 `etc/env/.env`（凭据注入，git 忽略），模板见 `etc/env/.env.example`。容器端口：MySQL 3306、Redis 6379、ZK 2181、RabbitMQ 5672 / 15672。

#### 4.2 启动前环境检查

```bash
# 1. 中间件已就绪（上一节的 docker status 应为 healthy / running）
# 2. 清理上次残留进程（Windows）
tasklist /FI "IMAGENAME eq connsvr.exe" 2>nul
taskkill /F /IM connsvr.exe 2>nul
taskkill /F /IM mainsvr.exe 2>nul
# Linux/Git-Bash：
pkill -f 'build/connsvr' 2>/dev/null || true
pkill -f 'build/mainsvr' 2>/dev/null || true
```

#### 4.3 拉起 6 服务

按依赖顺序启动：mysqlsvr → infosvr → mainsvr → roomcentersvr → connsvr → web_svr。

```bash
cd <project_root>
# 各服务读取 etc/config/server_conf_ide.yaml（含 connsvr.runtime.listen_port 等运行时配置）
./build/mysqlsvr &
./build/infosvr &
./build/mainsvr &
./build/roomcentersvr &
./build/connsvr &
./build/websvr &
```

> 实际启动方式以 `etc/config/server_conf_ide.yaml` 与各服务 `cmd/<svc>svr/main.go` 为准。WebSocket 端口由配置项 `connsvr.runtime.listen_port` 决定（IDE 默认 11000），并非硬编码。

#### 4.4 启动验证检查清单

启动后等待 3-5 秒（让服务完成初始化 + 服务注册），然后逐项检查：

```
[√] 进程存在且未退出——tasklist/ps 确认 6 个进程均运行中
[√] 无 panic 日志——检查控制台输出无 panic/stack trace
[√] 服务注册成功——日志中包含向 etcd/zk 注册成功信息
[√] 端口监听正常——WebSocket/gRPC 端口（connsvr.runtime.listen_port）已监听
[√] 配置加载无错误——日志中无 config load error
[√] 中间件连通——MySQL/Redis/RabbitMQ 连接无 error
```

**具体验证**：

```bash
# 1. 确认进程启动（Linux/Git-Bash）
ps aux | grep -E 'build/(conn|mains|infos|mysql|roomcenters|webs)vr'

# 2. 检查日志输出（查找 ERROR/PANIC/FATAL）
# 日志中不应出现以下关键字：
#   - panic:
#   - FATAL
#   - nil pointer dereference
#   - index out of range
#   - dial tcp ...: connect: connection refused   # 中间件未起

# 3. 验证 WebSocket 端口监听（以配置项为准，示例用 11000）
netstat -an | grep 11000    # Windows: netstat -an | findstr "11000"
```

#### 4.5 验证失败处理

如果服务器启动验证失败：

| 失败类型 | 表现 | 处理方式 |
|---------|------|---------|
| **编译/链接错误** | 进程无法启动 | 回到第三步检查构建 |
| **启动即崩溃** | 进程启动后立即退出 | 分析 panic 日志，定位崩溃位置 |
| **配置加载失败** | 日志中 YAML 解析错误 | 检查 `etc/config/server_conf_ide.yaml` |
| **端口冲突** | bind: address already in use | 终止占用端口的旧进程后重试 |
| **中间件连接失败** | MySQL/Redis/RabbitMQ/ZK 连接失败 | 检查 `./main.sh docker status --env dev` 是否 healthy |
| **服务注册失败** | etcd/zk 注册 error | 检查注册中心容器与 GOONE_ETCD_ADDR 配置 |

#### 4.6 验证完成后清理

验证通过后，终止测试服务进程：

```bash
# Linux/Git-Bash
pkill -f 'build/(conn|mains|infos|mysql|roomcenters|webs)vr' 2>/dev/null || true

# Windows
taskkill /F /IM connsvr.exe 2>nul
taskkill /F /IM mainsvr.exe 2>nul
taskkill /F /IM infosvr.exe 2>nul
taskkill /F /IM mysqlsvr.exe 2>nul
taskkill /F /IM roomcentersvr.exe 2>nul
taskkill /F /IM websvr.exe 2>nul

# 停中间件（可选，发布到远端时本地可保留）
./main.sh docker down --env dev
```

---

### 第五步：Git 提交更新

服务器构建和启动验证均通过后，执行 Git 提交。

#### 5.1 选择分支与生成 commit message

GoOne 采用 `master`（主干）+ `dev`（开发集成分支）的分支模型。日常开发提交到 `dev`；发布节点由 `master` 合并打标签。

```bash
# 确认当前分支（日常应在 dev）
git branch --show-current
```

提交信息格式要求（**强制**，见 `docs/STYLE.md`）：`<type>: <summary>`

**type 取值**：`feat` / `fix` / `perf` / `refactor` / `test` / `docs` / `chore`

**生成规则**：

1. **提取 type**：根据变更主体判定（新功能 `feat`、修 bug `fix`、性能优化 `perf`、重构 `refactor`、测试 `test`、文档 `docs`、构建/杂项 `chore`）
2. **提取变更模块名**：从第二步收集的变更文件清单中提取
3. **提取需求名称**：从开发报告中提取（如可用）
4. **summary 精简**：一行，优先保留核心业务关键词

**commit message 模板**：

```
<type>: {需求名称/模块} {变更摘要}

- {变更点1}
- {变更点2}
```

**示例**：
```
feat: 签到系统 新增 VIP 加成模块

- mainsvr 新增 signin rpc 与 VIP 加成逻辑
- common/game_proto 新增 signin service proto
- module/gamedata 新增 signin 配置表生成代码
```
```
fix: 道具系统 修复 item_use 并发竞态

- mainsvr 修复道具发放事务一致性
```
```
refactor: 大厅服务 角色创建限制校验
```

#### 5.2 确认待提交文件

在提交前，列出所有待提交文件供用户确认：

```bash
git status
```

对于状态为 `??`（未纳入版本控制）的新文件，询问用户是否需要 `git add`：

```bash
# 添加所有新文件（需用户确认）
git add <file_path>
# 或按模块添加：
git add src/mainsvr/ module/gamedata/ common/
```

> ⚠️ 不要自动添加 `build/`（二进制产物）、`*.exe`、`etc/env/.env`（凭据）等。这些应已在 `.gitignore` 中排除。若误添加，用 `git reset HEAD <file>` 撤出暂存区。

> ⚠️ 若改动涉及 proto / 生成器，**必须**把 `api/gen/**`、`common/protocol/**`、`module/gamedata/repository/**` 的生成改动一并提交（生成代码与 proto 必须同源），且先通过 `./main.sh check-genproto`。

#### 5.3 执行 Git 提交

```bash
cd <project_root>
git add <待提交文件>
git commit -m "<type>: <summary>" -m "<详细说明>"
```

提交后验证：

```bash
# 确认提交成功，查看最新 commit
git log -1 --oneline
git show --stat HEAD
```

#### 5.4 推送与同步（如需）

```bash
# 推送到远端 dev 分支
git push origin dev
```

#### 5.5 提交失败处理

| 失败类型 | 原因 | 处理方式 |
|---------|------|---------|
| **nothing to commit** | 没有暂存的改动 | 检查 `git add` 是否遗漏 |
| **out of date**（push 时） | 远端有新提交 | 先 `git pull --rebase origin dev`，解决冲突后重推 |
| **merge conflict** | 合并/rebase 冲突 | 手动解决冲突，`git add` 后 `git rebase --continue`（或 `git merge --continue`） |
| **authentication** | 认证失败 | 检查 Git 凭证 / SSH key / token，提示用户重新登录 |
| **pre-commit hook 失败** | golangci-lint / check-genproto 拦截 | 按 hook 输出修复（如重新跑 genproto、修 lint 告警）后重新提交 |

> Git 没有 SVN 的 `cleanup`；如工作区状态混乱，可用 `git status` 排查，必要时 `git stash`（暂存未提交改动）或 `git reset`（调整暂存区）。**不要**对已推送的提交做 `reset --hard`。

---

### 第六步：（可选）Ansible 发布到目标环境

代码提交后，若需发布到 dev / test 等目标环境，使用 GoOne 的 Ansible 部署（`./main.sh deploy`，封装 `deploy/deploy.sh`）。

#### 6.1 列出可用环境与角色

```bash
./main.sh env list     # 列出可部署环境（如 dev1）
./main.sh role list    # 列出可部署角色
```

可部署角色（见 `deploy/roles/`）：`connsvr` / `mainsvr` / `infosvr` / `mysqlsvr` / `roomcentersvr` / `websvr` / `commconf`（统一配置）/ `gamedata`（配置表分发），以及 `chatsvr` / `friendsvr` / `mailsvr`（按需）。

#### 6.2 执行部署动作

```bash
# 推送构建产物 + 重启某角色
./main.sh deploy --env dev1 --action push --role mainsvr
./main.sh deploy --env dev1 --action restart --role mainsvr

# 多角色 + dry-run 预演
./main.sh deploy --env dev1 --action restart --roles websvr,mainsvr --dry-run

# 限制主机 + 透传 ansible 参数
./main.sh deploy --env dev1 --action push --limit 43.139.3.228 --role websvr -- -vv

# 首次初始化目标机
./main.sh deploy --env dev1 --action init --role mainsvr
```

`--action` 取值（见 `deploy/deploy.sh`）：`init`（首次初始化）/ `push`（推送产物）/ `start` / `stop` / `restart`。

#### 6.3 目标机配置

部署后，目标机统一配置位于 `/data/GoOne/commconf/server_conf.yaml`（由 `commconf` role 下发，模板见 `deploy/roles/commconf/templates/server_conf.yaml`）。各服务二进制与运行时数据按 role 分别落在 `/data/GoOne/<role>/` 下。

#### 6.4 部署后健康检查

| 检查项 | 方式 |
|--------|------|
| 进程存活 | 目标机 `ps` / `systemctl status` 对应服务 |
| 端口监听 | `netstat -an \| grep <listen_port>` |
| 服务注册 | 注册中心（etcd/zk）中节点已注册 |
| 日志无 error | 查看服务日志，确认无 FATAL/PANIC |

---

### 第七步：输出部署更新报告

构建、验证、提交（及可选发布）全部完成后，输出部署更新报告到知识库。

**输出目录**：`{analysis_output_dir}/{需求名称}/`

**输出命令**：
```bash
obsidian vault="{vault_path}" create name="{analysis_output_dir}/{需求名称}/部署更新报告-{YYYY-MM-DD}" content="{报告内容}"
```

```markdown
# {需求名称} — 部署更新报告

> 更新日期：{YYYY-MM-DD HH:MM}
> 操作人：{自动检测或询问用户}
> 基于开发报告：[[{analysis_output_dir}/{需求名称}/需求开发报告]]
> 部署状态：部署成功 / 部分成功 / 失败

---

## 一、本次更新概要

### 1.1 需求名称
{需求名称}

### 1.2 涉及服务

| 服务 | 改动类型 | 说明 |
|------|---------|------|
| {服务名} | 新增/修改/无 | {说明} |

### 1.3 Git 提交信息

| 项目 | 内容 |
|------|------|
| 分支 | dev |
| commit SHA | {short sha} |
| commit message | {type}: {summary} |
| 提交文件数 | {n} 个文件 |

---

## 二、构建结果

| 检查项 | 状态 | 说明 |
|--------|------|------|
| `./main.sh build` 执行 | ✅ 通过 / ❌ 失败 | exit code: {code} |
| `build/` 二进制生成 | ✅ / ❌ | connsvr/mainsvr/... 大小: {size} |
| check-genproto | ✅ / ❌ / N/A | 改动 proto 时必查 |
| golangci-lint | ✅ / ❌ | --new-from-rev=origin/dev |

---

## 三、启动验证结果

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 中间件（MySQL/Redis/ZK/RabbitMQ） | ✅ 就绪 / ❌ 异常 | `./main.sh docker status` |
| 进程启动（6 服务） | ✅ 正常运行 / ❌ 崩溃 | PID: {pid} |
| panic 检查 | ✅ 无 / ❌ 有 | {如有：崩溃位置和原因} |
| 服务注册 | ✅ 正常 / ❌ 异常 | etcd/zk 注册 |
| 端口监听 | ✅ 正常 / ❌ 异常 | connsvr.runtime.listen_port: {port} |

---

## 四、Git 变更文件清单

| 状态 | 文件路径 | 模块 |
|------|---------|------|
| M | {文件路径} | {模块名} |
| A | {文件路径} | {模块名} |
| D | {文件路径} | {模块名} |

---

## 五、目标环境发布结果（如执行了第六步）

| 环境 | 角色 | 动作 | 结果 | 说明 |
|------|------|------|------|------|
| dev1 | mainsvr | push + restart | ✅ / ❌ | {说明} |

---

## 六、注意事项与后续操作

（如果有需要人工跟进的事项，在此说明）

- 请在测试环境进一步验证 {功能} 的联调表现
- 本次提交仅包含服务端代码，客户端需同步更新协议
- {其他注意事项}
```

---

## 流程闭环图

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  收集变更    │ →  │  自动构建   │ →  │  启动验证   │ →  │  Git 提交   │
│  git status │    │ main.sh     │    │  6 服务      │    │ git commit  │
│             │    │  build      │    │  +中间件     │    │ → push dev  │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
       ↑                  ↓                  ↓                  ↓
   ┌───┴────────── 变更文件清单    构建成功/失败     启动正常/异常       │
   │                                                                ↓
   │                                            (可选) ┌─────────────┐
   └──────────────────── 发现异常 → 修复 → 重新走流程 ←──│ Ansible发布 │
                                                              → 输出报告
```

---

## 关键命令速查

| 命令 | 用途 | 路径/说明 |
|------|------|----------|
| `./main.sh build` | 构建全部活动服务到 `build/` | `<root>/` |
| `./main.sh build <target>` | 构建单个服务 | target ∈ conn/main/info/mysql/roomcenter/web/tester/stress |
| `.\build.ps1` | Windows PowerShell 本地构建 | `<root>/` |
| `./build/tester` / `./build/stress` | 启动集成测试 / 压测 | `<root>/build/` |
| `./main.sh docker up --env dev` | 拉起 MySQL/Redis/ZK/RabbitMQ | `<root>/` |
| `./main.sh docker status --env dev` | 中间件状态检查 | `<root>/` |
| `./main.sh check-genproto` | proto 生成一致性门禁 | `<root>/` |
| `./main.sh env list` / `role list` | 列出可部署环境 / 角色 | `<root>/` |
| `./main.sh deploy --env <e> --action <a> --role <r>` | Ansible 发布（init/push/start/stop/restart） | `<root>/` |
| `git status` | 查看 Git 变更状态 | `<root>/` |
| `git diff --stat` / `git diff --cached --stat` | 查看 Git 变更摘要 | `<root>/` |
| `git add <file>` | 暂存变更 | `<root>/` |
| `git commit -m "<type>: <summary>"` | 提交（type ∈ feat/fix/perf/refactor/test/docs/chore） | `<root>/` |
| `git push origin dev` | 推送到远端 dev 分支 | `<root>/` |
| `git pull --rebase origin dev` | 同步远端并 rebase | `<root>/` |
| `git log -1 --oneline` | 查看最新提交 | `<root>/` |
| `git stash` / `git reset` | 工作区整理（对应 SVN cleanup 场景） | `<root>/` |

---

## 部署原则

1. **先构建后验证**：每次提交前必须先通过完整构建和启动验证，不允许跳过验证直接提交
2. **变更可追溯**：commit message 遵循 `<type>: <summary>` 规范，便于后续回溯与 changelog 生成
3. **构建即检查**：`golangci-lint --new-from-rev=origin/dev` 增量门禁 + `check-genproto` 生成代码门禁，零告警是目标
4. **启动即验证**：6 服务启动后的前 5 秒是关键验证窗口，必须确认无 panic、中间件连通、服务就绪
5. **提交前确认**：Git 提交前应列出所有待提交文件，让用户确认变更范围
6. **commit message 精简有力**：单行 summary 优先保留核心业务关键词，详情放 body
7. **失败即停止**：构建或启动验证任何一步失败，都应停止后续步骤，先排查修复
8. **生成代码同源提交**：proto 改动必须连同 `api/gen/**`、`common/protocol/**`、`module/gamedata/repository/**` 一起提交，且先过 `check-genproto`
9. **环境一致性**：本地验证用 `etc/config/server_conf_ide.yaml` + docker-compose；目标机用 `/data/GoOne/commconf/server_conf.yaml`（Ansible 下发）
10. **报告即资产**：每次部署产生的报告归档到知识库，形成可追溯的版本发布历史

---

## 部署状态码参考

| 状态码 | 含义 | 说明 |
|--------|------|------|
| `BUILD_OK` | 构建成功 | `./main.sh build` exit code 为 0 |
| `BUILD_FAIL` | 构建失败 | 编译错误，需修复代码 |
| `GENPROTO_FAIL` | 生成代码不一致 | 需重新跑 genproto 并提交 |
| `STARTUP_OK` | 启动验证通过 | 6 服务启动无 panic，服务就绪，中间件连通 |
| `STARTUP_FAIL` | 启动验证失败 | 启动崩溃 / 服务未就绪 / 中间件连接失败 |
| `GIT_OK` | Git 提交成功 | 代码已提交到 dev 分支 |
| `GIT_FAIL` | Git 提交失败 | 冲突 / 认证 / pre-commit hook 拦截 |
| `DEPLOY_OK` | 远端发布成功 | Ansible push + restart 全通过 |
| `DEPLOY_PARTIAL` | 部分完成 | 构建通过但提交/发布失败（需人工介入） |
