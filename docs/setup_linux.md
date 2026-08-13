# GoOne Linux 安装与运行指南

本文以当前仓库内的 `main.sh` 与 `readme.md` 为准，适用于：

- Linux 本地开发机
- Linux 部署控制机

## 1. 当前推荐方式

当前推荐工作流是：

- 用 `main.sh` 作为统一入口
- 用 `docker compose` 或 `main.sh docker ...` 管理依赖环境
- 用 `main.sh go ...` 管理 Go 版本
- 用 `main.sh build` 编译
- 用 `-svr_conf` 指向统一 YAML 配置运行服务

如果只是本地开发，通常不需要手工安装 RabbitMQ / ZooKeeper / Redis 到宿主机；直接使用 `env/env_docker.yaml` 即可。

## 2. 前置依赖

建议先准备以下工具：

- `bash`
- `git`
- `docker`
- `docker compose`
- `curl` 或 `wget`
- `tar` / `gzip`

如果你还需要执行部署、远程 Docker 环境管理或主机初始化，再额外准备：

- `python3`
- `python3-venv`

快速检查：

```bash
bash --version
git --version
docker --version
docker compose version
python3 --version
```

## 3. 进入项目并自检

```bash
cd /path/to/GoOne
./main.sh doctor
./main.sh help
```

`doctor` 会检查：

- `build.sh`
- `deploy/*`
- `env/*`
- `go-manager`
- `ansible-playbook` 是否存在

如果你只是本地开发，`ansible-playbook` 不存在不一定是问题。

## 4. 安装或切换 Go 版本

项目当前以 `go.mod` 为准，要求 Go `1.25.4`。

如果你已经安装了匹配版本的 Go，可以跳过这一步；否则推荐直接使用项目自带的 Go 管理脚本：

```bash
./main.sh go list
./main.sh go install 1.25.4
./main.sh go use 1.25.4
./main.sh go current
go version
```

说明：

- `main.sh go ...` 实际委托给 `env/go-manager.sh`
- `main.sh help` 里的示例版本号可能不是最新值，实际请以 `go.mod` 为准

## 5. 启动依赖环境

### 5.1 本地开发推荐：直接使用 Docker Compose

```bash
docker compose -f env/env_docker.yaml up -d
docker compose -f env/env_docker.yaml ps
```

这会拉起当前项目本地联调需要的基础依赖环境。默认文档与脚本语义下，核心依赖包括：

- MySQL
- Redis
- ZooKeeper
- RabbitMQ

停止环境：

```bash
docker compose -f env/env_docker.yaml down
```

### 5.2 远程或统一管理：通过 `main.sh docker`

如果你是在部署控制机上为远端机器准备依赖环境，可以使用：

```bash
./main.sh install ansible
./main.sh docker install --env dev_local
./main.sh docker status --env dev_local
./main.sh docker logs --env dev_local
```

补充说明：

- `--env dev1/dev2/dev_local` 对应 Ansible inventory group
- 相关说明见 `env/README.md`
- 相关 inventory 通常在 `deploy/hosts/host_dev.txt`

## 6. 编译项目

推荐使用主入口：

```bash
./main.sh build
./main.sh build web
```

说明：

- `./main.sh build` 默认编译一组核心服务
- `./main.sh build web` 编译 `websvr`
- 编译产物输出到 `build/`

虽然仓库中仍然存在 `build.sh`，但当前推荐入口仍然是 `main.sh`。

## 7. 本地运行服务

各服务通过统一 flag `-svr_conf` 读取配置。当前本地调试推荐配置文件：

- `env/server_conf_ide.yaml`

典型运行方式：

```bash
./build/connsvr  -svr_conf=./env/server_conf_ide.yaml
./build/mainsvr  -svr_conf=./env/server_conf_ide.yaml
./build/infosvr  -svr_conf=./env/server_conf_ide.yaml
./build/websvr   -svr_conf=./env/server_conf_ide.yaml
```

建议顺序：

1. 先确认依赖环境已经启动
2. 再启动 `mysqlsvr` / `infosvr` / `mainsvr` / `connsvr`
3. 有 Web 管理需求时再启动 `websvr`

## 8. 部署与控制机操作

如果你要把 Linux 机器作为部署控制机使用，推荐仍通过 `main.sh` 调度：

### 8.1 安装 Ansible

```bash
./main.sh install ansible
```

### 8.2 查看环境与角色

```bash
./main.sh env list
./main.sh role list
```

### 8.3 部署或重启服务

```bash
./main.sh deploy --env dev1 --action restart --role websvr
./main.sh deploy --env dev1 --action restart --roles websvr,mainsvr --dry-run
```

### 8.4 初始化目标主机

```bash
./main.sh host init
./main.sh host init --env dev1
./main.sh host init --limit 192.168.50.250
```

更详细的部署说明请参考：

- `deploy/README.md`

## 9. 配置表与生成工具

当前仓库不再推荐旧的手工导表脚本或历史导表目录。

如果需要导表、生成配置数据、生成配置仓库代码，请使用：

- `tools/cfgtool`

入口文件：

- `tools/cfgtool/main.go`

## 10. 当前文档与历史流程的区别

请注意，以下历史做法不再是当前推荐路径：

- 手工安装 Go `1.13`
- 手工安装 RabbitMQ / ZooKeeper / Redis 到宿主机再联调
- 直接依赖旧的导表脚本
- 把 `build.sh`、`deploy.sh` 当成首选入口

当前推荐路径是：

- `main.sh`
- `env/env_docker.yaml`
- `env/server_conf_ide.yaml`
- `tools/cfgtool`

## 11. 常见问题

### 11.1 `doctor` 提示没有 `ansible-playbook`

如果你只是本地开发，可以先忽略；只有在你要使用：

- `./main.sh install ansible`
- `./main.sh docker ...`
- `./main.sh deploy ...`
- `./main.sh host init ...`

这些功能时才需要补装。

### 11.2 本地开发一定要用 Ansible 吗

不需要。当前本地开发最简单的方式是：

```bash
docker compose -f env/env_docker.yaml up -d
./main.sh build
./build/connsvr -svr_conf=./env/server_conf_ide.yaml
```

### 11.3 为什么命令里统一使用 `-svr_conf`

因为各服务都通过统一的 YAML 配置入口启动，示例配置在：

- `env/server_conf_ide.yaml`
- `common/gconf/server_conf.yaml`

### 11.4 如何关闭彩色输出

```bash
NO_COLOR=1 ./main.sh doctor
```
