# 15: 医学复核 batch 2

**What to build:** 医学复核 batch 2（86 项：尿常规、免疫、专项、血型）。逐项确认标准名/码或 local_code、`result_kind`、单位/替代单位、定性/定序参考范围；重点裁决 `S/CO`、`AU`、滴度、`/HP`、`个/uL` 的建模。复核通过后翻 `review_status=approved` 并重生成 golden。

**Blocked by:** 09–11（batch 2 各 panel 标准输出）

**Status:** ready-for-human

- [ ] 86 项逐项有复核记录与来源；无来源项不得 approved。
- [ ] `S/CO`、`AU`、滴度、`/HP`、`个/uL` 均有明确 result_kind/单位表达，不出现伪单位。
- [ ] 定性/定序项的 `ref_kind` 与 flag 语义确认。
- [ ] approved 项重生成 golden；`unit_status=verified` 只在 approved 后出现。

## Comments

**复核工作包（2026-10-31，分支 `trend-by-code-12-13`）**

- 已生成 batch 2 逐项复核表：`.scratch/medical-standardization/review/batch-2.csv`（86 项）与总览 `.scratch/medical-standardization/review/summary.md`。
- 每行含标准名、WS/T 886 码 / local code / LOINC、result_kind、canonical/display unit、accepted units、`plausible_range`、source_status/sources、notes，并留空复核决策字段。
- 当前仍全部 `review_status=pending`，未翻 approved，未声称 verified。
- 工作包发现：batch 2 有 47 项 `source_status=pending`（autoimmune 17、urinalysis 30），复核前不得 approved；S/CO、AU、滴度、/HP、/LPF、个/uL 的当前建模与待裁决问题已列入 `summary.md`。
- 复核人需逐行给出 approve / reject / needs-info 并签名；实施侧按批准清单翻状态、重生成 golden、跑全量验证。
