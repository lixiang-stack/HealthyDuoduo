# 01: 趋势契约补 `name` 路径参数

**What to build:** 修正趋势端点的 OpenAPI 契约缺陷：路径 `/members/{member_id}/indicators/{name}/trend` 已经使用 `{name}`，但参数表没有声明它，导致 `openapi-typescript` 生成的 `getTrend` 只有 `member_id`，App 无法类型安全调用。补 `IndicatorName` 可复用参数、修正 `NotFound` 描述、重新 codegen、加契约断言；不改运行时行为。

**Blocked by:** None。

**Status:** resolved

## 验收清单

- [x] `schemas/openapi.yaml` 新增 `components/parameters/IndicatorName`：
  - `in: path`
  - `required: true`
  - `schema: {type: string, minLength: 1}`
  - 描述指明「指标项名称（通常为词典规范名；修正后以修正值为准），URL 编码」。
- [x] trend 端点 parameters 用 `$ref` 引用 `IndicatorName`（与其他 path 参数风格一致）。
- [x] `components/responses/NotFound` 描述删除「或 trend name 未收录」，与「不做词典成员检查、空结果 200」一致。
- [x] 12 端点集合不变；既有 HTTP fixtures 不变；不新增/删除端点或字段。
- [x] `cd apps/mobile && npm run codegen` 后，`src/api/schema.d.ts` 的 `getTrend.parameters.path` 同时包含 `member_id` 与 `name`。
- [x] `internal/httpapi/contract_test.go` 增加断言：趋势端点声明了 `name` path 参数，且 `in: path`、`required: true`、`minLength: 1`；`NotFound` 描述不含「trend name 未收录」。
- [x] `go test ./internal/httpapi` 通过；`npm run typecheck` 通过。
- [x] 不改 handler / storage / App 页面；本票只改契约与生成物。

## 实现提示

- 端点定义在 `schemas/openapi.yaml` 约 line 273；现有参数是 `MemberID` 与 `unit`。
- 可复用参数定义在 `components/parameters`（`MemberID` / `ReportID` / `ItemID`）；新增 `IndicatorName` 与它们并列。
- contract 测试的路径集合在 `internal/httpapi/contract_test.go` 的 `TestOpenAPIFrozenPathSet`；新增独立断言即可，不要把参数检查塞进路径集合测试。
- codegen 脚本：`apps/mobile/package.json` 的 `"codegen": "openapi-typescript ../../schemas/openapi.yaml -o src/api/schema.d.ts"`。
- 该参数是已有路径的正式声明，不是契约扩展；不改 `docs/design/2026-10-13-viz-and-client-spec.md` 的端点表。

## Comments

**实现（2026-10-23，分支 `abnormal-trends`）**

- 新增可复用 `components/parameters/IndicatorName`（`in: path` / `required: true` / `minLength: 1`，描述含「通常为词典规范名；修正后以修正值为准」）；趋势端点 `$ref` 引用；`NotFound` 删除「或 trend name 未收录」。
- `npm run codegen` 重新生成 `apps/mobile/src/api/schema.d.ts`，`getTrend.parameters.path` 同时包含 `member_id` 与 `name`。
- `contract_test.go` 新增 `TestOpenAPITrendNameParam` / `TestOpenAPINotFoundDescription`；12 端点集合与 fixtures 不变；未改 handler/storage/App 页面。
- 验证：`go test ./internal/httpapi`、`go test ./...`、`npm run typecheck`、`npm test -- --silent` 全绿。
