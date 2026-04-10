# GoOne 相对 due 框架的对比与改进规划

本目录存放 **GoOne**（本仓库微服务游戏后端）与开源框架 **[due](https://github.com/dobyte/due)**（Go 分布式游戏服务端框架，模块设计借鉴 Kratos）的对照分析，以及可落地的改进计划与执行方案。

## 文档索引

| 文档 | 内容 |
|------|------|
| [01-architecture-due-vs-goone.md](./01-architecture-due-vs-goone.md) | 架构范式、Gate/Node 映射、事务与 Actor、协议与路由 |
| [02-module-improvements.md](./02-module-improvements.md) | 容器化启动、脚手架、配置与插件边界 |
| [03-platform-capabilities.md](./03-platform-capabilities.md) | 网关、传输、注册发现、事件总线、定位与锁 |
| [04-operations-quality.md](./04-operations-quality.md) | 平滑演进、可观测性、测试策略、发布与回滚 |
| [05-roadmap-prioritized.md](./05-roadmap-prioritized.md) | 分优先级路线图（精简表，含历史 P2 项） |
| [06-execution-plan.md](./06-execution-plan.md) | 任务拆解、责任人建议、验收标准 |
| [execution-plan-detailed.md](./execution-plan-detailed.md) | **按 P0 / P1 / P3 的详细执行计划（推荐主读）** |

## 对比结论摘要

- **GoOne 优势**：业务已按 `connsvr` / `mainsvr` / `roomcentersvr` 等落地；`lib/service/bootstrap` 生命周期清晰；`ssrpc` + 生成代码约束协议；分片 `TransactionMgr` 适合强一致游戏逻辑序列化。
- **due 可借鉴点**：统一的 `Container` 与组件生态心智；Gate/Node/Mesh 文档化角色；原生调试客户端与 `due-cli` 工程效率；多后端事件总线与定位器插件化；平滑重启与后台管理接口的产品化封装。
- **改进方向**：在**不推翻现有架构**前提下，补齐「开发体验、运维闭环、可选 Actor/隔离域、协议与路由文档化」等短板，分阶段吸收 due 的工程化优点。

## 使用说明

- 实施时请以 **仓库代码与 `AGENTS.md`** 为准；本规划为方向性方案，需经评审后纳入迭代。
- 协议与 `api/gen/**` 变更须走 proto 与生成流程，不得手改生成物。
