# 01: 列归属改中点分区（左对齐长名称不再整行漏抽）+ 免疫别名缺口

**What to build:** 修复真实报告「指标项成片缺失」：`_extract_items` 的列归属原为「最近表头列中心且偏离 ≤ 180px」；名称列左对齐，格中心随文本长度向右漂移，长名称格被整格丢弃 → 整行漏抽（实测免疫单 16 项只出 5 项、自免肝单 10 项直接判 failed）。改为最近列中心（中点分区，无固定偏离上限），并把两张真实单冻结为掩码语料回归。

**Status:** resolved

- [x] 列归属改中点分区；删除 `_ASSIGN_TOLERANCE_PX` 固定容差
- [x] 两处别名缺口按样本证据补齐（抗Sc1-70抗体 / ANA(IIF) 空格形）
- [x] 掩码语料 ana_04/ana_05（真实单冻结、无原图）：报告 golden 回归锁定版式；别名证据门禁通过
- [x] 内环：pytest 162 通过（+2 新语料）；本机 26 语料 check 无新增漂移；VL score 与改前逐字节一致
- [x] 外环：snapshot → refresh（16 reparse）→ verify → seed → verify --golden → clean 全过；共享库 5/16 份报告增数据、0 份变差

## Comments

**实现（分支 `column-assign`，worktree `.worktree/column-assign`，单 commit，push/PR 待用户）**

- **规则**（`recognizer/postprocess.py`）：`_extract_items` 的「1. 逐格归 (角色, 块)」从「最近列中心且 `dx ≤ _ASSIGN_TOLERANCE_PX(180)`，超限整格丢弃」改为「最近列中心（中点分区，`best_d` 上界无穷）」，常量删除。根因数据：免疫单 11 个名称格中心距名称列头 190–397px（「抗Ribosomal P抗体(Ribosomal P)」cx=631 vs 列头 233），全部被静默丢弃、`_match_name` 根本没被调用（词典条目本就齐全）。丢弃集合是「按文本长度漂移的量」，故固定容差在同一版式上系统性饿死长名称——非单张特例。
- **别名**（`immunology_dict.yaml`）：`抗Scl-70抗体` 补 `抗Sc1-70抗体(Sc1-70)`（OCR 把 l 读成 1）、`ANA(IIF)` 补 `ANA(IIF) (ANA(IIF))`（括号前空格形）。两条均由真实单见证，经 `test_every_alias_has_corpus_evidence` 门禁。
- **掩码语料**（`samples/expected/ocr/report/ana_04|05.json`）：从共享栈真实报告冻结 OCR 后经 `local_golden.py mask` 脱敏（人工复核 kept 行：无姓名/证件/机构/时间；PII 扫描 0 命中）。ana_04 = 抗核抗体谱 16 项（含 OCR 漏值行 → golden 记 partial 的诚实形态）；ana_05 = 自免肝谱 10 项（兼作两条别名证据）。README 语料清单同步（24→26 条）。
- **工具**：`local_golden.py` 的掩码判据与解析侧解耦为本地常量 `_MASK_COLUMN_TOLERANCE_PX`（沿用 180 冻结口径）——掩码只需保守判断「格是否表列依据」；若跟随中点分区会把全库脱敏产物整体翻新，收益为零、评审面激增。改前/改后对 23 份既有语料 mask 输出逐字节一致（脚本 cmp 验证）。
- **内环证据**：pytest 162 通过 / 27 skip；本机 26 语料 `check` 21/26 unchanged（5 漂移 = 既有单位归一化陈旧，与改动前同一集合）；**VL score `matched 230/236 value 206 unit 193 ref 194` 与改前逐字节一致**（语料含 VL 真值的图样本无退化）。对照：未改代码对新增 ana_04/05 golden 会漂移（7 drifted）——回归门禁有效。
- **外环证据**（快照 `data/e2e-snapshots/pre-column-assign-20261018-095631.dump`，87969B）：`refresh --from <本 worktree>` ok（16 reparse）→ `verify` 全 ok（warn=既有 RDW-CV 印刷串列）→ `seed`（17 新）→ `verify --golden` pass → `clean`（33→16 行台账还原）。共享库效果：免疫 2025-09-24 **failed/0 → success/10**（假失败解开）与 **5 → 16**、生化 2026-09-04 14→17、临检 2026-09-04 25→26、免疫 2026-05-16 2→3（离线全量重跑 16 份：5 增 0 减）。
- **已知边界（本票不修，属 OCR 引擎层）**：该单为拍屏摩尔纹照片，第 8–15 行数值（1.53/1.02/2.11/1.38/0.46/0.47/1.24）在 OCR 输出中不存在 → 修复后以 `value=null` + partial 诚实呈现；重拍好照片即可。另 ana_05 第 1 行 value 取到 OCR 的杂散 "1"（真值「阳性(+)」在同行乱位格上）——属引擎级配对质量，golden 记录现状。

**后续（另立，不在本票）**：① `e2e.py verify` 增「ocr_lines 链接覆盖率 + 歧义裕度」持续度量（依赖票 07 的 `/reports/{id}` 端点，待其合入后开票）；② ocr_lines 精确化（契约 v2：识别侧直吐行号）定为本 feature 域的 09 前置小票。
