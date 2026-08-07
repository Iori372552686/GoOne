# AGENTS.md

## Scope
These instructions apply to the whole `GoOne` repository.
Prefer code over README or older docs when they disagree.

## Repository Snapshot
- `GoOne` is a Go microservice game backend.
- Active services under `src/` are `connsvr`, `infosvr`, `mainsvr`, `mysqlsvr`, `roomcentersvr`, and `web_svr`.
- Core shared layers live under `lib/`, shared config struct types under `module/gconf`, protocol sources under `api/proto/` and `game_protocol/proto/`, generated code under `api/gen/`.
- Deployment and local environment entrypoints are `main.sh`, `deploy/`, and `etc/env/`.

## Runtime Model
- Service entrypoints follow `cmd/<service>/main.go`: parse flags, then `<pkg>.NewApp().Run(context.Background())`; wiring lives in `src/<service>/app.go` as package `connsvr` / `mainsvr` / `infosvr` / `mysqlsvr` / `roomcentersvr` / `websvr` (for `web_svr`).
- `NewApp()` returns `*runtime.App` built with `runtime.MustNew("<svc>", bussvc.WithConfLoader(hooks...))` plus `app.MustRegister(...)` (registration order is Start order; reverse is Quiesce/Drain/Stop). The service name appears only in `MustNew`: standard components come from `lib/service/runtime/bussvc/stdcomp.go` constructors (`NewLoggerComponent(app)`, `NewAdminComponent(app, readyCheck)`, `NewTracingComponent(app)`, `NewRouterComponent(app, transMgr, drivers, ...)`) which read `app.Name()` and self-load config from `module/conf` at Start; bus drivers are wired with `rabbitmq.NewRegistry()`. Legacy `bootstrap.NewServiceApp` / `application.Init|Run` no longer exist.
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
- The single config entry point is `module/conf`: it loads the `-svr_conf` file (yaml/toml/json by extension), publishes an immutable snapshot, and exposes key-style access `conf.Get("connsvr.runtime.listen_port").Int()` and `conf.Unmarshal("base_cfg.dependencies.db_instances", &dbs)`. Do not introduce new package-level config globals.
- Config struct types (field contracts with yaml tags) live in `module/gconf/config.go`; use them as `conf.Unmarshal` targets. They no longer hold global state.
- Validation is registered per service in `module/conf/registry.go` and run via `conf.RunValidators("<service>")` during startup; admin port fallback uses named constants, not magic ints.
- Prefer the grouped config layout: `base_cfg.runtime`, `base_cfg.dependencies`, `base_cfg.debug`, and `<service>.identity/debug/runtime/capacity`.
- Bus services require `base_cfg.runtime.register_addr` and `base_cfg.runtime.bus_mq_addr`.
- `websvr` requires `websvr.runtime.http_server.port`; `websvr.runtime.grpc_server.port` is required only when gRPC is enabled.
- Local gamedata typically comes from `base_cfg.dependencies.game_data_dir`; remote/hot-reload setup goes through `base_cfg.dependencies.nacos_conf`.
- Config is start-up immutable; only gamedata hot-reloads. `conf.Watch` exists as a placeholder but returns `ErrWatchNotSupported`.

## Handler And Routing Rules
- Default integration path is IDL-driven `ssrpc`.
- Register handlers with generated code from `api/gen/**`, usually via `New<Service>SServer(...)`, `Register<Service>ToDispatcher(...)`, and `d.RegisterToTransactionMgr(...)`.
- Treat legacy `globals.TransMgr.RegisterCmd(...)` or `cmd_handler/register.go` as compatibility paths for older code, not the default for new work.
- When a handler needs domain state, reuse existing managers such as `globals.RoleMgr` or room managers instead of re-implementing load paths.
- Routing behavior depends on `BusId`, `module/misc.ServerRouteRules`, and `lib/service/svrinstmgr`; avoid ad-hoc routing logic.
- All bus services use sharded transaction processing via `bussvc.TransMgrComponent` (responses route by `DstTransID`, requests shard by RouterID/Uid serial key). Shard count is not externally tunable — it always defaults to `transaction.DefaultShardCount()` (it only partitions dispatch queues/transID space; handlers run one goroutine per transaction). Same-key queue backpressure defaults to `transaction.DefaultMaxPendingPerKey` (100); `roomcentersvr` explicitly overrides it to 200. Handlers must stay key-local; never rely on global single-thread ordering.

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
- `build.sh` is the repository build helper for the active `src/` services plus the tester tools: `connsvr`, `mainsvr`, `infosvr`, `mysqlsvr`, `roomcentersvr`, `web_svr` (output binary `websvr`), `tester`, and `stress`.
- On Windows, use `./build.ps1` for local builds; use PowerShell proto helpers such as `.\scripts\check_genproto.ps1 -Full`, and prefer WSL or Git-Bash for `main.sh`.
- Validate generated code with `./main.sh check-genproto`.
- Use `./main.sh check-genproto --full` when `game_protocol` output also needs verification.
- Local middleware dependencies are defined under `etc/env/env_docker.yaml`.
- Tester builds: `./build.sh tester stress` (or `.uild.ps1 tester stress` on Windows).

## Tester And Stress Client
- `tools/tester/` is the standalone client testing framework, ported from `seed-tester` and adapted to GoOne's `CSPacketHeader` + protobuf wire protocol.
- It supports two modes driven by `tools/tester/tester.toml` / `tools/tester/stress.toml`:
  - `regression`: `go run ./tools/tester/cmd/tester -config ./tools/tester/tester.toml`
  - `stress`: `go run ./tools/tester/cmd/stress -config ./tools/tester/stress.toml`
- New business test components go under `tools/tester/app/component/<name>/`, register via `component.Register(...)`, and implement `TesterComponent`; add `StressRunner` for pressure-test loops.
- The client speaks TCP by default (`transport = "tcp"`); set `transport = "ws"` to use WebSocket.

## Where To Look First
- For service startup and dependency wiring, inspect `src/<service>/app.go`.
- For shared boot behavior, inspect `lib/service/runtime/app.go` (lifecycle) and `lib/service/runtime/bussvc/` (standard component assembly).
- For config changes, inspect `module/gconf/config.go`.
- For routing or service discovery issues, inspect `lib/service/router/`, `lib/service/svrinstmgr/`, `lib/service/bus/`, and `module/misc/`.
- For web changes, inspect `src/web_svr/controller/` before touching bus-side handlers.
- For tester changes, inspect `tools/tester/internal/session/` (network/session layer) and `tools/tester/app/component/` (business components).
- For deployment behavior, inspect `deploy/README.md`, `deploy/deploy.sh`, and `deploy/scripts/server.sh`.

