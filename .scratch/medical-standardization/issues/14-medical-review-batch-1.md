# 14: 医学复核 batch 1

**What to build:** 医学复核 batch 1（95 项：凝血、CBC、生化、激素）。逐项确认标准名/码、canonical unit、替代单位与换算、`result_kind`、`plausible_range`；重点裁决 PDW SD/CV、D-二聚体 FEU/DDU、eGFR 1.73m²、U/L vs IU/L。复核通过后把 `review_status` 翻为 approved，并重生成受影响 golden。

**Blocked by:** 03–08（batch 1 各 panel 标准输出）

**Status:** ready-for-human

- [ ] 95 项逐项有复核记录与来源；无来源项不得 approved。
- [ ] `plausible_range` 每项确认；量级门能挡住 T3 mmol/L 这类 1000× 错误。
- [ ] PDW、D-二聚体基准、eGFR、U/L vs IU/L 四个争议项有明确结论。
- [ ] approved 项重生成 golden；`unit_status=verified` 只在 approved 后出现。

## Comments

**复核工作包（2026-10-31，分支 `trend-by-code-12-13`）**

- 已生成 batch 1 逐项复核表：`.scratch/medical-standardization/review/batch-1.csv`（95 项）与总览 `.scratch/medical-standardization/review/summary.md`。
- 每行含标准名、WS/T 886 码或 local code、LOINC、result_kind、canonical/display unit、accepted units/换算、`plausible_range`、source_status/sources、notes，并留空 `review_decision` / `reviewer` / `reviewed_at` / `review_comment`。
- 当前仍全部 `review_status=pending`，未翻 approved，未声称 verified。
- 工作包发现：batch 1 仅 `二氧化碳结合力` 为定量项但缺 `plausible_range`；PDW、D-二聚体、eGFR、U/L vs IU/L 的当前口径与待裁决问题已列入 `summary.md`。
- 复核人需逐行给出 approve / reject / needs-info 并签名；实施侧按批准清单翻状态、重生成 golden、跑全量验证。
