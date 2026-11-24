# 02: 护理事件 HTTP 契约与 Go handler

**What to build:** 把护理事件冻结进 OpenAPI：四个端点、`CareEvent*` schemas、类型专属 `oneOf`、fixtures 与 Go 镜像；实现 `cmd/hdd/serve_care.go` 的列表 / 创建 / 修改 / 删除 handler，接票 01 的领域与存储；更新冻结端点集合。

**Blocked by:** 01。

**Status:** resolved

## 验收清单

- [ ] `schemas/openapi.yaml` 增加：
  - `GET /members/{member_id}/care-events`
  - `POST /members/{member_id}/care-events`
  - `PATCH /care-events/{event_id}`
  - `DELETE /care-events/{event_id}`
- [ ] `CareEvent` / `CareEventCreate` / `CareEventUpdate` / `CareEventList` 进入 OpenAPI；`CareEvent` 用四个完整对象 `oneOf`（feed / diaper / sleep / solid），`additionalProperties: false`；`source` enum 仅 `parent`。
- [ ] `GET` 的 `from` / `to` 必填、带偏移、`from < to`；空列表返回 `{ "events": [] }`；排序 `started_at DESC, id DESC`。
- [ ] `POST` 201 + `CareEvent`；`PATCH` 200 + `CareEvent`；`DELETE` 204；未知成员 / 事件 404；非法 body / 范围 / 类型字段 400；成人成员创建 400。
- [ ] `internal/httpapi/types.go` 增加 Go 镜像；`internal/httpapi/contract_test.go` 更新冻结端点集合、fixtures 对账、非法 body 用例。
- [ ] `schemas/testdata/http/` 增加四类型事件、列表、创建、非法 body 的 golden；fixture 与契约双向回转。
- [ ] `cmd/hdd/serve_care.go` 只做 HTTP 编解码、成员存在 / `is_child` 校验与错误映射；业务校验在 `internal/care`。
- [ ] `cmd/hdd/serve.go` 注册路由；`cmd/hdd/commands.go` 注入 care 端口；fake 测试覆盖 handler 分岔。
- [ ] `npm run codegen` 更新 `apps/mobile/src/api/schema.d.ts`，无未提交 diff（票 03 消费）。
- [ ] `go test ./...`、`go vet ./...`、`bash scripts/test-db.sh`、`python3 scripts/check-docs.py` 全绿。
- [ ] 不新增鉴权 / 账号 / 云同步；护理事件不写入报告 / 趋势 / 分析。

## 实现提示

- 契约形状以 `../spec.md` §5 与 `docs/design/2026-11-23-care-event-contract.md` 为准；不要另发明字段。
- `PATCH` 是完整替换：缺省可选字段按 `null`；类型改变时清空旧类型专属字段；`member_id` / `source` / `id` 不接收客户端修改。
- 成员校验复用现有 `requireMember` / `MemberByID` 模式；创建时额外判断 `is_child`，修改 / 删除不因成员成年而拒绝历史事件。
- 列表 SQL 的 overlap 语义在票 01；handler 只解析 `from` / `to` 并调用存储。
- OpenAPI 的 `oneOf` 四个完整对象比 `allOf` 更稳：仓库的迷你 validator 支持 `oneOf` / `anyOf`，不依赖 `allOf` / discriminator 扩展。
- 错误体沿用 `httpapi.Error`，message 中性；不要把 SQL / 内部路径暴露给客户端。
- 更新 `internal/httpapi/contract_test.go` 的 frozen path set 是预期变更；不要用“测试放行”绕过。
- 文档收尾见票 05；本票可先更新 `docs/design/2026-11-23-care-event-contract.md` 的“HTTP 落地”状态。

## Comments

**实现（2026-11-24）：**

- `schemas/openapi.yaml`：新增 4 个端点、`EventID` 参数、`CareEvent*` / `CareEventCreate*` schemas；`CareEvent` 与创建 / 修改 body 都用四个完整对象 `oneOf`，每个分支 `additionalProperties: false`，`type` 用 `const` 判别；类型专属字段只出现在对应分支，未列出的字段不出现。
- `internal/httpapi/types.go`：新增 `CareEvent` / `CareEventCreate` / `CareEventUpdate` / `CareEventList` 镜像；`EndedAt` / `Note` 始终序列化为 null，类型专属可选字段用 `omitempty`。
- `cmd/hdd/serve_care.go`：列表解析必填 RFC3339 `from` / `to` 并校验 `from < to`；创建校验成员存在且 `is_child`；修改 / 删除按事件 id 返回 404；handler 只做编解码、成员校验与错误映射，类型规则复用 `internal/care.Validate`。
- `cmd/hdd/commands.go` / `config.go` / `serve.go`：注入 `care.Store` 并注册 4 条路由。
- `schemas/testdata/http/`：新增 feed / diaper / sleep / solid / list / create / update / invalid 8 个 fixture；`internal/httpapi/contract_test.go` 更新冻结端点集合、fixture 对账、invalid 用例。
- `cmd/hdd/serve_care_test.go`：覆盖创建 / 列表 / 空数组 / 修改保身份 / 删除 / 成人拒绝 / 未知成员 / 未知事件 / 范围校验 / 非法 body / 存储错误。
- `npm run codegen` 更新 `apps/mobile/src/api/schema.d.ts`；App 仍不消费（票 03）。
- 验证：`go test ./...`、`go vet ./...`、`bash scripts/test-db.sh`、`cd apps/mobile && npm run codegen && npm run typecheck && TZ=UTC npm test -- --runInBand && npm run lint -- --max-warnings=0`、`python3 scripts/check-docs.py` 全绿。
- 取舍：`CareEventUpdate` 与创建 body 共用同一组分支；`sleep.duration_min` 在 wire 上可选（缺省 = null），由服务端按 `ended_at` 配对校验；`CareEvent` 响应中空的可选类型字段省略，App 侧映射时按 null 处理。
