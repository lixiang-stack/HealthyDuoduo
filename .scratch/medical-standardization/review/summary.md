# 医学复核工作包（票 14/15）

本文件由 `recognizer/tools/review_packet.py` 生成。技术复核结果已应用：
133 项 `review_status=approved`，48 项保持 pending（PDW 1 + needs-info 47）。
`unit_status=verified` 只在 approved 且单位解析成功时出现。

复核记录：`docs/batch-1-reviewed.csv`、`docs/batch-2-reviewed.csv`、
`docs/summary-reviewed.md`。复核人为非持证技术复核，正式临床签核如有需要另行覆盖。

## 填写方式

1. 在 `batch-1.csv` / `batch-2.csv` 每行填写 `review_decision`：`approve` / `reject` / `needs-info`。
2. `approve` 必须同时填写 `reviewer`、`reviewed_at`，并确认 `sources` 非空；
   定量项还必须有 `plausible_range`。
3. 有异议或需补充来源的行填 `review_comment`，不要直接改词典。
4. 复核完成后由实施侧按批准清单翻 `review_status`、重生成 golden，并跑票 13 的全量验证。

## Batch 1（95 项）

| panel | 项数 | 缺 plausible_range（定量） | source_status=pending | 仅 LOINC | 无任何码 |
| --- | ---: | ---: | ---: | ---: | ---: |
| cbc | 31 | 0 | 0 | 18 | 0 |
| coag | 9 | 0 | 0 | 2 | 0 |
| electrolyte | 1 | 0 | 0 | 0 | 0 |
| glucose | 9 | 0 | 0 | 4 | 0 |
| kidney | 4 | 0 | 0 | 1 | 0 |
| lipid | 5 | 0 | 0 | 1 | 0 |
| liver | 20 | 0 | 0 | 7 | 0 |
| sex_hormone | 2 | 0 | 0 | 0 | 0 |
| thyroid | 9 | 0 | 0 | 0 | 0 |
| uric_acid | 1 | 0 | 0 | 0 | 0 |
| vitanemia | 4 | 0 | 0 | 0 | 0 |

## Batch 2（86 项）

| panel | 项数 | 缺 plausible_range（定量） | source_status=pending | 仅 LOINC | 无任何码 |
| --- | ---: | ---: | ---: | ---: | ---: |
| autoimmune | 26 | 0 | 17 | 0 | 0 |
| infection | 8 | 0 | 0 | 0 | 0 |
| other | 12 | 0 | 0 | 0 | 0 |
| urinalysis | 40 | 0 | 30 | 0 | 0 |

## 争议项（需明确结论）

### PDW

技术结论：拆成 PDW-SD（fL）与 PDW-CV（%）；当前无后缀条目保留为 local 历史兼容。
实施约束：现有样本只有无后缀 PDW（fL），没有 PDW-SD/CV 真实印形证据；
按 corpus gate 暂不注册新条目，待真实样本后拆分并重新复核。

### D-二聚体

当前：canonical `ug/L`，`result_basis=FEU`；`ng/mL FEU` 等值、`mg/L FEU` ×1000，DDU 不换算。
待裁决：FEU 基准是否作为唯一 canonical；DDU 报告是否保留 `unit=null` + `unverified`。

### 二氧化碳结合力

技术结论：补充 `plausible_range 10..40`（成人常见参考约 22–29，放宽以挡数量级错误）；
LOINC 2028-9 为 total CO2，实际测 CO2CP，保持 notes 标注。

### eGFR

当前：canonical `mL/min/1.73m2`；`mL/min` → `conversion: none`，输出 `unit=null` + `unverified`。
待裁决：是否接受 `mL/min/1.73m²` 为唯一 canonical；`mL/min` 是否永不换算。

### U/L vs IU/L

当前：酶类 canonical `U/L`，`IU/L` 不默认等值；无权威换算时保持 `unverified`。
待裁决：是否确认两者不可等价；若某方法学有权威换算，需逐项给出 source。

### Batch 2 伪单位建模

- `S/CO`：当前按 `ratio` + `unit=1` 处理；待确认是否保留 `S/CO` 仅作 `raw_unit`。
- `AU`：当前按 `arbitrary` + `[arb'U]`；待确认各免疫项的 arbitrary 语义。
- 滴度：当前按 `titer`；待确认条码/稀释度是否进 `value` 还是 `raw_value`。
- `/HP`、`/LPF`：当前按 `microscopy` + `field: HPF|LPF`；待确认字段表达。
- `个/uL`：当前 canonical `/[uL]` 计数；待确认与 `/HP` 不混用。

## 复核完成后的实施清单

- 按批准清单把对应 `review_status` 翻 `approved`；未批准的保持 `pending`。
- 定量项必须有 `plausible_range`；无来源项不得 approved。
- 重生成受影响 golden，跑 `pytest` / `go test ./...` / mobile / `e2e.py verify --golden`。
- 在票 14/15 记录复核人、日期、批准/拒绝数与争议项结论。
