# 12: 趋势按 code + canonical unit 连接

**What to build:** 把趋势连接从“指标名 + 原单位字符串”改为“标准 code（LOINC 兜底）+ canonical unit”。同一分析物跨报告、跨面板可连接；未验证单位点默认排除；跨单位换算只在 verified 时参与。

**Blocked by:** 03–11（各 panel 标准输出）

**Status:** resolved

- [x] 趋势键优先 `code`，缺失用 `loinc`，再回退 `name`；不再只靠名称字符串。
- [x] 同一 canonical unit 的 verified 点可连接；不同单位只有在换算 verified 后才连接。
- [x] `unit_status=unverified` 的点默认排除，`--all` 可见并标注原因。
- [x] App 趋势显示 `unit_display`，不显示 raw OCR 单位。
- [x] Go/API/App 测试覆盖跨面板、跨单位、未验证排除三类场景。

## Comments

**实现（2026-10-31，分支 `trend-by-code-12-13`）**

- 趋势键：`pipeline.TrendKey(code, loinc, name)` = 非空 code → 非空 loinc → name；存储侧 `COALESCE(NULLIF(code,''), NULLIF(loinc,''), name)` 与领域层同口径。CLI/HTTP 输入规范名、标准 code 或 LOINC 都会解析到同一键。
- CLI `hdd trend`：按成员全局趋势键查询，跨报告连接；默认同时排除 `unit_status=unverified`、低置信、`flag=unknown` 点；`--all` 全量显示，新增 `note` 列标注 `unverified_unit` / `low_confidence` / `unknown_flag`。`delta` 只在相邻点同 canonical unit 且单位已验证时计算。
- HTTP `GET /members/{id}/indicators/{name}/trend`：成员内解析趋势键后跨面板查询，仍默认排除 `unit_status=unverified`；`TrendPoint` 增加 `unit_display` / `unit_status`。
- 项目详情：`PanelItemsByMemberPanel` 先取该项目组出现的趋势键，再取成员全部同键指标项，同一分析物跨面板合并；`BuildPanelItems` 按趋势键 + canonical unit 分组，同名不同 code 不合并、同 code 不同 canonical unit 仍分行。`PanelItem` 增加 `key`。
- App：`selectSeries` 默认跳过 `unit_status=unverified` 点并在 meta 显示「另有 N 份单位未验证（未连线）」；`TrendSeries.unitDisplay` / `displayTrendUnit` 优先 `unit_display`，趋势卡片、详情层、项目详情与趋势页均不再显示 canonical/raw OCR 单位。
- 契约：`schemas/openapi.yaml` 的 `TrendPoint` 增加必填 `unit_display` / `unit_status`，`PanelItem` 增加必填 `key`；fixture 与 `schema.d.ts` 已同步。
- 测试：`internal/pipeline` 覆盖 code/LOINC/name 键优先级与跨名称聚合；`internal/storage` 覆盖跨面板同 code、code/LOINC 输入解析、unverified 默认排除与 `unit_display/unit_status` 透传；`cmd/hdd` 覆盖 `--all` 原因标注与 delta 门；App 覆盖未验证排除、display 单位与跨单位计数。
- 验证：`go test ./...` 全绿；`pytest 374 passed / 28 skipped`；mobile `236 passed` + `tsc --noEmit`；`ruff` 全绿；`e2e.py verify` / `verify --golden` 通过。
