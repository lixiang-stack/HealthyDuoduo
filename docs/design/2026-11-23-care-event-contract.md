# 护理事件最小契约（前置）

**状态：** 采纳（2026-11-23，工单 01）。
**范围：** B3 今天时间线、统计、记录弹层与 5 分钟重复提示所需的**客户端可见**护理事件形状。
**不在本契约：** 完整表结构、迁移、服务端校验、HTTP handler、OCR、报告 / 分析链路。

## 为什么做

B3 的 04 / 06 / 07 都需要同一条护理事件数据：今天时间线要按时间列出事件，统计要按类型汇聚，记录弹层要创建 / 修改 / 删除，重复喂养提示要读最近一条喂养。若每张 UI 票各自发明字段，`member_id`、时间语义和类型专属字段会漂移。

本契约只冻结 UI 需要的最小面，并把「没有账号」写进类型：护理事件只归属 `member_id`，没有 `recorded_by`、`device_id`、`caregiver` 或全局 active child。完整护理事件表 / API 仍按 `docs/reference/wechat-feeding-tracker.md` §7 另立实现 spec；届时 HTTP 契约进入 `schemas/openapi.yaml`，本契约的 TypeScript 类型继续作为 App 的消费面。

## 契约源

- TypeScript：`apps/mobile/src/care/careEvents.ts`（唯一客户端形状）。
- 合成 fixture：`apps/mobile/src/care/careEventFixtures.ts`。
- 内存 adapter：`apps/mobile/src/care/careEventRepository.ts`。
- 行为测试：`apps/mobile/src/care/careEvents.test.ts`。

本票没有直接改 `schemas/openapi.yaml`：OpenAPI 的端点集合在 `internal/httpapi/contract_test.go` 中冻结，新增护理事件端点必须同时落 Go handler、存储与 fixture；那属于后续护理事件 API 实现 spec，不在本票范围。先以独立 TypeScript 契约 + fixture adapter 解除 04 / 06 / 07 的阻塞，正式 API 落地后再按 `docs/agents/contracts.md` 同步 OpenAPI / codegen。

## 共同字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `id` | `number` | 是 | 存储分配的稳定标识 |
| `member_id` | `number` | 是 | 事件归属的孩子成员；成人不是护理目标 |
| `type` | `'feed' \| 'diaper' \| 'sleep' \| 'solid'` | 是 | 判别字段 |
| `started_at` | ISO 8601 字符串 | 是 | 绝对时刻，必须带时区偏移；跨午夜不拆事件 |
| `ended_at` | ISO 8601 字符串 \| `null` | 是 | 进行中为 `null`；非空时不得早于 `started_at` |
| `note` | `string \| null` | 是 | 去除首尾空白；空备注统一为 `null` |
| `source` | `'parent'` | 是 | 固定「家长记录」；不标识具体记录人 |

## 类型专属字段

| 类型 | 字段 | 类型 | 说明 |
| --- | --- | --- | --- |
| `feed` | `method` | `'breast' \| 'formula'` | 喂养方式 |
| `feed` | `amount_ml` | `number \| null` | 瓶喂 / 奶粉奶量；亲喂可不填 |
| `feed` | `duration_min` | `number \| null` | 亲喂时长；只记时刻时为 `null` |
| `diaper` | `diaper_type` | `'wet' \| 'dirty' \| 'mixed'` | 尿 / 便 / 尿+便 |
| `sleep` | `duration_min` | `number \| null` | 进行中为 `null`；结束后为正数 |
| `solid` | `food` | `string` | 必填、非空食物名 |
| `solid` | `amount_g` | `number \| null` | 可选克数 |

## 不变量

1. `member_id` 必填，且只指向 `is_child` 成员；服务端在 API 实现时校验。
2. 不存、不读、不展示 `recorded_by`、`device_id`、`caregiver` 或全局 active child。
3. 金额 / 时长 / 克数存在时必须 `> 0`。
4. `ended_at` 非空时 `>= started_at`。
5. `solid.food` 去除首尾空白后非空；`note` 去除首尾空白后为空则存 `null`。
6. 时间一律存绝对时刻；展示与「今天」分组由客户端按本机时区计算。
7. 护理事件不进入 `reports` / `report_items` / 趋势 / 分析快照。

## 读写形状

TypeScript seam：

```ts
type CareEventQuery = { member_id: number; from: string; to: string };

type CareEventRepository = {
  list(query: CareEventQuery): Promise<CareEvent[]>;
  create(event: CareEventCreate): Promise<CareEvent>;
  update(eventId: number, event: CareEventUpdate): Promise<CareEvent>;
  remove(eventId: number): Promise<void>;
};
```

- `from` 含、`to` 不含，均为带时区偏移的绝对时刻。
- `list` 返回与 `[from, to)` **有重叠**的事件，按 `started_at` 倒序；同一时刻按 `id` 倒序。这样 23:50 开始、次日 06:20 结束的睡眠会同时出现在两天的范围读取中；统计按重叠时长自行切分，不丢跨午夜数据。
- `create` 的 `member_id` 在 payload 中显式给出；未来 HTTP 适配器把它映射到 `/members/{member_id}/care-events` 的路径参数。
- `update` 替换可编辑内容（类型、时间、类型专属字段、备注），不改变 `id`、`member_id`、`source`。
- `remove` 删除单条事件。

未来 HTTP 映射（实现 spec 冻结，当前不落 OpenAPI）：

| 操作 | 形状 |
| --- | --- |
| 读 | `GET /members/{member_id}/care-events?from=&to=` → `{ events: CareEvent[] }` |
| 建 | `POST /members/{member_id}/care-events` body = 去掉 `member_id` 的 `CareEventCreate` → `CareEvent` |
| 改 | `PATCH /care-events/{event_id}` body = `CareEventUpdate` → `CareEvent` |
| 删 | `DELETE /care-events/{event_id}` → 204 |

## Fixture 与 adapter

`CARE_EVENT_FIXTURES` 提供 5 条合成事件，覆盖：

- 四个类型各至少一条；
- 空备注（`note: null`）；
- 可选奶量 / 克数为 `null`；
- 跨午夜睡眠（`2026-11-22T23:50:00+08:00` → `2026-11-23T06:20:00+08:00`）。

`InMemoryCareEventRepository` 实现上面的 seam，供 04 / 06 / 07 在真实 API 落地前渲染与验收。它的 `create` 固定 `source: 'parent'`，`update` 保留身份字段，`remove` 对未知 id 抛错。

内存 adapter 只模拟读写 seam，不替代服务端校验；不变量 3–5（金额 / 时长 / 克数 > 0、`ended_at >= started_at`、`food` / `note` 去空白）由后续 HTTP / storage 实现负责并补测试。

## 消费方式

- **04 记录弹层**：用 `CareEventCreate` 构造保存 payload；保存后刷新当前孩子当天 `list`。
- **06 提醒**：只从 `list` 的最新喂养派生「距上次喂养」，不新增服务端字段；提醒仍默认关闭、本机。
- **07 统计**：用 `list` 的 4 类事件按天聚合；跨午夜事件按重叠时长切分，不伪造趋势。
- **05 重复喂养**：保存前查同一 `member_id` 最近一条 `type === 'feed'`；5 分钟窗口与「不是正在修改的同一条」由客户端判断。

## 验收

- `apps/mobile/src/care/careEvents.test.ts` 覆盖四类型 fixture、范围重叠、倒序、创建 / 修改 / 删除与身份字段保留。
- `npx tsc --noEmit` 与 `npm test` 全绿。
- `rg 'recorded_by|device_id|caregiver' apps/mobile/src/care --glob '!*.test.ts'` 无新增归属字段（fixture 测试保留这些词用于断言缺失）。
- 后续 API 落地时按 `docs/agents/contracts.md` 补 OpenAPI、Go mirror、fixture 与 App codegen。

## 关联

- 决策：`docs/adr/0019-care-log-navigation-child-context.md`
- 需求与验收：`.scratch/care-log/spec.md` §6
- 护理事件草案：`docs/reference/wechat-feeding-tracker.md` §7
- 网络边界：`docs/adr/0006-lan-first-no-auth.md`
- 工单：`.scratch/care-log/issues/01-care-event-contract.md`
