# 01: `hdd serve` 骨架落真栈

**What to build:** 家长 App 的后端入口第一次存在：既有 CLI 二进制新增 serve 子命令成为常驻 HTTP 服务，随既有 compose 栈一起拉起，局域网上任何客户端都能探到它的心跳并读到版本。

**Blocked by:** None (can start immediately).

**Status:** resolved

- [x] `hdd serve`（带监听地址/端口参数）启动后，真栈上 `GET /api/v1/health` 返回 `{ok, version}`
- [x] compose 一条命令拉起四件套（postgres / 对象存储 / 识别服务 / 数据服务），数据服务对识别服务的依赖经既有依赖注入层就位
- [x] OpenAPI 3.1 契约文件初始化：`/api/v1` 前缀 + health 端点冻结入库
- [x] HTTP 边界集成测试骨架落位（复用既有编排依赖 fakes 模式），health 用例绿
- [x] 按 e2e 文档护栏对共享栈冒烟（快照锚点习惯保持）

## Comments

**实现（分支 `hdd-serve-skeleton`，单 commit，PR 待用户 push）**

- `cmd/hdd/serve.go`：serve 子命令。监听地址/端口 = `--addr <host:port>` 单参数（覆盖人工 `HDD_SERVE_ADDR`，缺省 `:8080` 绑全部网卡）；handler 栈 `newServeHandler(d *deps)` 持有 openDeps 注入的编排依赖（下票按此接 12 端点）。`GET /api/v1/health` → `{"ok":true,"version":"0.1.0"}`；其余路径/方法交 stdlib mux（404/405）。无信号处理/优雅关停——骨架阶段进程即服务生命周期。
- `cmd/hdd/main.go`：usage 与 dispatch 各加一行；`config.go` 的 `openDeps` 未动（依赖注入层原样复用）。
- `schemas/openapi.yaml`：OpenAPI 3.1 初始化，前缀经 `servers.url: /api/v1` 表达（换 IP/端口不碰契约），`Health` schema `{ok(const true), version}` 冻结;info.version 与 serve 常量同源对齐 `0.1.0`。
- `deploy/docker-compose.yml`：新增 `hdd` 服务（`docker compose -p deploy -f deploy/docker-compose.yml up -d` 一条命令四件套 postgres / seaweedfs / recognizer / hdd）。postgres 加 `pg_isready` healthcheck 供 `service_healthy` 启动门；hdd 对 recognizer 不落 depends_on——识别服务的依赖经 openDeps 的 `HDD_RECOGNIZER_URL` 注入（HTTP 客户端惰性建连）；`restart: unless-stopped` 兜底 seaweedfs 首启 raft 收敛窗口的 fail-fast 重试。
- `deploy/hdd.Dockerfile`：两段构建（golang:1.27-alpine 建静态二进制 → alpine 运行）。根 `.dockerignore` 从只带 `recognizer/` 扩为同时放行 `cmd/`、`internal/`、`go.mod/sum`——per-Dockerfile 兄弟 ignore 文件实测不被 compose/build 采用，改根白名单（recognizer 镜像只 `COPY recognizer/`，镜像内容不受影响，已实测 `recognizer.Dockerfile` 构建行为不变）。
- 测试 `cmd/hdd/serve_test.go`：HTTP 边界骨架（httptest 起 handler 栈 + 沿用 main_test.go 的本地小 fake 注入 deps）：health 200/形状、前缀冻结 404、参数错误 exit 2、无位置参数、坏地址 exit 1 立即返回。全仓 `go test ./...` / `go vet` / `gofmt` 绿（未动 Python 侧）。

**共享栈冒烟（e2e 护栏）**

1. `snapshot -o pre-serve-smoke` → `data/e2e-snapshots/pre-serve-smoke-20261013-203336.dump`（76148B）。
2. compose `up -d`（HDD_SERVE_PORT 缺省时 hdd 映射 8080）→ `curl 127.0.0.1:8080/api/v1/health` = `{"ok":true,"version":"0.1.0"}`；裸 `/health`=404、POST=405；`docker logs` goose 无迁移可跑（version 3，识别端/库零写入）。
3. 栈完整性核对：recognizer `/healthz` ok、DB 计数 24/24/24/315 未变、seaweedfs 未被触碰（12h  uptime）。
4. ⚠️ 环境披露：(a) postgres healthcheck 属于 compose 配置变更，`up` 时 postgres/recognizer 容器被重建一次（数据在 pgdata 卷上原样保留，快照也在）。

**review 修正（code-review 两轴后并入本 commit）**

- 🔴 修复：`.gitignore` 裸词 `hdd` 把 `cmd/hdd/serve.go`/`serve_test.go` 吞为新文件之外的路径——首轮 commit 因此漏掉两个源文件（镜像内 COPY 不经 git 所以冒烟仍过）。锚定为 `/hdd`(仅仓库根产物)后并入；两文件现随本 commit。
- 修复：对「bind 成功前打印 listening」的次序——改为先 `net.Listen` 失败即报错，成功后打印 `ln.Addr()` 真值。
- 修复：`hdd.Dockerfile` 注释改为如实描述根 `.dockerignore` 白名单方案；main.go/commands.go 头注释的「五命令/Five」计数同步更新。
- 三处有意保留（对抗性审查后）：`restart: unless-stopped`（「一条命令拉起四件套」在冷启动必需——seaweedfs raft 收敛窗口内 EnsureBucket fail-fast 退出，无重启策略则栈永缺第四件）；postgres healthcheck（同上的 `service_healthy` 启动门，重建容器一次属 adopting 代价，数据在卷上）；version 字符串在 Go 常量与 openapi.yaml 各写一份（M0 无 codegen，双处以注释互指同源，票 02 的契约镜像测试接管校验）。
- 镜像双架构（NAS amd64）按票 12 验收，本票不涉及 compose 双架构 buildx 配置。

**验收对照（orig. ticket 表）**：serve 带监听参数/真栈 health {ok,version} ✅；compose 一命令四件套 + 识别依赖经 openDeps ✅；OpenAPI 3.1 初始化(/api/v1 + health 冻结) ✅；HTTP 边界测试骨架(既有 fakes 模式) + health 绿 ✅；e2e 护栏共享栈冒烟(快照锚点 + 零库写入) ✅。

**留给下票的口子**：handler 栈延续 `newServeHandler(d)` 注入风格；OpenAPI 其余 11 端点与 Go 镜像/HTTP fixtures 对账见票 02。

