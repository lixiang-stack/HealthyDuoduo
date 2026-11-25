# 01: 护理事件表与存储

**What to build:** 新增 `care_events` 表（migration 0021）与 `internal/care` 领域包：四类事件的类型 / 校验 / 存储端口；`internal/storage/store_care.go` 实现列表（范围重叠）、创建、修改、删除；成员删除级联；storage 测试覆盖约束与重叠语义。

**Blocked by:** None。

**Status:** resolved

## 验收清单

- [ ] `internal/storage/migrations/0021_care_events.sql` 新建 `care_events`：`id` / `member_id` / `type` / `source` / `started_at` / `ended_at` / `note` / 类型专属列；`member_id` 外键 `ON DELETE CASCADE`。
- [ ] DB 约束覆盖：四类型枚举；类型专属字段互斥；`amount_ml` / `amount_g` / `duration_min` 存在时 > 0；`ended_at >= started_at`；`sleep` 的 `ended_at` 与 `duration_min` 同时为空或同时非空；`note` / `food` trim；`source = 'parent'`。
- [ ] 索引 `(member_id, started_at DESC, id DESC)`。
- [ ] `internal/care` 提供 `CareEvent`、`CareEventInput`、类型常量、`Validate` 与存储端口；不出现 `recorded_by` / `device_id` / `caregiver` / 全局 active child。
- [ ] 列表按重叠语义返回：`from` 含、`to` 不含；点事件只落在 `started_at`；进行中睡眠开放；跨午夜睡眠同时出现在两天；排序 `started_at DESC, id DESC`。
- [ ] 创建固定 `source = 'parent'`；修改完整替换内容但保留 `id` / `member_id` / `source`；删除未知 id 返回未找到；重复删除不静默成功。
- [ ] 成员删除后该成员护理事件全部消失。
- [ ] storage 测试覆盖四类型、非法类型字段、非法数值、时间倒序、范围边界、跨午夜、进行中睡眠、级联；`bash scripts/test-db.sh` 全绿。
- [ ] 不新增账号 / 鉴权 / 云同步；不改报告、趋势、分析、识别链路。

## 实现提示

- 表结构、CHECK、索引与重叠 SQL 以 `../spec.md` §4 为准；不要把类型专属字段塞进 JSONB。
- 领域规则放 `internal/care`；`internal/storage` 只做 SQL 与行扫描；HTTP 边界另票。
- 类型专属字段的互斥约束建议一条 `care_events_type_fields` CHECK 完成，数值约束、时间顺序、sleep 配对、trim 分开命名，失败时更容易定位。
- `source` 是契约字段，固定 `parent`；DB 存列并在 API 返回，不在客户端接收。
- 存储层也要调用领域校验（HTTP 边界之外的直连路径兜底），但 DB CHECK 仍是最后防线。
- 列表 SQL 用 `type` 条件区分“点事件 / 进行中睡眠”，不要用 `ended_at IS NULL` 直接判断。
- 迁移编号沿用 0021；Down 只删表。没有历史数据需要回填。
- 测试数据用真实形状（ISO 带偏移、跨午夜、可选字段为空），不要只造 `2026-01-01` 占位。

## Comments

**实现（2026-11-24）：**

- 新增 `internal/care/care.go`：四类型常量、`CareEvent` / `EventInput`、`Normalize` + `Validate`、领域错误；`store.go` 定义 `care.Store` 端口。
- 新增 `internal/storage/migrations/0021_care_events.sql`：单表四类型、类型专属 CHECK、数值 / 时间 / sleep 配对 / trim / source 约束、`(member_id, started_at DESC, id DESC)` 索引、级联删除。
- 新增 `internal/storage/store_care.go`：列表按重叠语义、创建 / 完整替换 / 删除；写入前 `Normalize` + `Validate`；`Update` 保留 `id` / `member_id` / `source`，未知 id 返回 `(nil, nil)`；`Delete` 返回是否删除。
- 新增 `internal/care/care_test.go` 与 `internal/storage/store_care_test.go`；`internal/storage/migrate_test.go` 更新 goose 基线 21、`care_events` 表 / 索引 / 约束 / 级联断言。
- 取舍：`Normalize` 单独导出，note 去空白、空串转 nil，solid food 去空白；其余类型把 food 留给 `Validate` 判类型字段非法。DB 仍用 CHECK 兜底，HTTP 边界不信任客户端。
- 验证：`go test ./...`、`go vet ./...`、`bash scripts/test-db.sh` 全绿（含 `TestMigrationSchema`、`TestCareEventStoreSmoke`）。
- 未实现 HTTP / OpenAPI / handler（票 02）；未改 App / 契约文档。
