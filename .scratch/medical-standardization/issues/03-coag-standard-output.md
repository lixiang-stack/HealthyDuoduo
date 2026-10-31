# 03: 凝血标准输出端到端

**What to build:** 完成凝血 9 项的第一条完整标准输出路径：canonical UCUM、展示单位、原文单位/值、`unit_status`、`result_kind`、D-二聚体结果基准，以及 App/趋势展示。凝血试点已迁移 schema，本票只做输出、换算与端到端验收。

**Blocked by:** 01 (ReportItem 标准字段贯穿), 02 (单位换算层与 OCR 形分离)

**Status:** ready-for-agent

- [ ] PT/APTT/TT 输出 `s`/展示 `秒`；PTR/INR 为 ratio，单位 `1`。
- [ ] D-二聚体 canonical `μg/L` + `result_basis=FEU`，`ng/mL`/`mg/L` 按基准换算；FIB `g/L`，FDP `μg/mL`。
- [ ] 已复核项 `unit_status=verified`；未复核项 `unit=null` + `unverified`，`raw_value`/`raw_unit`/`raw_ref_range` 保留。
- [ ] App 显示展示单位与未验证标记；趋势只连接 verified 的 canonical unit。
- [ ] 凝血 golden 更新并逐项说明；contract/Go/App 测试全绿。
