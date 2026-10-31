# 04: CBC 标准输出端到端

**What to build:** CBC 31 项端到端标准输出：WBC/RBC/PLT 计数、HGB/MCV/MCH/MCHC、HCT、RDW、PDW、MPV、PCT、P-LCR、白细胞分类、NRBC、ESR、CRP/hs-CRP/SAA。重点验证 HCT `%→L/L`、RDW-CV/SD 区分、PDW 的 SD/CV 定义决策。

**Blocked by:** 03 (凝血标准输出端到端)

**Status:** resolved

- [x] 31 项都有标准名/代码或 local_code、canonical unit、result_kind、source/review 状态。
- [x] HCT 的 `%` 按 ×0.01 换算到 `L/L`，value 与 ref_range 同时换算；WBC/RBC/PLT 等值替身正确归一。
- [x] RDW-CV 与 RDW-SD 不混用；PDW 按已确认的 SD/CV 方案拆分或标 local，不把 fL/% 当换算。
- [x] 已复核项 verified；未复核项 raw+unverified；`unit` 无 OCR 碎片。
- [x] CBC golden/App/趋势验收；分类计数与百分比量纲正确。

## Comments

**实现（2026-10-31）**

- 31 项迁到标准 schema；标准名按 `mapping.csv`，旧名移入 `synonyms` 保召回。
- HCT `%→L/L` ×0.01（value/ref 同步）；WBC/RBC/PLT 计数 `10^3/uL`/`10^6/uL` 等值归一；HGB/MCHC `g/dL→g/L`、PCT `L/L→%` 按 pending 系数。
- RDW-CV `%` 与 RDW-SD `fL` 保持独立；PDW 暂按单据 `fL` 标 local（SD 定义，`LOCAL-CBC-PDW`），不把 `%` 当换算，留给票 14 裁决 SD/CV 拆分。
- golden：9 份 CBC 的 HCT 值与参考范围换算；RDW-CV/SD/PDW 的错误印形按约定置 null + unverified；status/item 数/flag 未变。
