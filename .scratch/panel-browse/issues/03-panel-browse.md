# 03: 按项目浏览（项目列表 + 项目详情 + 报告筛选）

**Spec:** ../spec.md（§7.3、§8.3、§8.4）

**What to build:** 档案页增加「按项目」视图：列出当前成员实际出现过的项目组；点项目进项目详情，看到该项目全部指标的最新值和跨报告趋势；底部可跳到包含该项目的报告列表。影像直接进报告详情，失败报告不进项目列表。

**Blocked by:** 02（报告项目组纵切）

**Status:** resolved

- [x] 项目列表只显示当前成员有报告的项目组，不显示空分类；排序按最近报告日期降序。
- [x] lft_02 出现在肝功 / 肾功 / 尿酸；lft_06 出现在肝功 / 血脂；lft_08 出现在肝功 / 肾功 / 尿酸 / 电解质；tft_01 出现在甲状腺；ua_05 只出现在尿常规，不出现血糖 / 肝功。
- [x] 项目详情展示该项目全部指标；异常项内联趋势；正常 / 未知 / 未检测项给最新值与入口；`latest_report_not_tested` 时标「最近一次未检测」。
- [x] 「包含该项目的报告」跳到 `/timeline?panel=<code>`，数量与项目列表一致；失败报告不出现在该筛选里。
- [x] 影像行直接进最新报告详情；`other` 一行显示来源报告类别；无报告 / 全部失败 / 空项目有对应空态。
- [x] 项目列表 / 详情 / 筛选有 E2E；`npm test`、`npm run typecheck`、`npx expo lint` 全绿。

## Comments

**实现（2026-10-27，分支 `panel-views-impl`）**

- Go 契约：OpenAPI 新增 `GET /members/{member_id}/panels` 与 `GET /members/{member_id}/panels/{panel}`、`PanelsList` / `PanelSummary` / `PanelDetail` / `PanelItem`；`Panel` 路径参数复用 `PanelCode` 枚举。`httpapi` 镜像 + fixtures（`panel.list.json` / `panel.detail.json`）与 Go 回转互证，冻结端点 12 → 14。
- 领域 / 存储：`pipeline.PanelStore`、`PanelSummary` / `PanelDetail` / `PanelItem` 与 §5.3 固定优先级排序；`storage.PanelSummariesByMember` 用 CTE 一次聚合（report_count / item_count / abnormal_count / latest / report_types），`PanelItemsByMemberPanel` 读平铺行后由 `pipeline.BuildPanelItems` 按 name+unit 分组并计算跨报告点、`not_tested` 与 `latest_report_not_tested`。failed 报告全部排除；未回填存量（`reports.panels={}`）一律归 `other`，与 App `?panel=` 回退一致。
- 项目组定义单一来源：`schemas/report.schema.json` `$defs.panel` 的 `enum` + `x-panel-order`；Python / Go / App 各加契约测试锁定枚举集合与 §5.3 顺序，改一处漏改其它端会直接失败（OpenAPI ↔ 识别 schema 的枚举互证沿用既有测试，App 的 `PanelCode` 仍由 codegen 得到）。
- HTTP：项目列表 / 详情 handler；未知成员 404、非法 panel 400；空列表 / 空 points 序列化为 `[]` 而非 null。趋势点投影抽出 `toAPITrendPoint`，项目详情与趋势端点共用 reviewed 派生。
- App：`reports/panels.ts` 增加 `PANEL_ORDER`；`panelOverview` 增加 `reportHasPanel` / `filterReportsByPanel` / `parsePanelParam` / `panelSourceLabel` / `splitPanelItems`；`api/reports.ts` 增加 `listMemberPanels` / `getPanelDetail`；新增 `useMemberPanels` / `usePanelDetail`。
- 档案页增加「按时间 / 按项目」分段与项目列表；`/panel/[panel]` 项目详情（异常项内联 `TrendSparkline`，正常 / 未知 / 未检测紧凑行，`TrendDetailSheet` 复用已加载 points，>12 项默认折叠）；`?panel=` 横幅与筛选；影像行直接打开最新报告详情；`other` 行显示来源类别。
- 项目详情头部同时读取成员与报告摘要：显示成员名；成员最新一份报告为 failed 时提示「另有更新的一份识别失败报告」并链到该报告（数值仍取最近非 failed）。空 items 时底部「包含该项目的报告」仍可用。
- 验证：`go test ./...`；`HDD_TEST_DSN=<一次性 PG>` 逐测试独立 schema 跑 migration / panel / report / review / trend / backfill smoke；`cd recognizer && uv run --frozen pytest`（330 passed / 28 skipped）；`cd apps/mobile && npm test`（20 suites / 196 tests）、`npm run typecheck`、`npx expo lint`（0 error，仅既有 `review/items.test.ts:212` warning）、`npm run e2e`（7 files / 21 tests 全绿）；`python3 scripts/check-docs.py`。
- E2E 新增 `panels.e2e.ts`（甲状腺 / 尿常规 / 仅失败报告）与 `trends.e2e.ts` 混合报告用例；`app.e2e.ts` 补影像行直接进报告。失败报告用 98 byte 无文本白图经真实 OCR 生成（`panels=[other]`、无指标项），不依赖 golden。`docs/agents/app-e2e.md` 同步 21 条用例、回放样本与 testID。
- 回放样本新增 `tft_01` / `ua_05`；掩码 OCR 语料（lft_06/08、fibrosis_01 等）没有原图，无法作为 E2E 上传样本，`other` 行来源类别由纯逻辑测试覆盖，完整回放留给票 07。

**本票未做 / 留给后续：**

- 档案「按项目」顶部的「全部异常 N 项 ›」入口归票 04 / 06（异常指标页改造与 IA 壳）。
- 项目详情的指标名使用契约 `PanelItem.name`（规范名）；`PanelItem` 不含 `raw_text` / `review_status`，未复用报告详情的「原文印名优先」展示口径。如需统一需扩契约字段，留给后续票。
- 「最新失败报告」提示、空 items 底部报告入口、`other` 多来源趋势层均由纯逻辑 / 存储测试覆盖；它们缺少 E2E fixture（需要带日期的失败报告 / 空项目），待票 07 收口时补。

**实现中修正的口径（code review 发现）：**

- 同日多份报告的「最近」统一按 `report_date DESC NULLS LAST, image_sha256 DESC`（dateless 最后、同日取 sha 降序），项目列表 `abnormal_count`、项目详情 `latest` / `latest_report_not_tested` 与「最新失败报告」提示同一口径；存储测试用同日 pn-b / pn-f 锁定。
- 未回填存量（`reports.panels={}`）的项目列表 / 详情 / App `?panel=` 统一按 `other` 处理，不再从指标项反推 panel；三处口径一致。
- `pipeline.BuildPanelItems` 承载 name+unit 分组 / 最近点 / not_tested / 排序业务规则，存储只负责 SQL 读取与扫描；纯逻辑在 `panels_test.go` 直测。
- 趋势层「最新：{reportType}」按该指标最近点所在报告的真实类别取（`report_type_by_id`），`other` 多来源项目不再用来源集合冒充单点类别。
- 项目组顺序不再靠三端手写清单互相对齐：`schemas/report.schema.json` 的 `x-panel-order` 是唯一来源，Python / Go / App 契约测试分别校验；方案 A（schema + 互证测试）替代引入三端生成器。

**E2E 样本边界（供票 07 参考）：** `samples/expected/report` 中 lft_06 / lft_08 / fibrosis_01 / unknown_01 等来自掩码 OCR 语料，无原图，`Replay` 无法按 sha 命中；本票 E2E 使用有原图的 tft_01 / ua_05 + 既有 lft_02 / lft_05 种子覆盖混合报告。失败报告用白图生成，不新增 golden。
