# AGENTS.md

## Scope
These instructions apply to the whole `GoOne` repository.
Prefer code over README or older docs when they disagree.

## Repository Snapshot
- `GoOne` is a Go microservice game backend.
- Active services under `src/` are `connsvr`, `infosvr`, `mainsvr`, `mysqlsvr`, `roomcentersvr`, and `web_svr`.
- Core shared layers live under `lib/`, shared config under `common/gconf`, protocol sources under `api/proto/` and `game_protocol/proto/`, generated code under `api/gen/`.
- Deployment and local environment entrypoints are `main.sh`, `deploy/`, and `etc/env/`.

## Runtime Model
- Service entrypoints follow `cmd/<service>/main.go`: parse flags, call `application.Init(<pkg>.NewApp())`, then `application.Run()`; wiring lives in `src/<service>/app.go` as package `connsvr` / `mainsvr` / `infosvr` / `mysqlsvr` / `roomcentersvr` / `websvr` (for `web_svr`).
- `newApp()` normally returns `bootstrap.NewServiceApp(...)` and defines `LoadConfig`, `InitDeps`, `RegisterHandlers`, `StartRuntime`, `OnProc`, `OnTick`, and `OnExit`.
- Main packet flow is: client -> `connsvr` -> `lib/service/router` -> bus/routing rules -> target service `globals.TransMgr`.
- `web_svr` is the main exception: it starts Gin HTTP routes and optional gRPC listeners rather than joining the normal bus transaction loop.

## Service Conventions
- `connsvr` is the TCP/WebSocket gateway and owns client-facing listeners.
- `mainsvr` holds player-facing business logic and commonly loads role state through `globals.RoleMgr`.
- `roomcentersvr` owns room lifecycle and room tick work.
- `mysqlsvr` is persistence-oriented and depends on ORM instances from config.
- `infosvr` is a lighter cache/profile service.
- `web_svr` mounts HTTP routes from `src/web_svr/controller` and may expose gRPC as well.

## Config Rules
- Shared config definitions are in `common/gconf/config.go`.
- All services read the `-svr_conf` flag and load `base_cfg` plus their own service section.
- Prefer the grouped config layout such as `base_cfg.runtime`, `base_cfg.dependencies`, `base_cfg.debug`, and `<service>.identity/debug/runtime/capacity`.
- Legacy flat fields still exist for compatibility; do not remove them casually unless the loader and config files are updated together.
- Bus services require `base_cfg.runtime.register_addr` and `base_cfg.runtime.bus_mq_addr`.
- `websvr` requires `runtime.http_server.port`; `runtime.grpc_server.port` is required only when gRPC is enabled.
- Local gamedata typically comes from `dependencies.game_data_dir`; remote/hot-reload setup goes through `dependencies.nacos_conf`.

## Handler And Routing Rules
- Default integration path is IDL-driven `ssrpc`.
- Register handlers with generated code from `api/gen/**`, usually via `New<Service>SServer(...)`, `Register<Service>ToDispatcher(...)`, and `d.RegisterToTransactionMgr(...)`.
- Treat legacy `globals.TransMgr.RegisterCmd(...)` or `cmd_handler/register.go` as compatibility paths for older code, not the default for new work.
- When a handler needs domain state, reuse existing managers such as `globals.RoleMgr` or room managers instead of re-implementing load paths.
- Routing behavior depends on `BusId`, `module/misc.ServerRouteRules`, and `lib/service/svrinstmgr`; avoid ad-hoc routing logic.
- Preserve the serialized transaction model in `mainsvr` and `roomcentersvr`; they intentionally use sharded transaction processing that differs from lighter services like `connsvr`.

## Generated Code Boundaries
- Do not hand-edit `api/gen/**`.
- Do not hand-edit `game_protocol/protocol/*.pb.go`.
- Do not hand-edit `common/gamedata/repository/**/*.gen.go`.
- When protocol changes are needed, edit the source proto files and regenerate.
- `go.mod` replaces `github.com/Iori372552686/game_protocol` with local `./game_protocol`, so protocol work belongs in the local module.

## Build And Verification
- Preferred top-level entrypoint is `main.sh`.
- Start with `./main.sh doctor` when checking a local environment.
- Common builds are `./main.sh build`, `./main.sh build web`, and `./main.sh build roomcenter`.
- `build.sh` is the repository build helper for the active `src/` services only: `connsvr`, `mainsvr`, `infosvr`, `mysqlsvr`, `roomcentersvr`, and `web_svr` (output binary `websvr`).
- On Windows, use `./build.ps1` for local builds; use PowerShell proto helpers such as `.\scripts\check_genproto.ps1 -Full`, and prefer WSL or Git-Bash for `main.sh`.
- Validate generated code with `./main.sh check-genproto`.
- Use `./main.sh check-genproto --full` when `game_protocol` output also needs verification.
- Local middleware dependencies are defined under `etc/env/env_docker.yaml`.

## Where To Look First
- For service startup and dependency wiring, inspect `src/<service>/app.go`.
- For shared boot behavior, inspect `lib/service/bootstrap/` and `lib/service/application/app.go`.
- For config changes, inspect `common/gconf/config.go`.
- For routing or service discovery issues, inspect `lib/service/router/`, `lib/service/svrinstmgr/`, `lib/service/bus/`, and `module/misc/`.
- For web changes, inspect `src/web_svr/controller/` before touching bus-side handlers.
- For deployment behavior, inspect `deploy/README.md`, `deploy/deploy.sh`, and `deploy/scripts/server.sh`.

