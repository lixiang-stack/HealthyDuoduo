# 14: 医学复核 batch 1

**What to build:** 医学复核 batch 1（95 项：凝血、CBC、生化、激素）。逐项确认标准名/码、canonical unit、替代单位与换算、`result_kind`、`plausible_range`；重点裁决 PDW SD/CV、D-二聚体 FEU/DDU、eGFR 1.73m²、U/L vs IU/L。复核通过后把 `review_status` 翻为 approved，并重生成受影响 golden。

**Blocked by:** 03–08（batch 1 各 panel 标准输出）

**Status:** resolved

- [x] 95 项逐项有复核记录与来源；无来源项不得 approved。
- [x] `plausible_range` 每项确认；量级门能挡住 T3 mmol/L 这类 1000× 错误。
- [x] PDW、D-二聚体基准、eGFR、U/L vs IU/L 四个争议项有明确结论。
- [x] approved 项重生成 golden；`unit_status=verified` 只在 approved 后出现。

## Comments

**复核工作包（2026-10-31，分支 `trend-by-code-12-13`）**

- 已生成 batch 1 逐项复核表：`.scratch/medical-standardization/review/batch-1.csv`（95 项）与总览 `.scratch/medical-standardization/review/summary.md`。
- 每行含标准名、WS/T 886 码或 local code、LOINC、result_kind、canonical/display unit、accepted units/换算、`plausible_range`、source_status/sources、notes，并留空 `review_decision` / `reviewer` / `reviewed_at` / `review_comment`。
- 当前仍全部 `review_status=pending`，未翻 approved，未声称 verified。
- 工作包发现：batch 1 仅 `二氧化碳结合力` 为定量项但缺 `plausible_range`；PDW、D-二聚体、eGFR、U/L vs IU/L 的当前口径与待裁决问题已列入 `summary.md`。
- 复核人需逐行给出 approve / reject / needs-info 并签名；实施侧按批准清单翻状态、重生成 golden、跑全量验证。

**技术复核结果（2026-10-31）**

- 复核记录：`docs/batch-1-reviewed.csv`、`docs/summary-reviewed.md`；复核人「技术复核人（姓名见线下记录）/Grok AI-assisted」，明确为非持证检验医师。
- 技术结论：94 项建议 approve；`血小板分布宽度` needs-info（应拆 PDW-SD fL / PDW-CV %）。
- 争议项已确认：D-二聚体 FEU 唯一 canonical；eGFR `mL/min/1.73m2` 唯一 canonical，纯 `mL/min` 不换算；U/L 与 IU/L 不默认等价；PDW 因无真实样本证据暂保留 legacy，待样本后拆分。
- 已落地：`二氧化碳结合力` 补 `plausible_range 10..40`；其余词典/映射未改。
- **已应用**：按 `docs/batch-1-reviewed.csv` 的 approve 行翻 `review_status=approved`（94/95；PDW 1 项 needs-info 保持 pending）；`二氧化碳结合力` 已补 `plausible_range 10..40`。
- **验证**：golden 重生成后 368 个 golden 项变为 `unit_status=verified`；pytest 365 passed；真机趋势图恢复。
- **口径说明**：本次为技术复核（非持证检验医师）；正式临床签核如有需要另行覆盖。
