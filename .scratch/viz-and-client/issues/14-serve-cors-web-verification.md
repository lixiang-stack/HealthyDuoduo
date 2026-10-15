# 14: `hdd serve` 的 CORS 开关（Web 验证通道直连）

**What to build:** `hdd serve` 增加 env 门控的 CORS 响应头与预检处理（`HDD_SERVE_CORS_ORIGIN`，缺省空 = 与现状完全一致），让 Web 验证通道（Expo Web，浏览器 `localhost:8081`）能跨源直连数据服务（`localhost:8080`）。

**Why:** 票 04（PR #14）收尾实测：Web 版 App 打 `/api/v1/health` 被浏览器拦截（fetch 抛 `Failed to fetch`）——响应无 `Access-Control-Allow-Origin`，`OPTIONS` 预检 405。原生端（Expo Go/模拟器）不受 CORS 约束，是票面主验证环境；但 spec 把 Web 列为长期验证入口（「保留长期验证入口」），不修则后续每张客户端票的浏览器验证都不可用。

**Blocked by:** None（与 PR #14 独立，可先行合入）。

**Status:** resolved

- [x] `HDD_SERVE_CORS_ORIGIN` 缺省（空）→ 与现状逐字节一致：无 CORS 头、`OPTIONS` 仍 405
- [x] 设为具体 origin 或 `*` → 所有响应带 `Access-Control-Allow-Origin:<值>`；`OPTIONS` 预检 → 204 + `Access-Control-Allow-Methods` / `Access-Control-Allow-Headers`
- [x] compose 栈透传该变量且**缺省关闭**（家庭常驻实例默认不开，不扩大 ADR-0006 无鉴权信任边界）
- [x] Go 测试覆盖：默认关无 CORS 头、开启后响应带头、预检 204 且 `go test ./...` 全绿

## Comments

**实现（分支 `serve-cors`，worktree `.worktree/serve-cors`，单 commit，push/PR 待用户）**

- `cmd/hdd/serve.go`：新增 `withCORS(next, origin)` 中间件——`origin == ""` 直接返回 `next`（与未加中间件逐字节一致）；非空时给全部响应加 `Access-Control-Allow-Origin`，并把 `OPTIONS` 直接答为 204 + `Allow-Methods/Allow-Headers/Max-Age`。`origin` 是固定配置值（不随请求 Origin 变化），故不加 `Vary`。
- `newServeHandler(d *deps, corsOrigin string)`：开关经参数显式注入（不在 handler 内隐式读 env，遵守「显式依赖优于隐藏的全局状态」）；`runServe` 用既有的 `fromEnv` 读 `HDD_SERVE_CORS_ORIGIN`，与 `HDD_SERVE_ADDR` 同一姿势。
- `deploy/docker-compose.yml`：hdd 服务透传 `HDD_SERVE_CORS_ORIGIN: ${HDD_SERVE_CORS_ORIGIN:-}`——Docker 内环境隔离，不透传则该变量进不了容器；缺省空 = 关，家庭常驻实例不受影响。
- 测试（先写后实现）：`TestServeCORSOffByDefault`（无头 + 预检仍 405）、`TestServeCORSEnabled`（GET 带头、预检 204 且给全允许方法/头）；既有两处调用点补 `""`。

**验证证据**
- `gofmt -l` 空、`go vet ./...` 零输出、`go test ./...` 6 包全绿。
- **共享栈冒烟（e2e 护栏）**：`snapshot -o pre-serve-cors` → `data/e2e-snapshots/pre-serve-cors-20261015-160213.dump`（82308B）；`HDD_SERVE_CORS_ORIGIN='*' ... up -d --build hdd` 重建单容器 →
  - `GET /api/v1/health` + `Origin: http://localhost:8081` → `Access-Control-Allow-Origin: *`（修复前该头缺失）；
  - `OPTIONS` 预检 → `204` + `Allow-Methods: GET, POST, PATCH, DELETE, OPTIONS` + `Allow-Headers: Content-Type`（修复前 405）；
  - 栈完整性：`goose: no migrations to run. current version: 4`；四容器在跑；DB 计数 25/315/25/0 未变。

**披露 / 留给下票**
- 共享栈 hdd 现跑**本分支未合入**的代码且开了 `HDD_SERVE_CORS_ORIGIN=*`（浏览器验证临时态）。合入 main 后建议按 main 重建一次、并把该变量留在缺省关。
- 常驻/家庭实例**默认不开** CORS：ADR-0006 无鉴权前提下放开跨源等于让任意网页可读家庭数据；只有开发/验证栈按需打开。
- 本票落地后，票 04（PR #14）的浏览器冷启动 + 设置页保存在本机即可完成（此前被跨源拦截）。

