# 01: ReportItem 标准字段贯穿

**What to build:** 打通标准字段的全链路透传，不改变现有识别/展示语义。报告指标项新增 `code`、`loinc`、`unit_display`、`unit_status`、`raw_value`、`raw_unit`、`raw_ref_range`、`ref_kind`、`result_kind`、`result_basis`；数据库、API、App 都能读到；旧 `value`/`unit` 行为保持不变。

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] API 返回新增字段；旧字段与旧 golden 不变。
- [x] 数据库迁移为新增列，旧行可空/有默认值，读取不报错。
- [x] App 优先显示 `unit_display`，缺失时回退 `unit`；详情可见 `raw_unit`/`unit_status`。
- [x] Python/Go/TS 契约测试锁定字段集合；不引入标准换算或新判定。

## Comments

**实现（2026-10-31，分支 `report-item-standard-fields`）**

- 契约：Python `ReportItem` / JSON Schema / OpenAPI / Go `contract.ReportItem` / HTTP `ReportItem` 新增 10 个标准层字段；null 时序列化省略，旧报告与旧 golden 逐字节不变。
- 语义：本期只打通字段透传，识别端不产出新字段（`unit_status` 等由票 03+ 逐 panel 启用）；`value`/`unit`/`ref_range` 行为不变。
- 存储：迁移 `0012` 新增 10 个可空 TEXT 列；`WriteReport`/详情/核对读取链路全量 scan + 写回。
- App：`displayItemUnit` 优先 `unit_display`、回退 `unit`（详情/核对卡与原图页）；核对卡展示「原文单位」差异与「单位未验证/已核验」。
- 验证：`pytest` 344 passed / 28 skipped（golden 未变）；`go test ./...` + 真库 storage 冒烟全绿；mobile 234 passed、`tsc --noEmit`；`check-docs.py`。
