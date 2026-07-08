## GoOne 环境（env 目录）

### Docker 环境服务（MySQL/Redis/ZooKeeper/RabbitMQ）

本目录的 `env_docker.yaml` 是一份 docker compose 配置，用于启动 GoOne 依赖的中间件服务。

推荐通过主控制台 `main.sh` 调度（统一日志/颜色/参数风格）：

```bash
./main.sh docker install --env dev1
./main.sh docker restart --env dev1
./main.sh docker status  --env dev1
./main.sh docker logs    --env dev1
```

- **env 选择**
  - `--env dev1/dev2/dev_local` 对应 `deploy/hosts/host_dev.txt` 里的 inventory group（已包含认证配置）。
- **只对某一台机器执行**

```bash
./main.sh docker install --env dev1 --limit 43.139.3.228
```

直接运行底层脚本也可以（仓库根目录下）：

```bash
cd etc/env
./docker.sh install --env dev1
```


