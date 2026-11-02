# 13: 全量 golden / reparse / e2e 验收

**What to build:** 全量收口：重新生成所有 golden，执行 `hdd reparse`，跑 e2e verify，确认 `unit` 字段没有 OCR 碎片、未验证项按约定为 null + unverified、趋势/展示/契约一致。允许 `review_status=pending` 的项以 raw+unverified 形态存在，verified 翻转由 14/15 负责。

**Blocked by:** 12 (趋势按 code + canonical unit)

**Status:** resolved

- [x] 全量 golden 重冻结，逐项变更说明；不再有 `.09/L`、`7/6.0`、`ag/L` 等 `unit`。
- [x] `hdd reparse` 全量重跑成功；API/App/趋势读到的字段一致。
- [x] e2e verify 通过；未验证项数量与原因可审计。
- [x] CBC/生化/甲功/凝血 matched/value 不下降；unit 变更全部有说明。
- [x] 本地无数据时，CI + 样本子集验收，并在票内记录缺口。

## Comments

**验收（2026-10-31，分支 `trend-by-code-12-13`）**

- **Golden**：`uv run python -m recognizer.golden` 全量重生成，`samples/expected/report` 零 diff（main 的 49 份 golden 已是票 03–11 后的最终冻结态）。所有量化项 `unit` 均为 UCUM 码或 `null`，无 `.09/L`、`7/6.0`、`ag/L` 等 OCR 碎片。
- **Reparse**：`e2e.py refresh --from .worktree/trend-by-code` 构建 hdd、重建 recognizer、对库内 70 份报告全量 reparse，全部成功；DB goose 升到 0012，`code/loinc/unit_display/unit_status` 等标准列可用。
- **E2E**：`verify --item 血小板计数` 通过（单位归一 stale=0，ocr-lines 752/752 + 8/8 sections 一致，trend 默认隐藏 13/13 全部有原因）。`seed` 0 new（49 份样本均已缓存），`verify --golden` 49 份逐字段一致（items / sections），通过。未运行 `clean`：本次 seed 未新增行，避免误删此前台账中与真实数据重合的行。
- **未验证项审计**（`e2e.py verify` 新增可重复输出）：`unverified=668（unit=null 76，low_confidence 76）`、`unit_status=null` 84（全部为 qualitative/microscopy，无单位）、`invalid=0`。668 项中 quantitative 587、ratio 43、arbitrary 38；原因均为 `review_status=pending`，不是单位解析失败。verified 翻转留给票 14/15。
- **效果不下降**：recognizer 代码相对 main 零改动（本票只改 Go/App/契约与 e2e 工具），`local_golden.py score` 基线 `matched 230/236，value 189，unit 126，ref 194`；分组 临检 129/131、生化 51/53、激素 5/6、免疫 45/46。本地 `data/` 无凝血样本，凝血以 `samples/expected/report/coag_*` golden 与 pytest 覆盖。
- **契约一致性**：`TrendPoint` / `PanelItem` 新增字段经 OpenAPI fixture、Go 契约测试、codegen 与 mobile 类型检查互证；App 趋势显示 `unit_display`，未验证点不连线。缺口：本机真实数据全部 pending，因此默认趋势为空是预期状态，需 14/15 复核后才会出现 verified 序列。
