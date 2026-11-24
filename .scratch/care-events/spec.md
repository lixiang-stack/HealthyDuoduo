# Spec：护理事件持久化与跨设备共享（A1）

Status: **proposed**（2026-11-24；等待实现）
范围：把票 01 的客户端 `CareEvent` 契约落到 LAN 后端：`care_events` 表、HTTP API、App HTTP adapter、跨设备刷新与提醒重排。
不包含：睡眠记录方式改动（`../care-log/issues/09-sleep-completion.md`）、删除确认、跨天补录、键盘遮挡、统计窗口、健康档案圆点、就诊摘要。
依据：`docs/design/2026-11-23-care-event-contract.md` · `docs/adr/0019-care-log-navigation-child-context.md` · `docs/adr/0006-lan-first-no-auth.md` · `docs/reference/wechat-feeding-tracker.md` §7。

---

## 0. 决策速览

| 主题 | 决策 |
| --- | --- |
| 数据形态 | 四类事件共用一张 `care_events` 表；类型专属列为可空列 + CHECK，不拆四张表，不用 JSONB |
| API | 沿用客户端契约：`GET/POST /members/{member_id}/care-events`，`PATCH/DELETE /care-events/{event_id}` |
| 服务端校验 | 目标成员必须是 `is_child`；金额 / 时长 / 克数 > 0；`ended_at >= started_at`；食物非空；`note` trim；`source = 'parent'` |
| 时间语义 | 不变：点事件 `ended_at = null`，进行中睡眠也为 `null`，由 `type` 区分；SQL 用类型条件处理重叠 |
| 睡眠 | 生产 UI 之后改为“结束后补记”（护理记录票 09）；本 spec 保留 ongoing 契约，但不依赖生产创建 ongoing |
| 跨设备刷新 | focus 重载 + 下拉刷新；不做轮询 / SSE |
| 提醒 | 每设备一份设置，跟随当前孩子；切换孩子和冷启动按最新喂养重排 |
| App 接入 | 手写 `CareEvent` 保留为领域 seam；HTTP 层做 wire 映射；provider 显式注入，生产不静默回落内存 |
| 迁移 | `0021_care_events.sql`；新表，无回填 |
| 文档 | 本 spec 为权威设计记录；不另写 `docs/design` 复述；只更新既有契约文档与 ADR 链接 |

---

## 1. 为什么做

B3 的今天 / 统计 / 记录弹层 / 重复提示 / 提醒已经按票 01–08 落地，但护理事件仍由 `InMemoryCareEventRepository` 承载：

- App 重启后事件全部消失；
- 两台设备看不到对方刚记的喂养 / 尿布；
- 重复喂养提示查不到另一台设备的记录；
- 提醒的“上次喂养”重启后回到空态；
- 记录弹层写着“记录保存在家庭局域网”，但数据从未离开内存。

这直接否定 B3 的核心前提：爸爸、妈妈各自设备记录同一个孩子。A1 只解决“数据在哪里、如何在设备间共享”，不改变记录类型的业务含义，也不把护理事件写进报告 / 趋势 / 分析。

---

## 2. 目标与非目标

### 2.1 目标

1. 护理事件写入 LAN 后端，重启 App 后仍可读取。
2. 两台设备在同一成员、同一时间范围读到同一批事件；保存后另一端可通过下拉刷新看到。
3. 重复喂养提示仍能读到另一台设备刚写入的喂养。
4. 提醒在冷启动、切换孩子后按当前孩子的最新喂养重排，不保留旧孩子的系统通知。
5. 服务端独立执行契约不变量，不依赖 App 校验。
6. 不改报告 / 趋势 / 分析 / 识别链路；不新增账号、记录人、设备归属或全局 active child。

### 2.2 非目标

- 睡眠记录方式（护理记录票 09）。
- 删除二次确认、跨天补录、键盘遮挡（后续体验票）。
- 健康档案跨成员圆点、欢迎屏、30 天 / 自定义统计窗口、就诊摘要 / PDF。
- 账号、邀请、云同步、远程推送、跨公网访问。
- 护理事件写入 `reports` / `report_items` / 趋势 / 分析快照。
- 分页、WebSocket / SSE、离线队列、本地缓存。

---

## 3. 术语

- **护理事件**：`feed` / `diaper` / `sleep` / `solid` 四类，归属一个 `is_child` 成员。
- **点事件**：`feed` / `diaper` / `solid`，只发生在 `started_at`，`ended_at` 固定为 `null`。
- **进行中睡眠**：`sleep` 且 `ended_at = null`、`duration_min = null`；契约保留，生产 UI 在票 09 后不再创建。
- **范围读取**：`from` 含、`to` 不含，按重叠返回；倒序 `started_at DESC, id DESC`。
- **来源**：`source = 'parent'`，用户可见文案「家长记录」；没有具体记录人。

---

## 4. 数据模型

### 4.1 表

迁移：`internal/storage/migrations/0021_care_events.sql`。

```sql
CREATE TABLE care_events (
    id            BIGSERIAL PRIMARY KEY,
    member_id     BIGINT NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    type          TEXT NOT NULL CHECK (type IN ('feed', 'diaper', 'sleep', 'solid')),
    source        TEXT NOT NULL DEFAULT 'parent' CHECK (source = 'parent'),
    started_at    TIMESTAMPTZ NOT NULL,
    ended_at      TIMESTAMPTZ,
    note          TEXT,
    method        TEXT,
    amount_ml     DOUBLE PRECISION,
    duration_min  INTEGER,
    diaper_type   TEXT,
    food          TEXT,
    amount_g      DOUBLE PRECISION
);
```

不新增 `recorded_by`、`device_id`、`caregiver`、`active_child` 或任何账号字段。

### 4.2 约束

```sql
ALTER TABLE care_events
    ADD CONSTRAINT care_events_type_fields CHECK (
        (type = 'feed'   OR method IS NULL) AND
        (type <> 'feed'  OR (method IS NOT NULL AND method IN ('breast', 'formula'))) AND
        (type = 'diaper' OR diaper_type IS NULL) AND
        (type <> 'diaper' OR (diaper_type IS NOT NULL AND diaper_type IN ('wet', 'dirty', 'mixed'))) AND
        (type = 'solid'  OR food IS NULL) AND
        (type <> 'solid' OR (food IS NOT NULL AND btrim(food) <> '')) AND
        (type = 'solid'  OR amount_g IS NULL) AND
        (type = 'feed'   OR amount_ml IS NULL) AND
        (duration_min IS NULL OR type IN ('feed', 'sleep')) AND
        (type = 'sleep'  OR ended_at IS NULL)
    ),
    ADD CONSTRAINT care_events_positive_values CHECK (
        (amount_ml IS NULL OR amount_ml > 0) AND
        (amount_g IS NULL OR amount_g > 0) AND
        (duration_min IS NULL OR duration_min > 0)
    ),
    ADD CONSTRAINT care_events_time_order CHECK (
        ended_at IS NULL OR ended_at >= started_at
    ),
    ADD CONSTRAINT care_events_sleep_pair CHECK (
        type <> 'sleep' OR ((ended_at IS NULL) = (duration_min IS NULL))
    ),
    ADD CONSTRAINT care_events_note_trimmed CHECK (
        note IS NULL OR (btrim(note) <> '' AND note = btrim(note))
    ),
    ADD CONSTRAINT care_events_food_trimmed CHECK (
        food IS NULL OR food = btrim(food)
    );
```

索引：

```sql
CREATE INDEX care_events_member_started_idx
    ON care_events (member_id, started_at DESC, id DESC);
```

### 4.3 范围重叠

`from` 含、`to` 不含；点事件只出现在 `started_at` 所在范围；进行中睡眠开放到未来；跨午夜睡眠同时出现在两天的范围里。

```sql
SELECT ...
FROM care_events
WHERE member_id = $1
  AND started_at < $3
  AND (
      (type = 'sleep' AND ended_at IS NULL)
      OR (ended_at IS NOT NULL AND ended_at > $2)
      OR (type <> 'sleep' AND ended_at IS NULL AND started_at >= $2)
  )
ORDER BY started_at DESC, id DESC;
```

### 4.4 写入语义

- `create`：`member_id` 来自路径；`source` 由服务端固定；`id` 由存储分配。
- `update`：完整替换可编辑内容；`id`、`member_id`、`source` 不变；类型可改变，旧类型专属列必须清空。
- `delete`：硬删除；未知 id 返回 404。
- 成员删除：`ON DELETE CASCADE` 清理该成员全部护理事件。
- 时间一律存绝对 `TIMESTAMPTZ`；展示与“今天”分组由客户端按本机时区计算。

---

## 5. HTTP API

### 5.1 路径

| 操作 | 路径 | 成功 |
| --- | --- | --- |
| 列表 | `GET /api/v1/members/{member_id}/care-events?from=<RFC3339>&to=<RFC3339>` | `200 { events: CareEvent[] }` |
| 创建 | `POST /api/v1/members/{member_id}/care-events` | `201 CareEvent` |
| 修改 | `PATCH /api/v1/care-events/{event_id}` | `200 CareEvent` |
| 删除 | `DELETE /api/v1/care-events/{event_id}` | `204` |

`from` / `to` 必填，必须带时区偏移，`from < to`；本期不分页。App 只读最近 7 天窗口（今天页 / 统计页），提醒复用同一读模型。

### 5.2 形状

`CareEvent` 继承 `docs/design/2026-11-23-care-event-contract.md`：

```json
{
  "id": 1,
  "member_id": 2,
  "type": "feed",
  "source": "parent",
  "started_at": "2026-11-24T14:05:00+08:00",
  "ended_at": null,
  "note": null,
  "method": "formula",
  "amount_ml": 120,
  "duration_min": null,
  "diaper_type": null,
  "food": null,
  "amount_g": null
}
```

- 创建 / 修改 body 不包含 `id`、`member_id`、`source`。
- `PATCH` 是完整替换：缺少的可选字段按 `null` 处理；类型改变时旧类型字段必须为 `null`。
- 列表空结果返回 `{ "events": [] }`，不返回 `null`。

### 5.3 状态码

| 场景 | 状态 |
| --- | --- |
| 成员不存在 | 404 |
| 成员存在但不是孩子（仅创建时校验） | 400 |
| 事件不存在（修改 / 删除） | 404 |
| body / 时间范围 / 类型专属字段非法 | 400 |
| 存储失败 | 500 |

错误体沿用 `httpapi.Error`：`{ "message": "..." }`，message 为中性说明。

### 5.4 OpenAPI / 契约

- `schemas/openapi.yaml` 增加上述 4 个端点与 `CareEvent*` schemas。
- `CareEvent` 用 `oneOf` 四个完整对象（feed / diaper / sleep / solid），每个 `additionalProperties: false`，避免判别联合在 wire 上退化为宽表。
- `CareEventCreate` / `CareEventUpdate` 用同样四个 body 对象。
- `internal/httpapi/types.go` 增加 Go 镜像；`internal/httpapi/contract_test.go` 更新冻结端点集合与 fixture 对账。
- `schemas/testdata/http/` 增加四类、列表、创建、非法 body 的 golden。
- App `npm run codegen` 更新 `apps/mobile/src/api/schema.d.ts`。

---

## 6. 服务端校验

按顺序：

1. 路径 `member_id` 可解析；成员存在，否则 404。
2. 创建时成员 `is_child`，否则 400；修改 / 删除不重新校验 `is_child`（历史事件不因成员成年而不可编辑）。
3. 时间：`started_at` / `ended_at` 可解析且带偏移；`ended_at >= started_at`。
4. 类型专属：
   - `feed`：`method` 必填（`breast` / `formula`）；`amount_ml` / `duration_min` 可选且 > 0；其余类型字段必须为 `null`；`ended_at` 必须为 `null`。
   - `diaper`：`diaper_type` 必填（`wet` / `dirty` / `mixed`）；其余类型字段必须为 `null`；`ended_at` 必须为 `null`。
   - `sleep`：`duration_min` / `ended_at` 必须同时为 `null`（进行中）或同时非空（已完成）；`duration_min > 0`；其余类型字段必须为 `null`。
   - `solid`：`food` trim 后非空；`amount_g` 可选且 > 0；其余类型字段必须为 `null`；`ended_at` 必须为 `null`。
5. `note` trim；空串存 `null`；不做长度限制（本期）。
6. `source` 不接收客户端输入，服务端固定 `parent`。
7. 存储层用同一套约束兜底；HTTP 边界与 DB CHECK 必须同时存在。

领域规则放在 `internal/care`（新包）：类型、校验、输入 / 输出、存储端口。`internal/storage/store_care.go` 实现端口；`cmd/hdd/serve_care.go` 只做 HTTP 编解码、成员校验与错误映射。

---

## 7. App 接入

### 7.1 API 封装

新增 `apps/mobile/src/api/careEvents.ts`：

- `listCareEvents(client, memberId, range)`
- `createCareEvent(client, memberId, body)`
- `updateCareEvent(client, eventId, body)`
- `deleteCareEvent(client, eventId)`

以 OpenAPI 生成类型为 wire 边界；`apps/mobile/src/care/careEvents.ts` 的手写 `CareEvent` 继续作为 UI / 领域 seam，在 API 层做映射。这样 UI 不依赖生成类型的 union 细节，契约测试仍能锁定 wire 形状。

### 7.2 HTTP repository

新增 `apps/mobile/src/care/httpCareEventRepository.ts`：

```ts
export class HttpCareEventRepository implements CareEventRepository {
  constructor(private readonly resolveClient = resolveApiClient) {}
  // list / create / update / remove 每次解析 client
}
```

- 每次调用 `resolveApiClient()`，兼容设置页修改 Base URL。
- 未配置后端时抛可识别错误（文案「尚未配置后端地址」），不静默回落内存。
- 服务端错误用 `Error(message)` 抛出；页面现有 loading / error 呈现继续复用。
- 不做本地缓存、乐观更新或离线队列。

### 7.3 Provider

- `CareEventProvider` 的 `repository` 改为必填；生产根布局注入 `HttpCareEventRepository`，测试 `renderScreen` 继续注入 `InMemoryCareEventRepository`。
- 移除生产默认 `new InMemoryCareEventRepository()`，防止“忘了接线”时静默回到内存；根布局用 `useMemo` 或模块级单例，避免每次 render 新建仓储。
- `CareEventProvider.test.tsx` 更新为断言显式注入；不保留 production 默认内存仓储。

### 7.4 跨设备刷新

- 今天页时间线、统计页保留 focus 重载。
- 两个页面增加下拉刷新（`RefreshControl`），调用各自的 `reload`；刷新失败保留旧数据并显示现有错误态。
- 不做轮询、SSE 或后台同步。
- 重复喂养提示在保存前仍读服务端；跨设备并发仍是 best-effort（ADR-0019）。

### 7.5 提醒

- 设置仍是每设备一份，跟随当前孩子；不按孩子拆多套设置。
- 在育儿域拿到当前孩子后调用 `syncReminderToLatestFeed`（该函数自己读最新喂养，不依赖今天页是否挂载）：
  - 切换孩子：先取消旧排程，再按新孩子的最新喂养重排；新孩子读取失败时保持取消（宁缺勿错，避免旧孩子通知误报）；
  - 冷启动：按持久化后的最新喂养重排；读取失败保留既有通知（同一孩子），下次成功加载再校正；
  - 没有喂养：取消通知，卡片显示空态；
  - 提醒关闭：不调度。
- 避免每次 render 重复调度：以 `childId + lastFeed.started_at` 为去重键（或等价机制）。
- 统计页切换孩子也应重排；同步逻辑放在育儿域共享层，而不是只放在今天页。

---

## 8. 迁移与兼容

- 新表迁移 `0021_care_events.sql`；`Down` 删除表。
- 没有历史护理事件需要回填：当前生产仓储是内存，重启即空；升级后从空表开始。
- 契约里 `ended_at = null` 的 ongoing 分支保留；票 09 完成后生产 UI 不再创建 ongoing，但 API / 统计 / fixture 仍可处理。
- 护理事件不进入 `reports` / `report_items` / 趋势 / 分析快照；不新增鉴权 / 账号表。
- 成员删除级联；补成员删除测试。

---

## 9. 怎么实施

1. **睡眠 UX（护理记录票 09）**：结束时间 + 时长、开始时间推导；生产不再创建 ongoing。先于 A1 App 接入。
2. **后端存储（care-events 票 01）**：`internal/care` 领域类型 / 校验 / 端口；`internal/storage/store_care.go`；migration 0021；storage 测试；`scripts/test-db.sh`。
3. **HTTP（care-events 票 02）**：OpenAPI、`internal/httpapi` 镜像、`cmd/hdd/serve_care.go`、fixtures、contract test、冻结端点集合。
4. **App（care-events 票 03）**：`api/careEvents.ts`、`HttpCareEventRepository`、provider 注入、codegen、RNTL。
5. **提醒与刷新（care-events 票 04）**：下拉刷新、切换孩子 / 冷启动重排、RNTL + mock notifications。
6. **验收（care-events 票 05）**：`go test ./...`、`go vet ./...`、`bash scripts/test-db.sh`、App typecheck / test / lint、Android 双设备 smoke、契约扫描、文档状态更新。

---

## 10. 怎么验收

### 10.1 自动化

- Go：`go test ./...`、`go vet ./...`；care 领域校验单测；storage 测试覆盖四类型、范围重叠、进行中睡眠、跨午夜、排序、级联、约束拒绝。
- DB：`bash scripts/test-db.sh` 全绿，含 migration 0021 与 `care_events` 约束。
- 契约：`internal/httpapi/contract_test.go` 端点集合、fixtures、Go / OpenAPI 双向对账；`npm run codegen` 无 diff。
- App：`cd apps/mobile && npm run typecheck && npm test && npm run lint -- --max-warnings=0`。
- 扫描：`rg 'recorded_by|device_id|caregiver|active_child|activeChild' apps/mobile/src internal cmd --glob '!*.test.*'` 只允许契约注释命中；护理事件不进入报告 / 趋势 / 分析导入。

### 10.2 Android 真机 smoke

- 隔离验收栈；两台设备（或两个 App 实例）指向同一后端。
- 设备 A 创建喂奶 / 尿布 / 睡眠 / 辅食；设备 B 今天页下拉刷新后看到同一批事件，时间线按 `member_id` 隔离。
- 设备 B 在 5 分钟内再记同孩子喂养，触发重复提示；「仍然添加」落两条；「返回修改」不落库。
- 设备 A 修改 / 删除；设备 B 下拉刷新后同步。
- 冷启动两台设备：事件仍在；当前孩子保持；提醒开关 / 间隔保持。
- 提醒：开启后切换孩子，AlarmManager 中的通知按新孩子的最新喂养重排；冷启动后不残留旧孩子的通知；最近 7 天无喂养时取消。
- 未配置 / 后端不可达：显示现有错误态，不静默使用内存数据。

### 10.3 文档 / 契约

- 更新 `docs/design/2026-11-23-care-event-contract.md`：HTTP 端点 / OpenAPI 落地状态。
- 更新 `docs/agents/contracts.md`：护理事件 API 的 mirror 行。
- 更新 `docs/adr/0019-care-log-navigation-child-context.md`：持久化实现状态与本 spec 链接。
- 更新 `.scratch/care-log/spec.md` §15：持久化已完成 / 剩余差异。
- `python3 scripts/check-docs.py` 全绿。

---

## 11. 风险与开放问题

- **单表 CHECK 复杂**：类型专属约束写在 DB 与领域层两处；用存储测试覆盖每种非法组合，避免只靠 HTTP 校验。
- **跨设备不是实时**：下拉刷新前 B 可能看到旧数据；重复提示仍以服务端读为准，但并发写入仍是 best-effort。
- **提醒跟随当前孩子**：切换孩子会移动提醒；如果真实使用需要每个孩子独立提醒，另开票。
- **无离线能力**：后端不可达时不能记录；与现有报告链路一致，不做本地队列。
- **无账号 / 无鉴权**：沿用 ADR-0006；跨公网前必须另评审。
- **睡眠 ongoing 契约保留**：票 09 后生产 UI 不再创建，但 API / 统计仍支持；不要把旧 fixture 当生产数据。

开放问题：无。睡眠 UX 的结束时间 + 时长字段、夜间分段与文案在护理记录票 09 内冻结。

---

## 12. 工单拆分

| 票 | 内容 | 依赖 |
| --- | --- | --- |
| [care-log 09](../care-log/issues/09-sleep-completion.md) | 睡眠改为结束后补记 | 无 |
| [care-log 10](../care-log/issues/10-cleanup.md) | 删 `/today-records` 占位 + 时间线真机滚动 smoke | 无 |
| [01](issues/01-care-events-storage.md) | `care_events` 表、领域校验、storage 测试 | 无 |
| [02](issues/02-care-events-http.md) | OpenAPI、Go handler、fixtures、contract tests | 01 |
| [03](issues/03-care-events-app-adapter.md) | App HTTP adapter、provider、下拉刷新、RNTL | 02、care-log 09 |
| [04](issues/04-reminder-resync.md) | 提醒切换孩子 / 冷启动重排 | 03 |
| [05](issues/05-acceptance.md) | 跨票验收、双设备 smoke、文档收尾 | 01–04、care-log 09 |

---

## 13. 关联

- 客户端契约：`docs/design/2026-11-23-care-event-contract.md`
- 导航与孩子上下文：`docs/adr/0019-care-log-navigation-child-context.md`
- LAN / 无账号：`docs/adr/0006-lan-first-no-auth.md`
- 调研与数据草案：`docs/reference/wechat-feeding-tracker.md` §7
- 前置工单：`.scratch/care-log/issues/01-care-event-contract.md`
- 已知差异：`.scratch/care-log/spec.md` §15
