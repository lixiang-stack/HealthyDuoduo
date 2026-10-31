# medical-standardization

Status: in progress（2026-10-31）

| 文件 | 用途 |
|---|---|
| `spec.md` | 决策记录、目标模型、单位/换算规则、CI 门禁、验收 |
| `mapping.csv` | 181 项映射骨架；batch 1（95 项）已填标准名/码/单位/结果类型，batch 2（86 项）待映射 |
| `issues/` | 15 张实施票（01–13 agent，14–15 human）；依赖顺序见每张票的 Blocked by |

**已完成：**

- 审计结论四问一因写入 `spec.md` §1。
- 全量 181 行映射骨架；batch 1 95 行已填，batch 2 86 行待映射。
- 新标准 schema 试点：`recognizer/recognizer/coag_dict.yaml` 9 项已迁移（WS/T 886 码 / LOINC / canonical UCUM / accepted units / result_kind / result_basis）。
- `postprocess.py` loader 支持新字段；`test_dict_coverage.py` 区分 `synonyms`（来源门）与 `ocr_aliases`（语料门）；新增 `test_standard_schema.py`。
- 非本地 golden 测试：335 passed / 27 skipped；本地 golden 未刷新（既有 panels 漂移）。
- 票 01：`ReportItem` 标准层字段（code/loinc/unit_display/unit_status/raw_*/ref_kind/result_kind/result_basis）Python/JSON Schema/Go/DB/API/App 全链路透传；识别端产出留待 03+，旧 golden 不变。
- 票 02：`unit_families.yaml` 拆为 `unit_conversions.yaml` + `ocr_forms.yaml`；loader 合并加载并门禁真实单位/OCR 形混放；单位输出不变。

**Frontier：** `03`（凝血 tracer bullet，依赖 01 + 02）；之后按 04–11 逐 panel，12 趋势，13 全量 golden/reparse。

**下一步：**

1. 复核 `mapping.csv` batch 1 的 `standard_name` / `wst886_code` / `loinc` / `canonical_unit` / `accepted_units`。
2. 确认具名临床检验复核人；未确认前 `review_status=pending`。
3. 把 CBC / 生化 / 激素三组按 coag 模式迁移到新 schema。
4. 实现输出层：`unit` = UCUM、`unit_display`、`raw_unit`、`unit_status`、数值/参考范围换算、量级门。
5. 更新契约 / Go / App / golden / reparse。
