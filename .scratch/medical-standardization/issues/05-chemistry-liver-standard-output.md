# 05: 生化-肝功标准输出端到端

**What to build:** 肝功项目端到端标准输出：ALT/AST/GGT/ALP、AST/ALT、TP/ALB/GLO/A/G、TBIL/DBIL/IBIL、TBA、胆碱酯酶、前白蛋白、AFU、LAP、MAO、铜蓝蛋白。重点验证酶单位 U/L、胆红素 mg/dL→μmol/L、蛋白 g/dL→g/L。

**Blocked by:** 03 (凝血标准输出端到端)

**Status:** ready-for-agent

- [ ] 肝功项都有标准名/代码或 local_code、canonical unit、result_kind、source/review 状态。
- [ ] 胆红素 `mg/dL→μmol/L`、蛋白 `g/dL→g/L` 的 value/ref_range 同步换算；AST/ALT 与 A/G 保持 ratio。
- [ ] 酶类 U/L 与 IU/L 不默认等价；无权威换算时标 unverified，不静默改写。
- [ ] 已复核项 verified；未复核项 raw+unverified；`unit` 无 OCR 碎片。
- [ ] 肝功 golden/App/趋势验收。
