# 14: 医学复核 batch 1

**What to build:** 医学复核 batch 1（95 项：凝血、CBC、生化、激素）。逐项确认标准名/码、canonical unit、替代单位与换算、`result_kind`、`plausible_range`；重点裁决 PDW SD/CV、D-二聚体 FEU/DDU、eGFR 1.73m²、U/L vs IU/L。复核通过后把 `review_status` 翻为 approved，并重生成受影响 golden。

**Blocked by:** 03–08（batch 1 各 panel 标准输出）

**Status:** ready-for-human

- [ ] 95 项逐项有复核记录与来源；无来源项不得 approved。
- [ ] `plausible_range` 每项确认；量级门能挡住 T3 mmol/L 这类 1000× 错误。
- [ ] PDW、D-二聚体基准、eGFR、U/L vs IU/L 四个争议项有明确结论。
- [ ] approved 项重生成 golden；`unit_status=verified` 只在 approved 后出现。
