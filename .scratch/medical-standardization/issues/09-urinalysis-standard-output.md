# 09: 尿常规标准输出端到端

**What to build:** 尿常规 40 项端到端标准输出：干化学定性/定序、尿沉渣计数、镜检视野、尿钙/尿肌酐/微白蛋白等定量项。重点验证 `result_kind`（qualitative/ordinal/microscopy/quantitative）与 `/HP`、`/LP`、`个/uL` 的建模，不把伪单位塞进 `unit`。

**Blocked by:** 03 (凝血标准输出端到端)

**Status:** resolved

- [x] 尿常规项都有标准名/代码或 local_code、result_kind、source/review 状态。
- [x] 干化学项无 unit，`ref_kind` 为 qualitative/ordinal；镜检项用 `field=HPF/LPF`，不造 UCUM 伪单位。
- [x] 沉渣计数 `个/uL` 归一到 `/uL` 或等价计数单位；定量项用 canonical unit。
- [x] 已复核项 verified；未复核项 raw+unverified；`unit` 无 OCR 碎片。
- [x] golden/App/趋势验收；定性结果不参与数值趋势。

## Comments

**实现（2026-10-31）**

- 40 项迁移；干化学标准名/WS/T 码按 mapping，旧名保留 synonym；定性项 `result_kind=qualitative`、unit null、`ref_kind=qualitative`。
- 沉渣计数 canonical `/uL`/display `个/μL`；`个/uL`、`个/ul`、`/ul` 精确等值。机器计数项 `result_kind=quantitative`。
- 镜检项（`镜检*`、`尿透明管型计数`、`颗粒管型`、`尿粘液丝`）`result_kind=microscopy`、unit null；`/HP`、`/LP` 保留在 `raw_unit`，不造 UCUM 伪单位。
- pH/比重 canonical `1`；尿胆原 `umol/L`；尿钙/尿肌酐 `mmol/L`；微白蛋白 `mg/L`；微白蛋白/尿肌酐 `mg/mmol`。
- `flag_policy.yaml` 增加标准名键，ua_04 定性项 flag 保持 normal；golden status/item/flag 与迁移前一致。
