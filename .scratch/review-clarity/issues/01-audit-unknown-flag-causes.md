# 01: 调查指标项「未知」的成因

**What to build:** 基于真实数据产出一份「未知」指标项的成因审计：按缺数值、缺参考范围、参考范围不可解析/上下界写反、定性结果、影像测量、其他分类，统计 `member=aaa` 与现有 samples 的分布，给出代表样本、命中的代码路径，以及哪些 unknown 能用现有「修正」消除、哪些不可判定。本票不改产品行为。

**Blocked by:** None（可立即开始）。

**Status:** resolved

## 验收清单

- [x] 用 `member=aaa`（后端可访问时）与 `samples/expected/report/*.json` 统计 `flag=unknown` 的指标项，按成因分类并给出数量/占比。
- [x] 每个成因至少一个真实样本：报告类别 / 指标项名 / `value` / `ref_range` / `raw_text` / `low_confidence`；注明命中的代码路径（识别端 `_flag`、服务端 `ComputeFlag`、提取/参考范围拆分）。
- [x] 区分 unknown 与「低置信/待核对」：说明确认只改 `review_status`、修正才重算 flag；说明高置信 unknown 不会进入 `pendingItems`。
- [x] 结论：哪些 unknown 可由「修正」消除（如缺参考范围但纸面有范围、OCR 误读），哪些是数据本身不可判定（定性、无参考范围、影像测量），哪些是提取缺陷。
- [x] 产出报告归档到 `docs/design/2026-10-21-unknown-flag-audit.md`（或 `.scratch/review-clarity/`），含复现命令/查询；`member=aaa` 不可访问时用 samples 替代并注明边界。
- [x] 不改生产代码；不引入新依赖。

## 实现提示

- 参考 `recognizer/recognizer/postprocess.py` 的 `_flag` / `_num` 与 `internal/pipeline/flag.go` 的 `ComputeFlag`：数值非纯数、无参考范围、范围不可解析/上下界写反 → unknown。
- `samples/expected/report/cbc_01.json` 的「中性细胞数 4.19」是现成样本：`value=4.19`、`ref_range=null`、`flag=unknown`、`low_confidence=false`、`ocr_line_numbers=[169,170,171]`。
- `samples/expected/report/*.json` 覆盖多类别；`member=aaa` 的分布用 `GET /reports?member_id=` + 详情，或直接 SQL 只读查询。

## Comments

**审计完成（2026-10-21，分支 member-overview）**

- 报告归档：`docs/design/2026-10-21-unknown-flag-audit.md`。
- `member=aaa`（member_id=9，共享栈）：18 份 / 188 项 / 43 unknown（22.9%）；全部 success；未知项中低置信 2 项。
- samples：49 份 / 559 项 / 130 unknown（23.3%）；未知项中低置信 4 项。
- 成因：缺值 21/52（aaa/samples）、定性文本 10/36、缺参考范围 8/24、上下界写反 1/8、单侧比较 1/8、区间结果 2/2；参考范围不可解析 0/0；影像 0/0。
- 可被现有修正消除：11/43（25.6%）与 17/130（13.1%）。
- 代表样本已按原图 OCR 行框裁切核对（抗 Jo-1 `1.53`、淋巴细胞 `22.8%`、cbc_04 列错位）；掩码语料无原图处已注明边界。
- 未改生产代码，未引入依赖。
