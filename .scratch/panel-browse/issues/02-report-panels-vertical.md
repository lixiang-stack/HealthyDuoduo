# 02: 报告项目组纵切（数据 → API → 时间线卡片 / 报告详情）

**Spec:** ../spec.md（§7、§8.3、§8.6）

**What to build:** 新上传的报告把项目组写入存储；报告摘要与详情 API 返回 `panels` / `panel`；App 时间线卡片显示前 2 个项目组 + `+N`，报告详情按项目组分组并展示全部指标。失败报告与未回填旧报告回退 `report_type`，不崩、不空。

**Blocked by:** 01（识别端输出项目组与医学依据）

**Status:** resolved

- [x] 迁移后旧行有安全默认；新报告的 `panels` / `panel` 正确写入，摘要与详情读回一致。
- [x] 时间线卡片显示「肝功 · 肾功 +1」这类主 + 次项目组；只有 1 个项目时不加 `+N`；失败报告回退 `report_type`。
- [x] 报告详情按 `panels` 顺序分组，组内展示报告全部指标；混合报告（肝功 + 肾功 + 尿酸）三组都出现；核对入口仍可用。
- [x] 未回填旧报告 `panels=[]` 时卡片 / 详情回退 `report_type`，不出现空标题。
- [x] 契约测试覆盖 summary / detail / failed / imaging / upload 的 `panels` / `panel`；App 类型与测试工厂同步。
- [x] `go test ./...`、`npm test`、`npm run typecheck`、`npx expo lint` 全绿；现有上传 / 核对 / 趋势 E2E 不回归。

## Comments

**实现（2026-10-27，分支 `panel-serve-impl`）**

- 迁移 `0011_report_panels.sql`：`report_items.panel`（text NOT NULL DEFAULT `other`）、`reports.panels`（text[] NOT NULL DEFAULT `{}`）、两个索引；down 可回退。新增 0011 升降版测试：0001–0010 下种旧行，升版后 `panel=other` / `panels={}`。
- Go `internal/contract` 已由 PR #45 镜像 `Panel` / `Panels`；本票补齐 `pipeline.ReportSummary.Panels` / `ReportRow.Panels`、storage 全部写读投影（summary / detail / review / CLI show）、httpapi 类型与 handler 投影。`WriteReport` 把 nil panels 归一为空数组，满足 NOT NULL。
- OpenAPI 新增 `PanelCode`（与识别端 `$defs.panel` 枚举逐值一致）、`ReportHeader.panels`、`ReportItem.panel`；fixtures summary / detail / failed / imaging / upload / review.result 全部补齐；`npm run codegen` 更新 `schema.d.ts`。
- App 新增 `reports/panels.ts`（16 code → 中文 label）与 `reports/panelOverview.ts`：`reportPanelLabels`（前 2 + `+N`；failed / `panels=[]` 回退 report_type）、`reportTypeNote`、`reportItemGroups`（panels 顺序；不在 panels 的项归末组「其他」；未回填按 report_type 单组）、`abnormalItemCount`。
- `ReportCard` 主标签改为项目组，`report_type` 降到日期行；报告详情按项目组分组、组头显示 `N 项 · M 项异常`，组内仍逐个渲染全部 `ReviewItemCard`。核对 / 修正 / 改归属 / 删除路径未改。
- 验证：`go test ./...`；`HDD_TEST_DSN` 真库跑 migration smoke / 0011 升降版 / report store / invariant；`cd apps/mobile && npm test`（19 suites / 184 tests）、`npm run typecheck`、`npx expo lint`（0 error）、`npm run e2e`（6 files / 17 tests 全绿）；`python3 scripts/check-docs.py`。
- 未做：项目列表 / 详情端点（票 03）、`PANEL_ORDER` 与按项目筛选（票 03）、`hdd backfill-panels`（票 05）、异常指标页分组（票 04）、医学复核（票 08）。
