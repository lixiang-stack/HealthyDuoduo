# 01: 护理事件最小数据契约（前置）

**What to build:** 为 B3 的今天 / 统计 / 记录弹层 / 重复提示定义**客户端可见的护理事件最小契约**：事件必须有 `member_id`、类型、发生时间、类型专属字段与 `source=parent`；提供按成员 + 时间范围读取、创建、修改、删除的接口形状。该契约是 04 / 06 / 07 的前置，不包含完整表结构、迁移、OCR 或报告链路。

**Blocked by:** None（可立即开始；完整数据模型另立 spec）。

**Status:** resolved

## 验收清单

- [x] `CareEvent` 是判别联合：`feed` / `diaper` / `sleep` / `solid`；公共字段含 `id`、`member_id`、`type`、`started_at`、`ended_at`、`note`、`source`。
- [x] `feed` 含 `method`（breast / formula）、可选 `amount_ml`、可选 `duration_min`；`diaper` 含 `diaper_type`；`sleep` 含可选 `duration_min`；`solid` 含 `food`、可选 `amount_g`。
- [x] `member_id` 必填；不出现 `recorded_by`、`device_id`、`caregiver`、全局 active child 字段。
- [x] 读接口能按 `member_id` + `from` / `to` 返回当天 / 范围事件，倒序；写接口能创建、修改、删除；形状足以驱动今天时间线、统计和 5 分钟重复提示。
- [x] 契约写入 OpenAPI / TypeScript 类型或独立 spec；提供合成 fixture，覆盖四个类型与边界值（空备注、可选奶量、跨午夜）。
- [x] 本票不实现 UI；04 / 06 / 07 用 adapter / fixture 先行，正式 API 落地后再替换。
- [x] 不新增账号、鉴权、云同步；沿用 `docs/adr/0006-lan-first-no-auth.md`。

## 实现提示

- 字段草案见 `docs/reference/wechat-feeding-tracker.md` §7；本票只冻结 B3 UI 需要的最小面，不等同于完整护理事件 spec。
- 先把 `source: 'parent'` 固定为「家长记录」，不要为记录人归属留字段。
- 读模型可以先用 `GET /members/{member_id}/care-events?from=&to=` 的资源形状；最终路径在实现 spec 中按现有 OpenAPI 风格调整。
- 每类事件都要有 fixture，统计页的四指标（喂养 / 奶量 / 睡眠 / 尿布）才能在没有真实数据时验收。

## Comments

**拆分依据（2026-11-23）：** `../spec.md` §9.8 明确护理事件表 / API 另立 spec；本票是 04 / 06 / 07 的显式前置，避免 UI 票各自发明数据形状。完整表 / 迁移 / 契约冻结不在本票内。

**实现决定（2026-11-23）：** 先落独立 TypeScript 契约 + fixture adapter，不改 `schemas/openapi.yaml`。OpenAPI 端点集合在 `internal/httpapi/contract_test.go` 冻结，护理事件端点需要 Go handler / 存储 / fixture 同步；那属于后续 API spec。契约与验收见 `docs/design/2026-11-23-care-event-contract.md`。
