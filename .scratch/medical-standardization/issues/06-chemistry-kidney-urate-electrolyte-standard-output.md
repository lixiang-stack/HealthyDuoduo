# 06: 生化-肾功/尿酸/电解质标准输出端到端

**What to build:** 肾功、尿酸与电解质端到端标准输出：尿素、肌酐、尿酸、二氧化碳结合力、eGFR、胱抑素C。重点验证尿酸 `mmol/L→μmol/L`、eGFR `mL/min/1.73m²`、CO2CP 命名与量纲。

**Blocked by:** 03 (凝血标准输出端到端)

**Status:** ready-for-agent

- [ ] 肾功/尿酸/电解质项都有标准名/代码或 local_code、canonical unit、result_kind、source/review 状态。
- [ ] 尿酸 canonical `μmol/L`，`mmol/L` 按 ×1000 换算；0.246 mmol/L → 246 μmol/L。
- [ ] eGFR canonical `mL/min/1.73m²`；`mL/min` 不静默接受，缺 BSA 归一标 unverified。
- [ ] 二氧化碳按 CO2CP 建模，不误用血气 pCO2。
- [ ] 已复核项 verified；未复核项 raw+unverified；golden/App/趋势验收。
