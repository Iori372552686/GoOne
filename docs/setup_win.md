# GoOne Windows 安装与运行指南

本文以当前仓库内的 `main.sh` 与 `readme.md` 为准，目标是帮助你在 Windows 上用当前推荐方式完成：

- 开发环境准备
- 依赖环境启动
- 编译与本地运行
- 可选的部署控制机准备

## 1. 当前推荐方式

Windows 上当前推荐的工作流是：

- **首选：WSL2 + Ubuntu + Docker Desktop**
- **可选：Git-Bash**

结论先说：

- 如果你要完整使用 `main.sh go`、`main.sh build`、`main.sh docker`、`main.sh deploy`，**优先用 WSL2**
- 如果你只是偶尔跑简单 bash 脚本，Git-Bash 可以作为补充
- 不建议把 PowerShell / CMD 作为 `main.sh` 的主要执行环境

## 2. 推荐环境组合

建议使用以下组合：

- Windows 10 / Windows 11
- WSL2
- Ubuntu（或其他常见 Linux 发行版）
- Docker Desktop
- WSL 集成已打开

这样可以保证：

- `main.sh`、`build.sh`、`deploy/*.sh` 都在 Linux 风格环境中运行
- `docker compose` 可以直接在 WSL 中调用
- 编译出来的服务二进制与 Linux 部署环境一致

## 3. 安装 WSL2

在管理员权限的 PowerShell 中执行：

```powershell
wsl --install
```

执行完成后按提示重启系统，并完成 Ubuntu 的首次初始化。

如果你的系统已经安装过 WSL，可用以下命令确认：

```powershell
wsl -l -v
```

确认目标发行版状态为：

- Version = `2`

## 4. 在 WSL 中准备基础工具

进入 Ubuntu 后，先安装常用依赖：

```bash
sudo apt update
sudo apt install -y bash git curl wget tar gzip ca-certificates build-essential
```

如果你还需要在这台机器上执行部署、主机初始化或远程 Docker 环境管理，再额外安装：

```bash
sudo apt install -y python3 python3-venv python3-pip
```

## 5. 安装 Docker Desktop 并开启 WSL 集成

在 Windows 宿主机安装 Docker Desktop，然后确认：

1. 已启用 `Use the WSL 2 based engine`
2. 已对你的 Ubuntu 发行版开启 WSL Integration

完成后回到 WSL，检查：

```bash
docker --version
docker compose version
```

如果这两个命令在 WSL 中可用，说明当前环境已经满足本地依赖启动条件。

## 6. 在 WSL 中进入项目

假设项目位于 Windows 的 `E:\WorkCode\my\GoOne`，在 WSL 中对应路径通常是：

```bash
cd /mnt/e/WorkCode/my/GoOne
```

然后执行：

```bash
./main.sh doctor
./main.sh help
```

说明：

- `doctor` 会检查脚本与基本工具是否齐全
- 如果你只是本地开发，没有 `ansible-playbook` 也不一定是问题

## 7. Go 版本管理

当前项目以 `go.mod` 为准，要求 Go `1.25.4`。

推荐在 WSL 中直接使用项目自带的 Go 管理入口：

```bash
./main.sh go list
./main.sh go install 1.25.4
./main.sh go use 1.25.4
./main.sh go current
go version
```

说明：

- `main.sh go ...` 底层会调用 `env/go-manager.sh`
- 该工具链是 Bash / Linux 风格优先设计的，所以在 WSL 中使用最稳妥

如果你只安装了原生 Windows Go，也可以在 IDE 里做部分本地调试，但**当前仓库推荐流程仍然是 WSL2 下运行脚本**。

## 8. 启动依赖环境

### 8.1 本地开发推荐：直接用 Docker Compose

在 WSL 中执行：

```bash
docker compose -f env/env_docker.yaml up -d
docker compose -f env/env_docker.yaml ps
```

这会启动当前本地联调需要的基础依赖环境。

停止环境：

```bash
docker compose -f env/env_docker.yaml down
```

### 8.2 远程或统一管理：通过 `main.sh docker`

如果你把这台 Windows + WSL 环境当成部署控制机，可以执行：

```bash
./main.sh install ansible
./main.sh docker install --env dev_local
./main.sh docker status --env dev_local
```

补充说明：

- `install ansible` 会调用 `deploy/install.sh`
- 该脚本会使用 Python venv 安装 Ansible
- Windows 下官方建议也是 `WSL/Git-Bash + Python`

## 9. 编译项目

推荐仍使用主入口：

```bash
./main.sh build
./main.sh build web
```

说明：

- 默认编译一组核心服务
- 编译产物输出到 `build/`
- 在 WSL 中编译得到的是 **Linux 二进制**，这正是当前推荐运行路径

## 10. 本地运行服务

当前推荐本地调试配置文件：

- `env/server_conf_ide.yaml`

示例：

```bash
./build/connsvr  -svr_conf=./env/server_conf_ide.yaml
./build/mainsvr  -svr_conf=./env/server_conf_ide.yaml
./build/infosvr  -svr_conf=./env/server_conf_ide.yaml
./build/websvr   -svr_conf=./env/server_conf_ide.yaml
```

建议顺序：

1. 先确认 `env/env_docker.yaml` 里的依赖已启动
2. 再启动核心服务
3. 需要 Web 管理时再启动 `websvr`

## 11. 可选：把 Windows + WSL 当成部署控制机

如果你还需要做部署或主机初始化，可以在 WSL 中继续使用：

```bash
./main.sh install ansible
./main.sh env list
./main.sh role list
./main.sh deploy --env dev1 --action restart --role websvr
./main.sh host init --env dev1
```

相关说明见：

- `deploy/README.md`

## 12. 如果你只想用 Git-Bash

Git-Bash 可以执行一部分 Bash 脚本，但它不是当前最推荐的完整开发路径。

适合 Git-Bash 的场景：

- 快速执行 `./main.sh help`
- 快速执行 `./main.sh doctor`
- 做少量脚本调用

不太建议只靠 Git-Bash 的场景：

- 完整的 Go 版本管理
- 依赖 Linux 行为一致性的编译与运行
- 部署控制机操作

如果你发现：

- `go-manager` 行为与预期不一致
- `docker` / `ansible` / shell 路径问题较多
- 编译结果与 Linux 运行目标不一致

请直接切回 WSL2。

## 13. 当前推荐方式与历史方式的区别

请不要再沿用以下旧路径：

- 手工下载安装 Go `1.13`
- 手工下载安装 RabbitMQ / ZooKeeper / Redis 到 Windows 宿主机
- 把 Windows 原生命令行当作主执行环境

当前推荐路径是：

- `WSL2 + Ubuntu`
- `Docker Desktop + WSL Integration`
- `main.sh`
- `env/env_docker.yaml`
- `env/server_conf_ide.yaml`

## 14. 配置表与生成工具

如果你需要导表或生成配置数据，请使用：

- `tools/cfgtool`

入口文件：

- `tools/cfgtool/main.go`

不要再按照历史资料去寻找旧版导表目录或旧版导表脚本。

## 15. 常见问题

### 15.1 在 Windows 上能直接运行 `main.sh` 吗

可以，但推荐环境是：

- WSL2
- 或 Git-Bash

不建议用 PowerShell / CMD 直接当成主要入口。

### 15.2 本地开发一定要装 Ansible 吗

不需要。只要你是本地联调，通常这条路径就够了：

```bash
docker compose -f env/env_docker.yaml up -d
./main.sh build
./build/connsvr -svr_conf=./env/server_conf_ide.yaml
```

### 15.3 WSL 里编译出来的不是 `.exe`，正常吗

正常。因为当前推荐路径就是在 WSL 里以 Linux 方式编译和运行服务。

### 15.4 如何关闭彩色输出

```bash
NO_COLOR=1 ./main.sh doctor
```
