# 09: 尿常规标准输出端到端

**What to build:** 尿常规 40 项端到端标准输出：干化学定性/定序、尿沉渣计数、镜检视野、尿钙/尿肌酐/微白蛋白等定量项。重点验证 `result_kind`（qualitative/ordinal/microscopy/quantitative）与 `/HP`、`/LP`、`个/uL` 的建模，不把伪单位塞进 `unit`。

**Blocked by:** 03 (凝血标准输出端到端)

**Status:** ready-for-agent

- [ ] 尿常规项都有标准名/代码或 local_code、result_kind、source/review 状态。
- [ ] 干化学项无 unit，`ref_kind` 为 qualitative/ordinal；镜检项用 `field=HPF/LPF`，不造 UCUM 伪单位。
- [ ] 沉渣计数 `个/uL` 归一到 `/uL` 或等价计数单位；定量项用 canonical unit。
- [ ] 已复核项 verified；未复核项 raw+unverified；`unit` 无 OCR 碎片。
- [ ] golden/App/趋势验收；定性结果不参与数值趋势。
