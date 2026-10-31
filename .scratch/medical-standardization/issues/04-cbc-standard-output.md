# 04: CBC 标准输出端到端

**What to build:** CBC 31 项端到端标准输出：WBC/RBC/PLT 计数、HGB/MCV/MCH/MCHC、HCT、RDW、PDW、MPV、PCT、P-LCR、白细胞分类、NRBC、ESR、CRP/hs-CRP/SAA。重点验证 HCT `%→L/L`、RDW-CV/SD 区分、PDW 的 SD/CV 定义决策。

**Blocked by:** 03 (凝血标准输出端到端)

**Status:** ready-for-agent

- [ ] 31 项都有标准名/代码或 local_code、canonical unit、result_kind、source/review 状态。
- [ ] HCT 的 `%` 按 ×0.01 换算到 `L/L`，value 与 ref_range 同时换算；WBC/RBC/PLT 等值替身正确归一。
- [ ] RDW-CV 与 RDW-SD 不混用；PDW 按已确认的 SD/CV 方案拆分或标 local，不把 fL/% 当换算。
- [ ] 已复核项 verified；未复核项 raw+unverified；`unit` 无 OCR 碎片。
- [ ] CBC golden/App/趋势验收；分类计数与百分比量纲正确。
