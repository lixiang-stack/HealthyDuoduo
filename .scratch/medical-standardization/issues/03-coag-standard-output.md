# 03: 凝血标准输出端到端

**What to build:** 完成凝血 9 项的第一条完整标准输出路径：canonical UCUM、展示单位、原文单位/值、`unit_status`、`result_kind`、D-二聚体结果基准，以及 App/趋势展示。凝血试点已迁移 schema，本票只做输出、换算与端到端验收。

**Blocked by:** 01 (ReportItem 标准字段贯穿), 02 (单位换算层与 OCR 形分离)

**Status:** resolved

- [x] PT/APTT/TT 输出 `s`/展示 `秒`；PTR/INR 为 ratio，单位 `1`。
- [x] D-二聚体 canonical `μg/L` + `result_basis=FEU`，`ng/mL`/`mg/L` 按基准换算；FIB `g/L`，FDP `μg/mL`。
- [x] `accepted_units` 解析为类型化对象；linear 换算带 `from`/`to`/`factor`/`source`，方向 = accepted × factor = canonical。
- [x] 已复核项 `unit_status=verified`；未复核项 `unit=null` + `unverified`，`raw_value`/`raw_unit`/`raw_ref_range` 保留。
- [x] App 显示展示单位与未验证标记；趋势只连接 verified 的 canonical unit。
- [x] 凝血 golden 更新并逐项说明；contract/Go/App 测试全绿。

## Comments

**实现（2026-10-31，分支 `panels-standard-output`）**

- 输出层：`postprocess._standardize_item()` 建立统一标准输出路径；凝血 golden 输出 PT/APTT/TT `unit=s`/`unit_display=秒`、PTR/INR `unit=1`、D-二聚体 `unit=ug/L`/`unit_display=μg/L`/`result_basis=FEU`、FIB `g/L`、FDP `ug/mL`，`raw_*` 全保留。
- 换算：`ng/mL` 等值、`mg/L` ×1000；Fibonacci 线性换算方向校验沿用票 02 loader。
- 待复核口径：经用户确认，`review_status=pending` 时仍输出可靠 canonical，但 `unit_status=unverified`；只有无法解析/量级门拒绝才 `unit=null` + `low_confidence=true`（票面“未复核项 null”按此口径执行）。当前无 approved 项，故 golden 无 `verified`。
- 趋势：CLI/HTTP 趋势查询已排除 `unit_status=unverified` 点（null 旧行保留）；按 code 连接与跨面板/`--all` 由票 12 继续。
- 验证：`pytest` 370 passed / 28 skipped；`go test ./...`；`ruff`；golden status/item 数/flag 与迁移前一致。
