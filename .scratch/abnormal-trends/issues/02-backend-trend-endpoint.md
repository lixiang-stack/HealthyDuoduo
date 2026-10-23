# 02: 后端趋势端点（成员维度 + 单位门控）

**What to build:** 实现冻结的 `GET /api/v1/members/{member_id}/indicators/{name}/trend?unit=`：成员维度查询同一指标项名称的跨报告时序点，返回日期、值、单位、参考范围、方法、flag、低置信与 reviewed 标记；按日期升序、dateless 沉底；不新增迁移、不改识别端/CLI。

**Blocked by:** 01（契约参数）。

**Status:** resolved

## 验收清单

- [x] `internal/pipeline/types.go` 新增窄接口 `TrendStore`（`TrendPoints(ctx, memberID, name, unit)`）与 `TrendPointRow`；不修改 `pipeline.Store` / CLI `TrendByName`。
- [x] `internal/storage/store.go` 新增成员维度查询：
  - `WHERE r.member_id = $1 AND i.name = $2`；`unit` 非空时 `i.unit = $3`。
  - `ORDER BY r.report_date ASC NULLS LAST, r.image_sha256 ASC, i.id ASC`。
  - 字段：`image_sha256`、`report_date`（YYYY-MM-DD / nil）、`value`、`unit`、`ref_range`、`method`、`flag`、`low_confidence`、`review_status`。
  - 空日期扫描为 nil；`*storage.Store` 实现 `pipeline.TrendStore`。
  - 与 CLI 的 `TrendByName` 语义不同（CLI 全局、无 unit/ref/method/review；HTTP 成员 + unit + 更多字段），本期不合并；注释写清。
- [x] `cmd/hdd/commands.go` 的 `deps` 增加 `trends pipeline.TrendStore`；`config.go` 注入同一 `*storage.Store`。
- [x] `cmd/hdd/serve.go` 注册 `GET /api/v1/members/{member_id}/indicators/{name}/trend`；`cmd/hdd/serve_trend.go` 实现 handler：
  - 非法/未知 member_id → 404 `member not found`。
  - `name` 从 `r.PathValue("name")` 取，支持中文与 `%2F`（带 `/` 的规范名）。
  - `unit` 从 query 取；空 = 全部单位。
  - 200 返回 `httpapi.TrendResult`，`points` 初始化为空 slice（无点 = `[]`）。
  - `reviewed = review_status ∈ {confirmed, corrected}`。
  - 存储错误 → 500 `list trend failed`。
- [x] handler 测试覆盖：200 形状、中文名、`谷草/谷丙比值`、unit 门控、未知成员与非法 member_id 404、空数组、reviewed、500。
- [x] storage smoke 覆盖：成员隔离、unit 门控、日期升序 / dateless 沉底、method / review_status 透传；按现有 gated 真库约定（`HDD_TEST_DSN`）。
- [x] 无数据库迁移；不改 CLI `hdd trend` 输出；不改识别端 / golden。
- [x] `go test ./...` 通过；storage smoke 在一次性 PG 上通过。

## 实现提示

- 现有 CLI 查询 `sqlTrendByName` / `sqlTrendByNameAndType` 在 `internal/storage/store.go:80-92`；新查询是不同消费面（成员维度 + 更多字段），不要把 CLI 语义耦合进 serve。
- `internal/httpapi/types.go:167-189` 已有 `TrendPoint` / `TrendResult`；handler 只做映射，不新增契约字段。
- 路由风格参考 `cmd/hdd/serve.go` 的 `/api/v1/reports/...`；成员校验参考 `handleListReports` / `requireMember`。
- 测试 fake 参考 `cmd/hdd/serve_reports_test.go` 的 `fakeServeStore` 与 `fakeMemberStore`；趋势测试可加独立 `fakeTrendStore`，不必膨胀既有 fake。
- 404 口径：契约只声明 200/404，非法 member_id 也按「未知成员」处理；不要返回 400。
- 空结果返回 200 + `points: []`；不做词典成员检查。

## Comments

**实现（2026-10-23，分支 `abnormal-trends`）**

- `TrendStore` / `TrendPointRow` 新增；`*storage.Store.TrendPoints` 成员维度查询（WHERE member/name + 可选 unit；ORDER BY date/sha/id）；未动 `pipeline.Store` / CLI `TrendByName`。
- `deps.trends` 注入同一 store；`serve.go` 注册路由；`serve_trend.go` handler（非法/未知成员 404、空数组、reviewed、500）。
- handler 测试覆盖 200 / 中文 + `%2F` / unit 透传 / reviewed / 空数组 / 404 / 500；storage smoke 覆盖成员隔离 / unit 门控（含 NULL unit）/ 日期 / 同日 sha + `i.id` / 字段透传。
- 验证：`go test ./...` 全绿；`HDD_TEST_DSN=... go test ./internal/storage -run Trend` 在一次性 `postgres:17-alpine` 上通过；未新增迁移、未改 CLI / 识别端 / golden。
