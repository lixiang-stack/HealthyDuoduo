# 06: 生化-肾功/尿酸/电解质标准输出端到端

**What to build:** 肾功、尿酸与电解质端到端标准输出：尿素、肌酐、尿酸、二氧化碳结合力、eGFR、胱抑素C。重点验证尿酸 `mmol/L→μmol/L`、eGFR `mL/min/1.73m²`、CO2CP 命名与量纲。

**Blocked by:** 03 (凝血标准输出端到端)

**Status:** resolved

- [x] 肾功/尿酸/电解质项都有标准名/代码或 local_code、canonical unit、result_kind、source/review 状态。
- [x] 尿酸 canonical `μmol/L`，`mmol/L` 按 ×1000 换算；0.246 mmol/L → 246 μmol/L。
- [x] eGFR canonical `mL/min/1.73m²`；`mL/min` 不静默接受，缺 BSA 归一标 unverified。
- [x] 二氧化碳按 CO2CP 建模，不误用血气 pCO2。
- [x] 已复核项 verified；未复核项 raw+unverified；golden/App/趋势验收。

## Comments

**实现（2026-10-31）**

- 尿酸 canonical `umol/L`；`mmol/L` ×1000、`mg/dL` ×59.48（pending）→ value/ref 同步换算；lft_02 `0.246 mmol/L` 输出 `246`、ref `100-420`，`raw_*` 保留。
- eGFR canonical `mL/min/1.73m2`/display `mL/min/1.73m²`；`mL/min` 登记 `conversion:none`，输出 null + unverified。
- `二氧化碳` 标准名改 `二氧化碳结合力`（local_code `LOCAL-CHEM-CO2CP`），旧名保留 synonym；不按血气 pCO2 建模。
- 肌酐/尿素 mg/dL 系数已登记；现有 golden 已是 μmol/L/mmol/L，无值变更。
