# Spec：成员概览（Phase 3 · 成员页信息架构）

Status: ready-for-agent

维护者 2026-10-21 评审定稿：待核对口径按 `status === 'partial'`（排除影像单）；概览不设「最近归档」；`/` 改成成员概览，`/timeline` 保留为全部档案；量级按 20–50 份/年/成员。

**依据文档：** 统一语言 `CONTEXT.md`（报告 / 报告类别 / 指标项 / 成员 / 家长 / 核对）· 需求 `docs/design/2026-10-13-viz-and-client-spec.md`（1.3，5.1 信息架构与 M3「成员卡片首页」目标）· `ADR-0005`（报告状态与未核对计数派生）· App E2E `docs/agents/app-e2e.md` · 现有实现 `apps/mobile/src/app/{index,timeline,settings}.tsx`、`apps/mobile/src/reports/{order,status,components}.ts`、`apps/mobile/e2e/{support.ts,tests/app.e2e.ts}`。

## 为什么做

现状有两个问题，叠在一起才让「100 份」显得难解：

1. **根页不是产品页**：`/` 仍是调试页——后端地址、连通状态、「重新检测」和几个跳转链接。家长打开 App 先看到「后端地址 192.168…」，成员概览根本不存在（`apps/mobile/src/app/index.tsx`）。
2. **时间线把待办埋进历史**：`/timeline` 用 `ScrollView` + `reports.map` 把当前成员的全部报告逐张 `ReportCard` 渲染（`apps/mobile/src/app/timeline.tsx:55,98`），无分组、筛选、搜索；待核对只是一张卡上的徽标，要划过历史才会遇到。

量级已确认：**单成员 20–50 份/年**，约 3–7 屏；100 份是 5–10 年累积上限，不是首年。也就是说，当前最痛的不是「列表太长」，而是「打开 App 看不到现在」和「待办被历史淹没」。数据侧已经够用：`ReportSummary` 自带 `status`、`flag_counts`、`unreviewed_low_confidence_count`，`GET /reports?member_id=` 一次返回该成员全部摘要。缺的是信息架构，不是接口。

本期把「现在」从「历史」里拆出来：

- `/` 成员概览回答「有没有要处理的、每类最近一次是什么」；
- `/timeline` 全部档案回答「某一份在哪」。

它与设计文档 5.1 / M3「成员卡片首页」共享同一目标（第一屏回答「现在有什么值得留意」），但落点是**当前成员的概览页**，不是多成员卡片列表；多成员卡片首页仍按 M3 另议。

**证据边界：** 20–50 份/年/成员是维护者确认的量级，100 份是 5–10 年上限；仍无真实使用数据，这是设计假设，不是已观测缺陷。

## 做什么

### `/` → 成员概览（替换现有调试页）

页面自上而下：

```text
查看谁的档案        [管理成员]
[成员 chips]
[上传检查单]
待处理
  待核对 · N 份 [· M 项]          →
  识别失败 · K 份                 →
  （两者都为 0：没有待处理的报告。）
各类最近一次
  [报告卡: 类别 / 日期 / 状态 / 徽标]   （每类一份，≤ 当前类别数）
全部档案（N 份）                    →
设置                                →
```

1. 成员 chips：沿用 `MemberChips` 与 `CurrentMemberContext`；「查看谁的档案」行右侧加「管理成员」链接到 `/members`。
2. 上传检查单：沿用现有链接位置与文案。
3. **待处理**：两个计数入口，不是卡片列表。
   - `待核对 · N 份`（有未核对低置信项时追加 `· M 项`）→ `/timeline?filter=partial`；
   - `识别失败 · K 份` → `/timeline?filter=failed`；
   - 两者都为 0 时显示一行 `没有待处理的报告。`，不显示入口。
4. **各类最近一次**：每个报告类别取最近一份，按最近日期降序；复用现有 `ReportCard`（类别、日期、状态、徽标），点进详情。
5. **全部档案（N 份）** → `/timeline`。
6. 页面底部「设置」→ `/settings`（后端地址、保存并校验、连通状态都在设置页）。
7. 空态与未选成员：
   - 加载中：`正在加载概览…`（ActivityIndicator）；
   - 无成员：沿用现有「还没有家庭成员。先到家庭成员建一个。」引导（链到 `/members`）；
   - 有成员但未选中：`选择一个成员查看其报告概览。`；
   - 选中成员无报告：只显示现有「这位成员还没有检查单。拍一张检查单，几秒后这里会出现归档卡片。」引导，隐藏 3–5；
   - 未配置后端：`尚未配置后端地址，请先到设置填写。`（链到 `/settings`）；
   - 加载失败：`加载失败:{detail}` + 「重试」（与现有时间线错误态同口径）。

### `/timeline` → 全部档案（保留路径，标题改「全部档案」）

- 成员 chips 与概览一致，切换成员即刷新档案；
- 当前成员的完整报告列表，沿用现有 `ReportCard` 与 `sortReports`；
- 可选 `?filter=partial|failed`：只显示对应子集，顶部一条筛选说明（`只看待核对 · N 份` / `只看识别失败 · N 份`）+「清除筛选」；
- 无筛选且无报告：沿用现有空态；
- 筛选后为空：`没有符合筛选条件的报告。` +「查看全部档案」。

### 计数与排序口径

- **待核对**：`status === 'partial'` 且非影像单的报告份数（与卡片状态文案同口径）；`M 项` = 这些报告的 `unreviewed_low_confidence_count` 之和，`M = 0` 时省略「· M 项」。这样 dateless 等「partial 但无低置信项」的报告不会被漏掉，存量 partial 影像单也不会被误计入。
- **识别失败**：`status === 'failed'` 的份数；失败报告不计入待核对。
- **各类最近一次**：`sortReports` 后每个 `report_type` 首次出现的那一份；`report_type` 从数据派生，不硬编码 6 类。失败报告同样参与，不隐藏。
- 所有排序复用 `sortReports`（日期降序、无日期沉底、同日期按 `report_id` 升序），不新增日期解析。
- 概览不设「最近归档」：不重复渲染同一份报告卡片——待处理只给计数，类别最近一次每类一张；同类别更早的报告只通过 `/timeline` 查看。

### 路由与文案

- `/` 改成成员概览（替换调试页）；`/timeline` 继续作为按时间排列的全部档案；不新增 `/overview`、`/reports` App 路由。
- `_layout.tsx`：`index` 标题「后端连通状态」→「成员概览」；`timeline` 标题「报告时间线」→「全部档案」。
- 现有调试页的后端地址、连通状态、重新检测收进 `/settings`：设置页已有 Base URL 输入、「保存并校验」与 `ConnectivityStatus`，不新增按钮；调试页的「家庭成员」「上传检查单」「报告时间线」「去设置」链接由概览的「管理成员」「上传检查单」「全部档案」「设置」承接。
- 上传页、核对页的「去时间线查看」→「去档案查看」，指向 `/timeline`（新报告/刚核对报告在档案里一定可见，概览只保证每类最新一份）。
- `ReportManageCard` 的改派成功文案「时间线上已移到该成员名下」→「已移到该成员名下」。
- 详情页删除后的 `router.dismissTo('/timeline')` 不变：回到全部档案。
- 页面文案保持中性事实侧，不出现「健康概况 / 异常预警 / 风险」等解读词。

### 明确不做

- 「最近归档」区块（评审删除）：与「各类最近一次」重复；新报告通过 `/timeline` 查看。
- 跨成员待办总览 / 家庭卡片首页：本期概览严格按当前成员过滤；M3 另议。
- 时间线压缩、年份分组、类别筛选 chips、搜索、指标名检索：Phase 1/2，另立 spec；本 spec 只新增一个 `filter` 参数。
- 服务端分页 / 过滤：`/timeline` 仍一次取全量摘要。
- 就诊事件分组、报告合并、趋势页改动。
- 概览卡不额外拉详情：影像单的检查项目名称仍在详情页，不在类别卡上展示（避免每类一次详情请求）。

## 契约与接口

**无 OpenAPI / 数据库 / 后端改动。** 消费现有：

- `GET /members`：成员 chips；
- `GET /reports?member_id=`：摘要列表；
- 详情 / 核对 / 改派 / 删除沿用现有路由。

`?filter=` 是 App 内部 URL 参数，不是 API 契约；只识别 `partial` / `failed`，其他值按无筛选处理。

新增纯函数模块 `apps/mobile/src/reports/overview.ts`：

- `pendingSummary(reports)` → `{ partialReports, unreviewedItems, failedReports }`
- `latestByCategory(reports)` → `ReportSummary[]`
- `filterReports(reports, filter)` → `ReportSummary[]`
- `parseReportFilter(raw)` → `'partial' | 'failed' | undefined`

计数与筛选共用同一判定函数（`status === 'partial'` 且 `!isImagingReport(report)` / `status === 'failed'`），避免「待核对 N 份」与实际列表口径漂移。不引入新依赖。

## 怎么实施

1. **纯逻辑**（可先测）：新增 `overview.ts` + `overview.test.ts`，复用 `sortReports` 与 `isImagingReport`。
2. **共享加载**：新增 `reports/useMemberReports.ts`，把现 `timeline.tsx` 的 client / members / reports 加载与 `loading | unconfigured | error | ready` 状态抽出来；概览与档案共用。行为不变：聚焦时重载、错误可重试。
3. **概览页**：重写 `index.tsx` 为上述结构，删除调试页内容（后端地址/连通/重新检测），保留「管理成员」「设置」入口；`ReportCard` 增加可选 `testIDPrefix`（默认 `report-card`），概览用 `overview-latest`，避免与档案页的 `report-card-${id}` 撞 locator。
4. **档案页**：`timeline.tsx` 改用共享 hook；读 `useLocalSearchParams().filter`，渲染筛选说明与现有列表。
5. **导航与文案**：`_layout.tsx`、`upload.tsx`、`review/[report_id].tsx`、`components.tsx`。
6. **E2E**：`support.ts` 增加概览/档案 locator；`app.e2e.ts` 更新根页断言与上传/核对文案，新增概览与筛选用例（见验收）。新增 locator：

   ```text
   overviewPendingPartial    = 'overview-pending-partial'
   overviewPendingFailed     = 'overview-pending-failed'
   overviewPendingEmpty      = 'overview-pending-empty'
   overviewLatest(id)        = `overview-latest-${id}`
   archiveFilterBanner       = 'archive-filter-banner'
   archiveFilterClear        = 'archive-filter-clear'
   archiveEmpty              = 'archive-empty'
   ```

   `reportCard(id)` 继续只用于 `/timeline` 与详情头部。

### 改动面

11 个代码/测试文件 + 1 处 E2E 文档计数：

| 文件 | 改动 |
| --- | --- |
| `apps/mobile/src/reports/overview.ts` | 新增：计数、分组、过滤、参数解析 |
| `apps/mobile/src/reports/overview.test.ts` | 新增：纯逻辑单测 |
| `apps/mobile/src/reports/useMemberReports.ts` | 新增：从 `timeline.tsx` 抽出的共享加载 |
| `apps/mobile/src/app/index.tsx` | 重写为成员概览；删除调试页内容 |
| `apps/mobile/src/app/timeline.tsx` | 改为全部档案 + 筛选；使用共享加载 |
| `apps/mobile/src/reports/components.tsx` | `ReportCard` 加 `testIDPrefix`；改派文案 |
| `apps/mobile/src/app/_layout.tsx` | index/timeline 标题 |
| `apps/mobile/src/app/upload.tsx` | 链接文案与目标 |
| `apps/mobile/src/app/review/[report_id].tsx` | 链接文案与目标 |
| `apps/mobile/e2e/support.ts` | 新增 testID 约定 |
| `apps/mobile/e2e/tests/app.e2e.ts` | 根页断言、文案、新增用例 |
| `docs/agents/app-e2e.md` | 完成判据 9/9 → 11/11 |

其中 4 处是一行标题/文案；`settings.tsx` 不改（已具备后端地址、保存并校验、连通状态）。无新服务、无新依赖、无迁移。

### 关键决策

1. **待处理用计数入口，不铺卡片**：概览长度与待办数量解耦，明细在档案页筛选看；待办很多时概览不会重新变成长列表。
2. **`/` = 概览，`/timeline` = 档案，不新增 `/overview` / `/reports`**：概览必须真的是首页，「首屏」才成立；`/timeline` 继续承担「按时间排列的全部档案」，名副其实，也不改删除回跳和现有深链。
3. **不设最近归档**：评审确认与「各类最近一次」重复；同一份报告不会在概览出现两张卡。上传/核对后的入口指向 `/timeline`，保证新报告一定可见。
4. **类别从数据派生**：`report_type` 是识别端自由字符串，硬编码 6 类会在新增词典时漏项。
5. **档案页暂不压缩/分组**：20–50 份约 3–7 屏，可接受；Phase 1 等 100+ 或真实使用反馈后再做，直接作用 `/timeline`。

## 怎么验收

### 单元测试

`cd apps/mobile && npm test`（新增 `overview.test.ts`）：

- `pendingSummary`：`partial` 份数（含无低置信项的 partial、排除影像单）、项数之和、失败份数、`success` 为 0、空列表；
- `latestByCategory`：每类一份、日期最新者胜、无日期沉底、同日期按 `report_id`、类别顺序按最近日期；100 条混合类别输入下类别数稳定、不重复；
- `filterReports`：`partial` / `failed` / 无筛选、非法值按全部；`partial` 与待核对计数同样排除影像单；
- `parseReportFilter`：单值、数组、未知值。

### 类型与 lint

```bash
cd apps/mobile
npm run typecheck
npx expo lint
npm test
```

### App E2E

`npm run e2e`：现有 9 条全绿 + 新增 2 条，共 11 条；同步 `docs/agents/app-e2e.md` 的完成判据 9/9 → 11/11。

新增用例（插在核对用例之前，避免核对把待核对计数清零）：

1. **成员概览**：`/`（小明）显示 `待核对 · 1 份`、`各类最近一次` 含 `cbc_03` 与 `us_03`、`全部档案（3 份）`；切到妈妈后待核对仍为 1 份（`cbc_04` 含低置信项），类别最近一次只含 `cbc_04`、全部档案为 1 份。
2. **全部档案与筛选**：重开 `/`（小明），点「全部档案（3 份）」进入 `/timeline`，显示 3 张卡；重开 `/` 点「待核对」进入 `/timeline?filter=partial`，只显示 `cbc_03`；点「清除筛选」回到全部 3 份；直接开 `/timeline?filter=failed` 显示筛选空态与「查看全部档案」。

既有用例调整：

- 设置用例：`/` 断言从「后端地址 + 已连通」改为「查看谁的档案」（后端地址与健康检查已在 `/settings` 断言）；
- 后端不可达用例：`/` 错误态从「无法连接后端」改为「加载失败」（概览的列表加载错误）；
- 「时间线按成员过滤报告」保持 `/timeline`，用例名改为「档案按成员过滤报告」，断言不变；
- 上传成功与同图重传后的「去时间线查看」改为「去档案查看」，`reportCard(${report_id})` 断言不变；
- 改归属/删除后的回跳仍是 `/timeline`，断言不变。

`识别失败` 非零入口不在 E2E 覆盖：现有 seed 无 failed 报告，只有单测与手工覆盖；E2E 断言 0 时该入口隐藏。

### 手工验收（先 Expo Web，再 Expo Go）

- 根页就是成员概览：打开 `/` 不再看到后端地址/连通状态；「设置」能进 `/settings`，后端地址、保存并校验、连通状态都在。
- 「管理成员」能进 `/members`；创建/切换成员后回概览，chips 与内容同步。
- 概览行数不随总数增长：待处理 ≤ 2 行 + 类别 ≤ 6 + 全部档案入口 1；单测用 100 条 summary 锚定类别数上限，手工用现有 seed 确认三个区块结构。
- 识别失败入口：用一张全空白/糊图上传，确认 `识别失败 · 1 份` 出现；点进去只显示该失败报告，卡片显示失败原因（不写入 E2E seed）。
- 无日期报告：类别最近一次显示「日期未知 / 检查单未印日期」，不排到有日期报告前面。
- 切换成员：概览与档案都随当前成员变化；筛选状态在切换成员后保留。
- 同类别多份：概览只显示最近一份；点「全部档案」能看到更早的报告。
- 删除当前成员最后一份报告后回跳 `/timeline`：档案进入空态，不崩。
- 待核对与失败都为 0 后：概览显示 `没有待处理的报告。`；`/timeline?filter=partial` 显示筛选空态。
- 首屏（390×844）不滚动即可看到待处理区；从概览到任意一份报告不超过 2 次点击。

## 风险与未决

- **家长可能只在上传时打开 App**：若从不主动看待办/最近一次，概览就是仪式。首月观察打开次数与从概览进入详情的次数；若极低，下一步应把上传闭环做得更短，而不是继续加 dashboard。
- **当前成员上下文可能不够**：若频繁切换成员，家庭卡片首页（M3）比按成员概览更值；首发先按当前成员，观察切换频率。
- **待办长期为 0 时概览显得空**：OCR 干净时只剩「各类最近一次」；届时可评估是否把「全部档案」上移或加紧凑摘要。
- **档案页仍会长**：20–50 份约 3–7 屏可接受；到 100+ 时 Phase 1（压缩 + 年份分组）或服务端分页提前。
- **筛选与分页的未来冲突**：当前客户端过滤依赖全量摘要；若未来 `/timeline` 上服务端分页，`?filter=` 必须下沉为 API 参数，否则筛选只作用于已加载页。本期明确不分页。
- **失败报告 E2E 覆盖**：现有 seed 无 failed 报告，`识别失败` 非零入口只由单测覆盖；若后续要覆盖，需加一张识别失败样张，另立票。
- **根页迁移影响排障**：后端地址/连通/重新检测从 `/` 移到 `/settings`；设置页已有保存并校验，E2E 覆盖保存 + 健康检查。回滚时恢复原 `index.tsx` 即可，无数据变更。
- **待核对计数包含无低置信项的 partial**：dateless / 缺数值报告会一直留在待核对，核对完全部低置信项也无法清零（维护者 2026-10-21 实测：7 份 / 5 项，核对后剩 3 份）。当前口径有意保留，避免漏掉这些报告；后续若要分清，可拆「待核对 / 信息不完整」或补录日期，另立票。
