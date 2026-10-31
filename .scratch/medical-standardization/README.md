# medical-standardization

Status: in progress（2026-10-31）

| 文件 | 用途 |
|---|---|
| `spec.md` | 决策记录、目标模型、单位/换算规则、CI 门禁、验收 |
| `mapping.csv` | 181 项映射表；全部已填标准名/码/单位/结果类型，待医学复核（batch 2 已迁移） |
| `issues/` | 15 张实施票（01–13 agent，14–15 human）；依赖顺序见每张票的 Blocked by |

**已完成：**

- 审计结论四问一因写入 `spec.md` §1。
- 全量 181 行映射表；全部已填标准名/码/单位/结果类型（batch 2 按票 09–11 补全），待复核。
- 新标准 schema 试点：`recognizer/recognizer/coag_dict.yaml` 9 项已迁移（WS/T 886 码 / LOINC / canonical UCUM / accepted units / result_kind / result_basis）。
- `postprocess.py` loader 支持新字段；`test_dict_coverage.py` 区分 `synonyms`（来源门）与 `ocr_aliases`（语料门）；新增 `test_standard_schema.py`。
- 票 01：`ReportItem` 标准层字段（code/loinc/unit_display/unit_status/raw_*/ref_kind/result_kind/result_basis）Python/JSON Schema/Go/DB/API/App 全链路透传；识别端产出留待 03+，旧 golden 不变。
- 票 02：`unit_families.yaml` 拆为 `unit_conversions.yaml` + `ocr_forms.yaml`；loader 合并加载并门禁真实单位/OCR 形混放；单位输出不变。
- 票 03–11：9 个词典 181 项全部标准输出；canonical/display、value/ref 同步换算、量级门、原文快照、result_kind/result_basis、定性 flag 策略全部落地；49 份 golden 重生成（status/item 数/flag 与迁移前一致）；趋势查询默认排除 unverified。

**Frontier：** `12`（趋势按 code + canonical unit 连接，依赖 03–11 完成）；之后 `13` 全量 reparse/e2e，`14`/`15` 医学复核。

**下一步：**

1. 票 12：趋势键改为 `code`（LOINC 兜底）→ canonical unit；`unit_status=unverified` 默认排除，`--all` 可见；App 趋势显示 `unit_display`。
2. 票 13：全量 golden 收口、`hdd reparse`、e2e verify；记录未验证项数量与原因。
3. 确认具名临床检验复核人；未确认前 `review_status=pending`，不得声称 `verified`。
4. 票 14/15：逐项确认标准名/码、canonical unit、换算系数、`plausible_range`；PDW、D-二聚体基准、eGFR、U/L vs IU/L 四个争议项给出结论。
5. 复核通过后将 `review_status` 翻 `approved`，重生成受影响 golden；`unit_status=verified` 才会出现。
