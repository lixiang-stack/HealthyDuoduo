# 02: 单位换算层与 OCR 形分离

**What to build:** 把现有 `unit_families.yaml` 拆成两张职责单一的表：真实单位换算表（精确等价、线性系数、方程）与 OCR 字形修复表（只做印形修复）。loader 同时加载两张表；现有报告的单位输出保持不变，为后续按项换算和未验证输出做准备。

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] 换算表只含真实单位/等价关系/系数/方程；OCR 形只能进修复表。
- [x] OCR 形与真实单位不得混放；同一字符串不得同时出现在两张表。
- [x] 现有单位归一测试与 golden 的 unit 输出不变。
- [x] 新增非法条目测试：OCR 形进换算表、真实单位进修复表都要失败。

## Comments

**实现（2026-10-31，分支 `unit-layer-split`）**

- 数据：`unit_families.yaml` 拆为 `unit_conversions.yaml`（真实单位 + exact/linear/equation 换算声明）与 `ocr_forms.yaml`（OCR/印刷损耗形，保留样本证据注释）。
- loader：`load_unit_forms()` 合并两表并校验——换算表条目须是真实单位（保守原子表）且带 conversion/source（linear 还须 from/to 与 accepted/canonical 一致）；OCR form 不得是真实单位、不得与换算表重复；OCR 族 canonical 须为已登记真实单位。
- 兼容：合并前后 27 条 form→canonical 映射逐条相等（含 `f1` 多族裁决顺序）；golden unit 输出不变。
- 验证：`cd recognizer && uv run pytest -q`（350 passed / 28 skipped）、`uv run ruff check .`、`go test ./...`、`python3 scripts/check-docs.py` 全绿。
