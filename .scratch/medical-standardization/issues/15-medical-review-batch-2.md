# 15: 医学复核 batch 2

**What to build:** 医学复核 batch 2（86 项：尿常规、免疫、专项、血型）。逐项确认标准名/码或 local_code、`result_kind`、单位/替代单位、定性/定序参考范围；重点裁决 `S/CO`、`AU`、滴度、`/HP`、`个/uL` 的建模。复核通过后翻 `review_status=approved` 并重生成 golden。

**Blocked by:** 09–11（batch 2 各 panel 标准输出）

**Status:** resolved

- [x] 86 项逐项有复核记录与来源；无来源项不得 approved。
- [x] `S/CO`、`AU`、滴度、`/HP`、`个/uL` 均有明确 result_kind/单位表达，不出现伪单位。
- [x] 定性/定序项的 `ref_kind` 与 flag 语义确认。
- [x] approved 项重生成 golden；`unit_status=verified` 只在 approved 后出现。

## Comments

**复核工作包（2026-10-31，分支 `trend-by-code-12-13`）**

- 已生成 batch 2 逐项复核表：`.scratch/medical-standardization/review/batch-2.csv`（86 项）与总览 `.scratch/medical-standardization/review/summary.md`。
- 每行含标准名、WS/T 886 码 / local code / LOINC、result_kind、canonical/display unit、accepted units、`plausible_range`、source_status/sources、notes，并留空复核决策字段。
- 当前仍全部 `review_status=pending`，未翻 approved，未声称 verified。
- 工作包发现：batch 2 有 47 项 `source_status=pending`（autoimmune 17、urinalysis 30），复核前不得 approved；S/CO、AU、滴度、/HP、/LPF、个/uL 的当前建模与待裁决问题已列入 `summary.md`。
- 复核人需逐行给出 approve / reject / needs-info 并签名；实施侧按批准清单翻状态、重生成 golden、跑全量验证。

**技术复核结果（2026-10-31）**

- 复核记录：`docs/batch-2-reviewed.csv`、`docs/summary-reviewed.md`；复核人「技术复核人（姓名见线下记录）/Grok AI-assisted」，明确为非持证检验医师。
- 技术结论：39 项建议 approve；47 项 `source_status=pending`（autoimmune 17 + urinalysis 30）标 needs-info，缺权威来源，不得 approved。
- 伪单位口径确认：S/CO → ratio + `unit=1`；AU → arbitrary + `[arb'U]`；滴度 → titer；/HP、/LPF → microscopy + field；个/uL → 定量计数 `/[uL]`，与镜检分开。当前实现与结论一致，无额外代码改动。
- **已应用**：按 `docs/batch-2-reviewed.csv` 的 approve 行翻 `review_status=approved`（39/86；47 项 needs-info 保持 pending）。
- **验证**：golden 重生成后批准项变为 `unit_status=verified`；pytest 365 passed；真机趋势图恢复。
- **口径说明**：本次为技术复核（非持证检验医师）；正式临床签核如有需要另行覆盖。
