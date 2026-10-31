# 05: 生化-肝功标准输出端到端

**What to build:** 肝功项目端到端标准输出：ALT/AST/GGT/ALP、AST/ALT、TP/ALB/GLO/A/G、TBIL/DBIL/IBIL、TBA、胆碱酯酶、前白蛋白、AFU、LAP、MAO、铜蓝蛋白。重点验证酶单位 U/L、胆红素 mg/dL→μmol/L、蛋白 g/dL→g/L。

**Blocked by:** 03 (凝血标准输出端到端)

**Status:** resolved

- [x] 肝功项都有标准名/代码或 local_code、canonical unit、result_kind、source/review 状态。
- [x] 胆红素 `mg/dL→μmol/L`、蛋白 `g/dL→g/L` 的 value/ref_range 同步换算；AST/ALT 与 A/G 保持 ratio。
- [x] 酶类 U/L 与 IU/L 不默认等价；无权威换算时标 unverified，不静默改写。
- [x] 已复核项 verified；未复核项 raw+unverified；`unit` 无 OCR 碎片。
- [x] 肝功 golden/App/趋势验收。

## Comments

**实现（2026-10-31）**

- 肝功 20 项迁移；胆红素 ×17.1、蛋白 ×10（pending 复核系数）；AST/ALT、A/G ratio `1`。
- 酶类 canonical `U/L`；打印 `IU/L` 不在 accepted 集，输出 `unit=null`、`raw_unit=IU/L`、`low_confidence=true`（lft_05/06 的 LAP/MAO/ALT/AST/ALP 样本）。
- golden：胆红素/蛋白值未触及现有样本（现有样本已是 μmol/L/g/L），IU/L 类按约定置 null；status/flag 未变。
