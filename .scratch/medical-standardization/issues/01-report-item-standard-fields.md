# 01: ReportItem 标准字段贯穿

**What to build:** 打通标准字段的全链路透传，不改变现有识别/展示语义。报告指标项新增 `code`、`loinc`、`unit_display`、`unit_status`、`raw_value`、`raw_unit`、`raw_ref_range`、`ref_kind`、`result_kind`、`result_basis`；数据库、API、App 都能读到；旧 `value`/`unit` 行为保持不变。

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] API 返回新增字段；旧字段与旧 golden 不变。
- [ ] 数据库迁移为新增列，旧行可空/有默认值，读取不报错。
- [ ] App 优先显示 `unit_display`，缺失时回退 `unit`；详情可见 `raw_unit`/`unit_status`。
- [ ] Python/Go/TS 契约测试锁定字段集合；不引入标准换算或新判定。
