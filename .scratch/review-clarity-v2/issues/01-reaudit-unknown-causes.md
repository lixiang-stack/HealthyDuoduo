# 01: 重做 unknown 审计（数据去重 + 根因分层 + 原图核验）

**What to build:** 基于真实数据重做 `flag=unknown` 审计，修正旧审计的三处证据问题：`member=aaa` 与 `samples` 的重叠报告、把 OCR 漏识别误写成提取器配对、把 OCR 行合并误写成 `_decompose` 跨格合并。产出按根因分层的分布、可操作残量表与原图核验记录，作为票 03 的决策输入。本票不改生产行为。

**Blocked by:** None（可立即开始）。

**Status:** ready-for-agent

## 验收清单

- [ ] **去重与重叠**：列出 `member=aaa` 与 `samples/expected/report/*.json` 的重叠报告；至少确认 `cbc_01 = member=aaa 报告 139fde0e0b0c`，并确认 `ana_04` 是否为 aaa 报告 `c9ce457c37ec` 的掩码副本。按 `report_id` 去重后统计，或分栏展示并明确“非独立样本”；不得再把 `22.9% vs 23.3%` 当作两次独立验证。
- [ ] **根因分层**：每个 unknown 归入且只归入一类：`OCR 漏识别`（值/范围不在 OCR 期望输出中）、`OCR 行合并`（反向/粘连串已出现在 OCR 行文本中）、`提取器配对`（值/范围在 OCR 中但未归到该指标项）、`纸面无参考范围`（原图核验）、`不可比较`（定性文本 / 单侧比较 / 区间值）、`参考范围不可解析`、`其他`。注明首匹配规则，避免“缺值 + 范围写反”被同时算进两类。
- [ ] **每个根因至少一个真实样本**：报告短 ID / 指标项 / `value` / `ref_range` / `raw_text` / `low_confidence` / OCR 行号或 `txts` 下标 / 原图裁切证据。样本优先取 `member=aaa`；samples 只作补充。
- [ ] **ana_04 / 抗 Jo-1 归因**：用 `samples/expected/ocr/ana_04.json` 的 `txts[62..98]` 证明对应行没有 1.53 等值，归 `OCR 漏识别`；引用 `.scratch/recognizer-quality/issues/01-column-assign.md` 的评论（拍屏摩尔纹、值不在 OCR 输出、重拍是诚实路径）。不得写成 `_extract_items` 或 `_decompose` 配对缺陷。
- [ ] **cbc_04 归因**：用 `samples/expected/ocr/cbc_04.json` 的 `txts[46/50/98]` 证明 `8210--1`、`1/6.0 130--31`、`13.5--9.5 109/L` 已是 OCR 行文本，归 `OCR 行合并`；说明 `_decompose`（`recognizer/recognizer/postprocess.py:338-425`）只拆单格、不做跨格合并。`淋巴细胞数`（`value=null`、`ref=6111.1--3.2`）计入同一缺陷家族；该家族至少 7 项，不是旧文档写的 6 项。
- [ ] **“有数值 + 缺参考范围”原图核验**：旧审计 aaa 8 / samples 24。建议 aaa 全部 8 项 + samples 至少 8 项按原图/行框核对，拆成“纸面无范围 / OCR 漏范围 / 无原图未核实”。不得仅凭 `raw_text` 没有范围 token 就判“不可操作”——`raw_text` 是 OCR 产物，OCR 漏了范围时它自然没有 token。
- [ ] **可操作残量表**：按根因给出 `unknown 总数 / 可被现有修正消除 / 其中低置信未核对 / 推荐动作（修 OCR、修提取器、人工补录、不处理）`；明确使用的判定与 `ComputeFlag` 同口径：参考范围只接受区间或单侧比较，标量范围不接受；`value` 必须可数值化；上下界写反仍是 unknown，需要同时改值/范围。
- [ ] **产出**：审计报告 `.scratch/review-clarity-v2/audit.md`；复现脚本 `.scratch/review-clarity-v2/audit.py`，含运行命令、输入边界与期望输出；脚本自带 `--self-test`，至少锚定 `ana_04`（OCR 漏识别）与 `cbc_04`（OCR 行合并）两个样例。不引入新依赖。
- [ ] 不改生产代码（`recognizer/recognizer/postprocess.py`、`internal/`、`apps/mobile/src` 均不改）；旧审计数字只作对照，逐项标“已复核 / 已推翻 / 未核实”。

## 实现提示

- `member=aaa`：共享栈 `http://127.0.0.1:8080/api/v1`，`member_id=9`；摘要用 `GET /reports?member_id=9`，详情用 `GET /reports/{id}`；原图用 `GET /images/{id}`，按 `items[].ocr_lines[].box` 裁切。不可访问时用 samples 替代并注明边界。
- samples：`samples/expected/report/*.json` 是期望报告，`samples/expected/ocr/*.json` 是 OCR 期望输出（`txts` / `boxes` / `scores`）；`txts` 下标从 0 开始。
- `ComputeFlag`：`internal/pipeline/flag.go`；参考范围支持 `a-b` / `a--b` / `a~b` 与 `<v / ≤v / >v / ≥v`，标量范围不解析；`value` 为文本、`<2.00`、`70-80`、`null`、范围 `null`/不可解析/上下界写反 → unknown。
- `_decompose`：`recognizer/recognizer/postprocess.py:338-425`；它只对单格文本做拆分。相邻栏的序号进入范围串，发生在 OCR 行/框组装，不在这个函数里。
- 旧审计的 `samples` 49 份包含 `cbc_01`，而 `cbc_01` 就是 aaa 的报告；`ana_04` 的 `raw_text` 与 aaa 免疫单一致。先列重叠，再算比例。
- 不要把旧审计的 `17/130`、`11/43`、C2 `11/15` 直接抄进新报告；它们是待复核的旧数字。

## Comments
