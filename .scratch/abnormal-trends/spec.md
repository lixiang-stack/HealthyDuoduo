# Spec：异常指标趋势（abnormal-indicator-trends）

Status: ready-for-agent（2026-10-23 评审后修订）

**范围：** 家长侧 App + `hdd serve` 趋势端点；不含识别端 / 词典 / 数据库迁移。

**维护者 2026-10-23 决策：**

1. **分组先按报告类别**（`report_type`），指标级「肝功 / 甲状腺」大项本期不做，后续另立需求。
2. **范围 = 每类最新非 failed 报告中的异常项**（`high / low / abnormal`）；最新报告正常或没做的指标不出现；failed 报告零指标项，不让整类消失。
3. **展示 = B**：列表内联迷你趋势图 + 点按打开全屏详情层（完整图、`method`、逐点数值、信任标记）；详情放大选中点（默认最新）。

**评审后修订（Q1–Q10）：**

- 连接键用报告中的 name+unit；改名/改单位会断开序列；不可变 `item_key` 另立需求。
- 单位归一有缺口；份数按同单位点数，另提示「另有 N 份单位不同（未连线）」；单位族覆盖另立需求。
- 术语：异常指标 = high/low/abnormal；UI 用「份报告」不用「记录」；CONTEXT 已补词条。
- 方向由趋势图表达；本期不加文字化「较上次」Δ。
- 参考带允许在极端值下压缩；参考范围文本始终可见。
- 组头带最新报告日期；组头计数 = 去重后行数。
- 同日期同类别按 `report_id` 升序；本期不加 `created_at`。
- 详情头「最新：{report_type} · {unit}」；`SafeAreaView` 用 `react-native` 或补 provider。
- 契约：非法 member_id 404；删除 NotFound 的「trend name 未收录」；`name` 用 `IndicatorName` 组件。

**原型（一次性，未入库）：** `.scratch/abnormal-trends/prototype.html` + `a.png` / `a-lower.png` / `b-list.png` / `b-detail.png` / `c.png`。浏览器打开原型，底部浮条切换 A/B/C，B 里点「查看趋势」打开详情层。

---

## 1. 为什么做

**问题不是「没有图表」，是「没有跨报告的比较」。** 当前 App 已经有成员概览、全部档案、报告详情和核对闭环：家长能看到某一份报告里哪些指标偏高/偏低，也能逐项核对数值；但看不到同一个指标在多次报告之间是变好、变差还是持平。对于需要长期观察的指标（肝功、甲状腺、血常规等），单次报告只能回答「这次是否超出参考范围」，回答不了「和上次比怎么样」。

M4 规格早已把「同名指标跨报告趋势」纳入范围，并在 M0 冻结了趋势端点的 HTTP 契约。但现状有三处缺口：

1. **服务端没实现**：`schemas/openapi.yaml` 有 `GET /members/{member_id}/indicators/{name}/trend`，`internal/httpapi/types.go` 有 `TrendResult`，但 `cmd/hdd/serve.go` 没有注册路由，storage 也没有成员维度的趋势查询。
2. **契约漏了路径参数**：趋势路径里有 `{name}`，OpenAPI 的参数表却没有声明 `name`。`openapi-typescript` 生成的 `getTrend` 只有 `member_id`，App 无法类型安全地调用。这是契约缺陷，不是新需求。
3. **分组口径受数据模型限制**：报告只有报告级 `report_type`，没有指标级「检查大项」字段。真实数据中「肝功 / 甲状腺」可能落在旧的 `report_type=肝肾功能 / 甲状腺功能`，也可能落在 2026-10 合并词典后的 `生化 / 激素`。本期按报告类别分组，零 schema 改动；指标级大项另立需求。

**目标：** 在成员概览之后，回答「现在有哪些异常指标值得看，它们最近往哪走」。不做诊断、不做建议，只陈述事实。

### 1.1 事实核查（2026-10-23）

以下三条来自代码与真实数据核查，直接修正了初稿的假设：

1. **单位归一不完整。** `unit_families.yaml` 只覆盖 6 个族；共享栈 `member 9` 有 9 个同名多单位指标，其中 4 个是同一物理单位的 OCR 变体：`红细胞计数` 的 `*10~12/L` vs `10^12/L`、`血小板` 的 `*10` vs `10^9/L`、`平均红细胞体积` 的 `IJ` vs `fL`、`总25羟基维生素D` 的 `ng/m1缺乏` vs `ng/ml缺乏`。另外 ALT/AST/ALP 存在 `U/L` vs `IU/L`（lft_06），`ng/ml` vs `ng/mL`。这些点会被拆成两条序列；行的份数只数候选单位，可能显示「仅 1 份报告」但实际有 2–4 份。
2. **趋势连接键用的 name/unit 是可变的。** 初稿称「规范名 + 归一单位」，实现用 `report_items.name` + `unit`；两者是最终生效值，`ApplyReviewAction` 把家长修正的 name/unit 原样写入（`internal/pipeline/review.go:92-94`），契约也允许 `correct.name/unit`。真实库 22 条修正对：0 条改名、1 条改单位（`孕酮` 的 OCR 垃圾单位 → `ng/mL`）。一旦改名，旧报告的规范名点接不上。
3. **参考带判据与线性轴矛盾。** 原型 `b-detail.png` 里 27.40→835 的参考带 7-40 已被压成 0 附近的一条细线；初稿的验收却写「参考带不被压没」。线性轴 + 835 的量级不可能同时满足；本期改为「参考范围文本始终可见，淡带在极端值下允许压缩」。

---

## 2. 做什么

### 2.1 `/trends` 页

页面自上而下：

```text
异常指标趋势（Stack 标题）
查看谁的档案
[成员 chips]（当前成员高亮；切换即刷新）
[首页入口进入时已在当前成员上下文]

生化 · 2021-08-02                       6 项异常
┌─────────────────────────────────────┐
│ 丙氨酸转氨酶                 偏高     │
│ 835 U/L              参考 7-40       │
│ [迷你趋势图]                          │
│ 2017-11-18 – 2021-08-02 · 2 份报告    │
│                              查看趋势 › │
└─────────────────────────────────────┘
... 同类别其余异常项 ...

临检 · 2026-05-16                       2 项异常
... 同结构 ...
```

- 分组键 = `ReportSummary.report_type`，不硬编码类别清单；类别顺序 = 每类最新非 failed 报告日期降序（复用 `latestByCategory` 的顺序）。
- 每类只取最新一份非 failed 报告；`flag_counts.high + low + abnormal > 0` 才请求详情，否则整类不出现。
- 组头 `{report_type} · {最新报告日期} · {N} 项异常`；`N` = 去重后实际列出的行数；0 行的组不输出；日期用 `reportDateLabel` 兜底「日期未知」。
- 只列 `flag ∈ {high, low, abnormal}` 的指标项；`normal / unknown` 不列。
- 同一指标（`name + unit`）在多个类别出现时，只保留最先出现的类别（类别日期序，最近者优先）；同一报告内重复 name+unit 只保留第一项。
- 行内：
  - 指标名；最新值为未核对低置信（`low_confidence && review_status === 'unreviewed'`）时加「低置信」标签；flag 文案用 `flagLabel`（偏高 / 偏低 / 异常）。
  - 最新值 / 单位；参考范围 `参考 {ref_range}`，无则 `无参考范围`。
  - 迷你趋势图（见下）；不可画时显示 `暂无可用数值趋势`。
  - 元信息：`YYYY-MM-DD – YYYY-MM-DD · N 份报告`；单点 `YYYY-MM-DD · 仅 1 份报告`；无日期 `日期未知 · N 份报告`；追加 `· 另有 N 份缺日期` / `· 另有 N 个非数值指标项` / `· 另有 N 份单位不同（未连线）`（条件独立，均可能同时出现）。
  - 「查看趋势 ›」；整行是 `Pressable`。
- 空态：
  - 加载中：`正在加载异常指标趋势…` + ActivityIndicator。
  - 未配置后端：`尚未配置后端地址，请先到设置填写。`（链到 `/settings`）。
  - 加载失败：`加载失败:{detail}` + 「重试」（重试走 `useMemberReports.reload`，会重新触发趋势加载）。
  - 无成员：`还没有家庭成员。先到家庭成员建一个。`（链到 `/members`）。
  - 有成员但未选当前成员：`选择一个成员查看异常指标趋势。`
  - 当前成员无报告：`这位成员还没有检查单。`（链到 `/upload`）。
  - 当前成员有报告但无异常：`这位成员暂时没有异常指标。`（链到 `/timeline`，文案「查看全部档案」）。

### 2.2 详情层（B）

- 整行点按打开全屏 `Modal`（`animationType="slide"`、`transparent={false}`、`onRequestClose` 关闭）；`SafeAreaView` 包裹（`react-native` 的，或补 `SafeAreaProvider`）。
- 头部：返回 `‹`（关闭，testID `trend-detail-close`）+ 指标名。
- 元信息：`最新：{report_type} · {unit ?? '无单位'}`。
- 完整趋势图（testID `trend-detail-chart`）：
  - 日期轴：首尾日期标签；单点居中一个日期。
  - 3 个取整 y 刻度（0 / 中 / 上；非负数据从 0 起）。
  - 参考范围淡带随点分段；无范围的分段不画。
  - 实心 / 空心点；选中点放大（默认最新）；中性灰连线。
  - 不可画时显示 `暂无可用数值趋势` + 原因：`最近一次为非数值结果` / `最近一次缺日期` / `没有可绘制的数值点`。
- 选中点卡片：
  - 日期（或 `日期未知`）、值 + 单位、`参考 ...` / `无参考范围`、flag 文案。
  - `方法：{method ?? '—'}`。
  - 信任：`未核对低置信（空心）` / `已核对（实心）` / `高置信（实心）`。
- 逐点列表（testID `trend-point-${index}`）：
  - 同单位、按日期升序、dateless 沉底、按 report_id 去重。
  - 每行：实心/空心圆点、日期（或 `日期未知`）、值 + 单位、flag 文案。
  - 点行切换选中点；默认选中候选指标对应的最新报告点；图同步放大选中点。
  - 列表包含非标量点（区间 / 不等式 / 文本），只影响能否画图，不隐藏数据。
- 图例：`实心 = 高置信 / 已核对`、`空心 = 未核对低置信`。
- 元信息补充：`另有 N 份缺日期` / `另有 N 个非数值指标项` / `另有 N 份单位不同（未连线）`。
- 不做跨单位换算/连线；只做中性事实计数；不做诊断/建议。
- 原型底部的「跨单位族不连线、不提示」是开发术语，家长侧不展示；只留图例和点按提示。
- Web 浏览器后退离开 `/trends`，不关层（本期接受）；`onRequestClose` 处理 Android 返回与 Web ESC。

### 2.3 迷你趋势图

- 高 64；宽度取卡片内容宽度（`onLayout`，默认 320）。
- 只画「有日期 + 标量」的点；x 按日期比例；单点居中，不画线。
- y 域包含数值与参考范围端点，非负数据从 0 起，8% 边距；迷你图不做轴刻度；单侧范围与标量参考复用 `parseRefRange` / `stripLayout` 口径。
- 参考范围按点分段铺淡带；无范围的分段不画。
- 标记：`filled = reviewed || !low_confidence`；`hollow = low_confidence && !reviewed`；颜色用共享 `FLAG_COLORS`。
- 无障碍标签：`{name}趋势，{N} 个数值点`。

### 2.4 首页入口

- `apps/mobile/src/app/index.tsx` 在「待处理」之后、「各类最近一次」之前加：
  - 文本「异常指标趋势」；`Link href="/trends"`；testID `overview-trends`。
  - 只在当前成员有报告时显示（与「各类最近一次」同一分支）。
- `_layout.tsx` 新增 `<Stack.Screen name="trends" options={{title: '异常指标趋势'}} />`。

### 2.5 明确不做

- **不做指标级检查大项**（肝功 / 肾功 / 甲状腺 / 血糖 / 血脂…）。这需要给识别端词典和指标项契约新增分组字段、回填存量、改 golden，是另一个需求；本期按 `report_type`。
- **不展示 `normal / unknown` 指标**。`unknown` 是「值与参考范围无法比较」的结果状态，不是异常；`normal` 不是本期主题。
- **不做跨单位换算**。连接键 = 报告中的指标项名称 + 归一单位；跨单位族的点段不连线；只以中性事实提示「另有 N 份单位不同（未连线）」，不做运行时换算和换算系数表。
- **不做完整 M4 独立趋势路由 / 深链 / 分享**。B 的详情层是页面内全屏层，不是新路由。
- **不做批量趋势接口**。先接受当前量级的 N+1；实测慢再评估。
- **不做诊断、评分、建议、风险文案**。所有文案停在事实侧。
- **不做文字化「较上次」Δ**。方向由趋势图表达；本期不增加方向箭头/差值文案，后续用真实使用验证。
- **Web 浏览器后退不关详情层**。详情层是页面内 `Modal`，`onRequestClose` 处理 Android 返回与 Web ESC；浏览器后退仍离开 `/trends`，本期接受，不做独立路由。

---

## 3. 契约与接口

### 端点

`GET /api/v1/members/{member_id}/indicators/{name}/trend?unit=...`

- `member_id`：int64；未知或非法 → 404 `member not found`（不返回 400，按本端点契约）。
- `name`：指标项名称（通常为词典规范名；修正后以修正值为准），URL 编码；OpenAPI 声明可复用 `IndicatorName` path 参数（本期补）。
- `unit`：可选；空/缺省 = 全部单位；非空 = 精确匹配归一单位。本期 App 不传 `unit`（取全部单位后由 `selectSeries` 过滤/计数）。
- 200：`TrendResult {member_id, name, points: TrendPoint[]}`；无点返回 `points: []`，不是 null。
- 500：存储查询失败，`Error {message}`。
- 排序：`report_date ASC NULLS LAST, image_sha256 ASC, item id ASC`。
- `TrendPoint` 字段：`report_id` / `report_date` / `value` / `unit` / `ref_range` / `method` / `flag` / `low_confidence` / `reviewed`。
- `reviewed = review_status ∈ {confirmed, corrected}`；`unreviewed` 为 false。
- 不查词典成员关系；App 只请求报告里真实存在的指标项名称。

### OpenAPI 修正

- `schemas/openapi.yaml` 新增 `components/parameters/IndicatorName`：`in: path`、`required: true`、`schema: {type: string, minLength: 1}`、描述「指标项名称（通常为词典规范名；修正后以修正值为准），URL 编码」；趋势端点 `$ref` 它。
- 修正 `components/responses/NotFound` 描述：删除「或 trend name 未收录」，与「不做词典成员检查、空结果 200」一致。
- 12 端点集合不变；fixtures 不变；`npm run codegen` 重新生成 `apps/mobile/src/api/schema.d.ts`。
- 新增 contract 断言：趋势端点声明 `name` path 参数，且 `in: path`、`required: true`、`minLength: 1`；`NotFound` 描述不含「trend name 未收录」。

### 无变更

- 无数据库迁移；无识别端 / 词典 / golden 改动；无新端点；无新环境变量。

---

## 4. 怎么实施

### 4.1 数据流

```text
/trends
  useMemberReports
    ├─ GET /members
    └─ GET /reports?member_id=...
         │
         ├─ latestByCategory(reports.filter(status !== 'failed'))  # 每类最新非 failed
         └─ 只保留 flag_counts.high+low+abnormal > 0 的类别
              │
              ├─ GET /reports/{report_id}        # 只取这些类别的报告详情
              │    └─ 筛 high/low/abnormal 项
              │    └─ 按 name + unit 去重 → 候选指标（组头计数 = 行数）
              │
              └─ GET /members/{member_id}/indicators/{name}/trend
                   └─ 全部单位时序点
                        ├─ selectSeries 按候选单位过滤 / 计数
                        ├─ 迷你图（列表）
                        └─ 完整图 + 点列表（详情层，复用同一份数据）
```

**关键边界：** 趋势序列只来自趋势端点；客户端不跨报告拼装序列。客户端只做「从最新非 failed 报告里发现哪些指标值得看」这一步筛选，不做时序聚合的第二套实现。趋势请求**不带 `unit`**，以便对「其他单位」做中性计数；`selectSeries` 负责按候选单位过滤和计数（端点的 `unit` 门控仍保留并测试，只是本期 App 不用）。

### 4.2 后端

1. **契约**：补 `IndicatorName` 参数 → codegen → contract test。
2. **接口与类型**：`internal/pipeline/types.go`
   - `TrendStore` 窄接口：`TrendPoints(ctx, memberID int64, name, unit string) ([]TrendPointRow, error)`。
   - `TrendPointRow`：`ReportID string`、`ReportDate *string`、`Value *string`、`Unit *string`、`RefRange *string`、`Method *string`、`Flag string`、`LowConfidence bool`、`ReviewStatus string`。
   - 不改 `pipeline.Store` / CLI `TrendByName`。
3. **存储**：`internal/storage/store.go`
   - 新增 `sqlTrendPointsByMemberName`：
     ```sql
     SELECT r.image_sha256,
            COALESCE(to_char(r.report_date, 'YYYY-MM-DD'), ''),
            i.value, i.unit, i.ref_range, i.method, i.flag, i.low_confidence, i.review_status
       FROM report_items i JOIN reports r ON r.image_sha256 = i.report_sha256
      WHERE r.member_id = $1 AND i.name = $2
        AND ($3 = '' OR i.unit = $3)
      ORDER BY r.report_date ASC NULLS LAST, r.image_sha256 ASC, i.id ASC
     ```
   - `TrendPoints` 扫描；空日期 → nil；`*storage.Store` 实现 `pipeline.TrendStore`。
   - 与 CLI 的 `TrendByName` 语义不同（CLI 全局、无 unit/ref/method/review；HTTP 成员 + unit + 更多字段），本期不合并；注释写清，避免以后误当重复实现删掉。
4. **依赖与路由**：`cmd/hdd/commands.go` 加 `trends pipeline.TrendStore`；`config.go` 注入同一 `*storage.Store`；`serve.go` 注册 `GET /api/v1/members/{member_id}/indicators/{name}/trend`。
5. **handler**：`cmd/hdd/serve_trend.go`
   - `strconv.ParseInt` 解析 member_id；失败/成员不存在 → 404。
   - `r.PathValue("name")`；Go mux 解码 `%2F`，带 `/` 的规范名可用。
   - `r.URL.Query().Get("unit")`。
   - 映射 `httpapi.TrendPoint`；`Points` 初始化为空 slice。
   - `reviewed = review_status ∈ {confirmed, corrected}`。
   - 存储错误 → 500 `list trend failed`。
6. **测试**：
   - `cmd/hdd/serve_trend_test.go`：200、中文名 / `谷草/谷丙比值`、unit 门控、未知成员与非法 member_id 404、空数组、reviewed、500。
   - `internal/storage/store_trend_test.go`：真库 smoke，成员隔离、unit 门控、排序 / dateless、字段透传。
   - `internal/httpapi/contract_test.go`：`IndicatorName` 的 `in/required/minLength` 断言；`NotFound` 描述断言。

### 4.3 App

1. **API**：`apps/mobile/src/api/reports.ts`
   - `export type TrendPoint = components['schemas']['TrendPoint']`；`TrendResult`。
   - `getTrend(client, memberId, name, unit?)`：`client.GET('/members/{member_id}/indicators/{name}/trend', {params: {path: {member_id, name}, query: unit ? {unit} : undefined}})`；`unwrap`。本期 hook 不传 `unit`（取全部单位，供 `selectSeries` 过滤/计数）。
2. **纯逻辑**：`apps/mobile/src/reports/trends.ts`
   - `isAbnormalFlag`。
   - `selectAbnormalCandidates(reports, details)`：先过滤 `status !== 'failed'` → `latestByCategory(reports)` → 过滤 `flag_counts` → 按 report_id 取 detail → 筛异常项 → 按 `name + unit` 去重（保持类别 / 项序）→ `{reportType, reportId, reportDate, item}`；候选数 = 组头计数，0 候选的组不输出。
   - `selectSeries(points, unit, candidateReportId)`：同单位（`unit ?? null` 归一）→ 按 report_id 去重 → 日期升序 / dateless 沉底 → `listPoints`（全部）、`chartPoints`（dated + 标量）、`undatedCount`、`nonNumericCount`、`otherUnitCount`、`totalCount`、`selectedPoint`（候选 report_id 对应点；找不到时回落到最新点）。
   - `chartLayout(series, width, height, {axes, selectedIndex?})`：x 按日期、同日期点微偏移；y 域含 ref；非负从 0 起；detail 轴取整；band 分段；单侧 / 标量 ref 复用 `parseRefRange`；marker `filled`；detail 放大 `selectedIndex`。
   - `formatSeriesMeta(series, candidate)`：上述元信息文案（`份报告` / `缺日期` / `非数值指标项` / `单位不同（未连线）`）。
   - `trendUnavailableReason(series, candidate)`：最近一次非数值 / 缺日期 / 无可绘制点。
3. **数据加载**：`apps/mobile/src/reports/useAbnormalTrends.ts`
   - 输入 `(memberId, reports)`；`reports === undefined` / `memberId === undefined` → idle。
   - `reports.length === 0` → ready + `groups: []`。
   - 解析 client；对异常类别 `Promise.all(getReport)`；`selectAbnormalCandidates`；对唯一 `name+unit` `Promise.all(getTrend)`（不带 `unit`）；按类别组装 `groups`（0 候选的组不输出）。
   - 状态 `idle | loading | unconfigured | error | ready`；页面级错误；`active` flag 防 stale 写入。
   - 不缓存在全局；切换成员 / 聚焦由 `useMemberReports` 触发重载。
4. **图表**：`apps/mobile/src/reports/TrendChart.tsx`
   - `react-native-svg`；`TrendSparkline`（64，无轴，放大最新点）与 `TrendDetailChart`（约 180，有轴，放大选中点）共用 `chartLayout`。
   - `onLayout` 测宽；默认 320；testID 由调用方传入。
5. **详情层**：`apps/mobile/src/reports/TrendDetailSheet.tsx`
   - `Modal` + `SafeAreaView`（`react-native` 的或补 `SafeAreaProvider`）；头部返回；完整图、点卡、逐点列表、图例；详情头 `最新：{report_type} · {unit}`。
   - 内部 `selectedIndex` 状态；默认 `series.selectedPoint`；图同步放大选中点。
   - `onRequestClose` 处理 Android 返回与 Web ESC；Web 浏览器后退离开 `/trends`（本期接受）。
6. **页面**：`apps/mobile/src/app/trends.tsx`
   - `useCurrentMember` + `useMemberReports` + `useAbnormalTrends`。
   - 七态；成员 chips；分组（组头 `{report_type} · {最新报告日期} · {N} 项异常`）；行；详情层。
7. **入口 / 标题 / 色板**：`app/index.tsx`、`app/_layout.tsx`、`reports/components.tsx` 或 `reports/colors.ts`（`FLAG_COLORS`）、`review/components.tsx`（改为 import）。
8. **依赖**：`cd apps/mobile && npx expo install react-native-svg`；提交 `package.json` / `package-lock.json`。

### 4.4 依赖与工具

- 新增唯一依赖：`react-native-svg`，用 `npx expo install react-native-svg` 安装。
- Expo SDK 57 文档确认：`react-native-svg` 包含在 Expo Go，支持 Android / iOS / Web；不需要 dev build 或 config plugin。
- 不新增服务、不新增环境变量、不新增数据库迁移。

### 4.5 请求预算与失败策略

- 详情请求：仅异常类别的最新非 failed 报告（通常 ≤ 6–8 份）。
- 趋势请求：唯一 `name+unit` 指标（通常 ≤ 10–20 个）。
- 失败：任一请求失败 → 整页 `error` + 重试；不做逐行局部错误。
- 不做客户端缓存；页面聚焦 / 成员切换即重载。

---

## 5. 改动面

约 30 个文件，一半是测试 / 生成 / 文档；生产逻辑集中在后端 handler/storage 与 App 的 trends 页 / 两个组件。

| 文件 | 改动 |
|---|---|
| `schemas/openapi.yaml` | 补 `IndicatorName` path 参数 + NotFound 描述 |
| `apps/mobile/src/api/schema.d.ts` | codegen 重新生成 |
| `apps/mobile/src/api/reports.ts` | `getTrend` + 类型 |
| `internal/pipeline/types.go` | `TrendStore` / `TrendPointRow` |
| `internal/storage/store.go` | `TrendPoints` 查询 / 方法 |
| `cmd/hdd/commands.go` | `deps.trends` |
| `cmd/hdd/config.go` | 注入 trends |
| `cmd/hdd/serve.go` | 注册趋势路由 |
| `cmd/hdd/serve_trend.go` | 新增 handler |
| `cmd/hdd/serve_trend_test.go` | 新增 handler 测试 |
| `internal/storage/store_trend_test.go` | 新增真库 smoke |
| `internal/httpapi/contract_test.go` | `IndicatorName` / `NotFound` 断言 |
| `apps/mobile/src/reports/trends.ts` | 新增纯逻辑 |
| `apps/mobile/src/reports/trends.test.ts` | 新增单测 |
| `apps/mobile/src/reports/useAbnormalTrends.ts` | 新增加载 hook |
| `apps/mobile/src/reports/TrendChart.tsx` | 新增迷你 / 完整图 |
| `apps/mobile/src/reports/TrendDetailSheet.tsx` | 新增详情层 |
| `apps/mobile/src/reports/components.tsx`（或新增 `colors.ts`） | `FLAG_COLORS` 单点化 |
| `apps/mobile/src/review/components.tsx` | 复用 `FLAG_COLORS` |
| `apps/mobile/src/app/trends.tsx` | 新增页面 |
| `apps/mobile/src/app/index.tsx` | 首页入口 |
| `apps/mobile/src/app/_layout.tsx` | 标题 |
| `apps/mobile/e2e/support.ts` | seed + testID |
| `apps/mobile/e2e/tests/trends.e2e.ts` | 新增 E2E |
| `apps/mobile/e2e/compose.yml` | 回放样本追加 |
| `docs/agents/app-e2e.md` | 14/14 → 15/15 |
| `CONTEXT.md` | 术语：异常指标 / 趋势 / 趋势点 / 信任标记；报告类别示例 |
| `apps/mobile/package.json` / `package-lock.json` | `react-native-svg` |
| `.scratch/abnormal-trends/spec.md` | 本需求文档 |

---

## 6. 关键决策（含被否方案）

| # | 决策 | 理由 | 被否方案 |
|---|---|---|---|
| 1 | 按 `report_type` 分组；每类取最新非 failed 报告；组头带最新报告日期 | 现有数据模型唯一可用的类别键；failed 报告零指标项，不让整类消失；日期让旧类别的新鲜度可见。 | 客户端维护指标名→大项映射（重复词典、易漂移）；识别端新增 panel 字段（跨契约/golden/存量，范围过大）；旧/新类别映射合并（范围过大）。 |
| 2 | 每类最新非 failed 报告 + 只列 high/low/abnormal；组头计数 = 去重后行数 | 回答「现在有什么值得留意」；请求量有界；组头与列表一致。 | 扫全部历史找每个指标最新值（N 份详情或新聚合端点）；历史异常永久保留（列表膨胀，需要忽略机制）；组头直接用 flag_counts（去重后对不上）。 |
| 3 | B：迷你图 + 全屏详情层；详情放大选中点（默认最新） | 用户看过原型后选择；列表保持轻盈，详情提供日期 / 方法 / 逐点 / 参考范围；复用已加载数据；图和点卡焦点一致。 | A 只有迷你图（信息不足）；C 每项完整图（页面过长）；独立路由（URL 编码 / 重复请求 / 额外状态）；始终放大最新点（点 2017 时图和卡不一致）。 |
| 4 | 只画有日期 + 标量点 | 区间 / 不等式 / 文本画线会误导；这些值仍出现在最新值和逐点列表。 | 把不等式按边界值画点（伪造精确值）；用区间带代替点（图复杂且本期无需求）。 |
| 5 | 实心 / 空心信任标记 | 不隐藏低置信数据，也不冒充可信；与详情页核对语义一致。 | 隐藏低置信点（CLI 的策略，对家长过强）；全部实心（与产品灵魂冲突）。 |
| 6 | 连接键 = 报告中的指标项名称 + 归一单位；跨单位不连线，中性提示「另有 N 份单位不同」 | M4 规格 1.3 定稿；单位印形归一在 ingest 已完成（但有缺口，见风险）；无运行时换算；中性计数不撒谎。 | 跨单位换算（系数表维护与医学语义风险）；按报告类别限定趋势（同名指标跨类别会被截断）；跨单位完全不提示（「仅 1 份报告」会与事实不符）。 |
| 7 | 详情层用页面内 Modal，不新增路由；ESC/Android 返回可关，浏览器后退离开页面 | 复用已加载的点；避免指标名含 `/` 进 URL；Android 返回可控；Web 后退的代价本期接受。 | 独立路由（需要重新取数或全局缓存；深链本期无需求）；为后退关层引入 Expo Router modal route（更大）。 |
| 8 | 不新增批量趋势端点 | 当前 20–50 份/年/成员，异常类别 ≤ 6–8，异常指标通常 ≤ 10–20；LAN 下 N+1 可接受。 | 新增聚合端点（契约变更、双实现风险）；客户端全量拉详情（请求更多）。 |
| 9 | 页面级错误 + 重试 | 同一端点、同一 LAN；失败通常是后端不可达，不是单条数据问题。 | 逐行局部错误（UI 复杂，收益低）。 |
| 10 | `react-native-svg` 手绘折线 | 只需折线 + 点 + 参考带；完整图表库依赖重、控制力弱；Expo Go 内置、Web 支持。 | Victory / gifted-charts（额外依赖、样式与信任标记控制差）；纯 View 旋转画线（脆弱）。 |
| 11 | 术语：异常指标 = high/low/abnormal；UI 用「份报告」不用「记录」 | CONTEXT 的「报告」明确 _Avoid_: 记录；「未知」明确 _Avoid_: 异常；统一语言。 | 继续用「N 次记录」（与词表冲突）；只改 spec 不改 CONTEXT（术语继续漂移）。 |
| 12 | 线性轴 + 参考带可压缩；参考范围文本始终可见 | 极端值 27.4→835 下线性轴不可能让 7-40 的带不被压没；点卡的「参考 7-40」是精确来源。 | 对数轴（前期已否）；为带单独做归一化条（本期不做）；删掉参考带（M4 要求保留）。 |
| 13 | 同日期同类别按 `report_id` 升序；本期不加 `created_at` | 现有 `sortReports` 与趋势 SQL 都是这个稳定序；无迁移/契约改动。 | 加 `created_at` 到 `ReportSummary`（契约 + codegen + 首页/档案排序一起改）；按指标项数等代理排序（任意）。 |
| 14 | 契约：非法 member_id 404；删除 NotFound 的「trend name 未收录」；`name` 用 `IndicatorName` 组件 | 契约只声明 200/404；不做词典检查；与其他 path 参数风格一致。 | 非法 id 返回 400（与兄弟端点一致，但要扩契约）；做词典检查（范围大）；内联 `name` 参数（风格不一致）。 |
| 15 | 组头计数 = 去重后行数；详情头 `最新：{report_type} · {unit}` | 组头与列表一致；趋势端点是成员维度，类别只标最新来源。 | 组头用 flag_counts（对不上）；详情头只写 `{report_type} · {unit}`（暗示类别限定）。 |
| 16 | `FLAG_COLORS` 放 `reports/components.tsx` 或 `reports/colors.ts` | `status.ts` 是纯逻辑；`components.tsx` 已有 `STATUS_COLORS`；避免展示色进逻辑模块。 | 放 `status.ts`（初稿，职责混杂）。 |

---

## 7. 怎么验收

### 7.1 自动验证

```bash
go test ./...

# 一次性 PG，跑 storage smoke；不设 HDD_TEST_DSN 则按仓库惯例 skip
docker run --name hdd-trend-smoke-pg -e POSTGRES_PASSWORD=smoke -p 55435:5432 postgres:17-alpine
HDD_TEST_DSN='postgres://postgres:smoke@localhost:55435/postgres?sslmode=disable' \
  go test ./internal/storage -run Trend

cd apps/mobile
npm run codegen
npx expo install react-native-svg
npm run typecheck
npx expo lint
npm test
npm run e2e
```

### 7.2 单元测试（App）

`cd apps/mobile && npm test`，新增 `reports/trends.test.ts`。测试数据用真实 golden（lft_05 / lft_02 / lft_04 / cbc_05 / cbc_07），不用凭空数字：

- `selectAbnormalCandidates`：
  - 只留 high/low/abnormal；normal/unknown 排除。
  - 每类只取最新非 failed 报告；旧报告 / failed 最新报告的异常不出现。
  - 按 `name+unit` 去重；跨类别保留最近类别；保持 item 序；0 候选的组不输出。
  - 组头计数 = 去重后行数（不是 flag_counts）。
  - 无异常类别 / 无 detail / 空 reports。
- `selectSeries`：
  - 同单位过滤；null 单位；跨单位排除并计入 `otherUnitCount`。
  - 按 report_id 去重；日期升序；dateless 计数 / 沉底。
  - 标量 / 区间 / 不等式 / 文本；`chartPoints` 只含 dated scalar。
  - 单点；候选点默认选中；候选点缺失时回落最新点。
  - 改名/改单位后序列分裂（用真实修正对形状的合成数据）。
- `chartLayout`：
  - 2 点 / 1 点 / 同日期微偏移；非负从 0 起；y 含 ref；band 分段 / 合并。
  - 单侧 ref（`<`/`≤`/`>`/`≥`）与标量 ref；band 分段边界。
  - 实心 / 空心；detail 轴取整；selectedIndex 放大。
- `formatSeriesMeta` / `trendUnavailableReason`：单点、缺日期、非数值、单位不同、零可绘制点。

### 7.3 Go

```bash
go test ./...
```

- handler 测试：200 形状、中文名 / `谷草/谷丙比值`（URL 解码）、unit 门控、未知成员与非法 member_id 404、空数组、reviewed 映射、500。
- contract 测试：趋势端点声明 `name` path 参数（`in/required/minLength`）；`NotFound` 描述不含「trend name 未收录」。
- storage smoke（真库，命令见 §7.1）。

### 7.4 App E2E

新增 `apps/mobile/e2e/tests/trends.e2e.ts`，独立 seed。**一条串行用例，5 组断言（8 个步骤，见票 05）**：

- 多点成员：`lft_05`（2017-11-18）+ `lft_02`（2021-08-02），生化，最新 6 项异常；丙氨酸转氨酶 27.40 → 835 U/L。
- 单点成员：`lft_04`（2011-07-02），生化，3 项异常；总胆红素 41.00 μmol/L 偏高。
- 无异常成员：`lft_03`（2026-06-16），生化，0 项异常。
- 无报告成员：只建成员。

断言组：

1. 首页点「异常指标趋势」进入；断言 `生化 · 2021-08-02 · 6 项异常`、`丙氨酸转氨酶 835 U/L 偏高`、`2 份报告`。
2. 打开详情：完整图、`最新：生化 · U/L`、`方法：—`（lft_02/04/05 的 method 为空；lft_03 有 method，但本用例不用）、逐点列表；点 2017-11-18 行 → 27.40 U/L 正常；关闭。
3. 单点成员：`生化 · 2011-07-02 · 3 项异常`、`仅 1 份报告`；详情点列表 1 行。
4. 无异常成员：`这位成员暂时没有异常指标。`
5. 无报告成员：`这位成员还没有检查单。`

新增 testID 进 `e2e/support.ts`：`overview-trends`、`trend-category-${index}`、`trend-indicator-${index}`、`trend-sparkline-${index}`、`trend-detail`、`trend-detail-chart`、`trend-detail-close`、`trend-point-${index}`。

`e2e/compose.yml` 回放列表追加 `lft_02,lft_03,lft_04,lft_05`；`docs/agents/app-e2e.md` 14/14 → 15/15。

### 7.5 手工验收

- Expo Web 先验，再 Expo Go。
- 2 点 / 1 点 / 缺日期 / 非数值 / 跨单位（含中性计数）/ 低置信空心 / 已核对实心 / 选中点放大。
- 正常项、only-unknown 类别不出现；切换成员数据同步；首页入口可见；failed 最新报告不让整类消失。
- 详情层返回、Android 物理返回、Web ESC、Web 浏览器后退（离开 `/trends`）、滚动性能。
- 极端值（27.4 → 835）图可读；参考范围文本始终可见，淡带允许压缩。
- 三端渲染一致（iOS / Android / Web）。

### 7.6 完成判据

- `/trends` 按当前成员展示 `report_type` 分组；每类最新非 failed；组头带日期与去重后行数；只列 `high/low/abnormal`。
- 每个异常指标有同单位、按日期的迷你趋势；详情层能看完整图、`method`、参考范围、逐点数值与信任标记；选中点放大。
- 跨单位不连线，中性提示「另有 N 份单位不同（未连线）」；`normal/unknown` 不出现；低置信点不隐藏。
- 所有空态 / 错误态可用；切换成员正确刷新；failed 最新报告不让整类消失。
- `go test ./...`、`npm test`、`npm run typecheck`、`npx expo lint`、`npm run e2e` 全绿；既有功能无回归。

---

## 8. 风险与未决

| 风险 | 影响 | 应对 |
|---|---|---|
| `report_type` 标签与用户心理分组不一致 | 页面显示「生化 / 激素」而不是「肝功 / 甲状腺」 | 用户已选本期按报告类别；组头带最新报告日期，旧类别的新鲜度可见；后续若真实数据证明不够，另立指标级 panel 需求。 |
| 单位归一不完整 | 同名同物理单位的点被拆成两条序列；份数少报 | 本期文案改「N 份报告」并中性提示「另有 N 份单位不同（未连线）」；另立 `unit_families.yaml` 覆盖需求（`*10~12/L`、`*10`、`IJ`、`U/L`↔`IU/L`、`ng/ml`↔`ng/mL` 等）。 |
| 趋势连接键可变（改名/改单位） | 家长修正后旧点接不上，序列变短 | 本期接受；风险表与 CONTEXT 写清；另立不可变 `item_key` 需求。 |
| 参考带在极端值下压缩 | 带只剩细线 | 判据改为「参考范围文本始终可见，淡带允许压缩」；点卡保留精确文本；不做对数轴（本期无需求）。 |
| 同日期同类别「最新」不是按上传时间 | 可能选到先上传的那份 | 本期按 `report_id` 升序稳定排序；文档写清；需要真实上传时间时再加 `created_at`。 |
| 最新报告 failed | 整类可能消失 | 每类取最新非 failed 报告；组头带日期；failed 报告仍在档案里可见。 |
| 数据密度低 | 很多指标只有 1 份报告，趋势退化为一个点 | 单点显示「仅 1 份报告」；不画假线；后续观察真实使用。 |
| N+1 请求 | 类别多 / 异常项多时首屏变慢 | 当前量级可接受；实测 >2s 再评估批量端点。 |
| 非标量 / 缺日期 | 这些点不能画线 | 图区中性说明；逐点列表仍展示；不缺数据。 |
| `react-native-svg` 平台差异 | iOS / Android / Web 渲染差异 | 官方支持三端；手工三端验收；不引入第二套图表实现。 |
| Android 物理返回 / Web ESC | 详情层可能被系统返回键绕过 | 用 `Modal` 的 `onRequestClose` 关闭详情层；RNW 的 Modal 已处理 ESC；手工验证。 |
| Web 浏览器后退 | 后退离开 `/trends` 而不是关层 | 本期接受；手工验收写清；需要后退关层时再评估 Expo Router modal route。 |
| 趋势端点空结果语义 | 200 + `points: []` 与 404 的边界 | 成员不存在或 id 非法才 404；空结果 200；App 只请求已知指标名。 |

**未决（不阻塞本期）：** 指标级检查大项、完整 M4 独立趋势页 / 深链、批量趋势接口、跨单位换算、单位族覆盖补全、不可变 `item_key`、同日期按 `created_at` 排序。均为明确推迟项，有后续触发条件。

---

## 9. 参考

- 统一语言：`CONTEXT.md`（报告 / 报告类别 / 指标项 / 异常指标 / 趋势 / 趋势点 / 信任标记 / 成员 / 家长 / 核对 / 未知）
- 需求：`docs/design/2026-10-13-viz-and-client-spec.md` §5.2「指标趋势」、§5.3 可视化规格、M4
- ADR-0005（最终生效值 / 核对状态）、ADR-0006（局域网先行）、ADR-0010（flag abnormal）
- 契约：`schemas/openapi.yaml`（`/members/{member_id}/indicators/{name}/trend`）
- 原型：`.scratch/abnormal-trends/prototype.html`（A/B/C 切换；B 点「查看趋势」打开详情层）
- 事实核查：`recognizer/recognizer/unit_families.yaml`、`internal/pipeline/review.go`、`apps/mobile/src/reports/order.ts`、共享栈真实数据（2026-10-23）
- 样张数据：`samples/expected/report/{lft_02,lft_03,lft_04,lft_05}.json`
- 现有实现：`cmd/hdd/serve_reports.go`、`internal/storage/store.go`、`apps/mobile/src/reports/{overview,status,refRange}.ts`、`apps/mobile/src/app/{index,timeline}.tsx`
- 实施票：`.scratch/abnormal-trends/issues/01-contract-name-param.md` … `05-app-e2e-and-docs.md`

---

## 10. Tickets

- `01-contract-name-param.md`：OpenAPI 补 `IndicatorName` 路径参数 + 修正 NotFound 描述 + codegen + contract 断言。
- `02-backend-trend-endpoint.md`：`TrendStore` / storage / handler / 路由 / 测试。
- `03-trends-page-and-entry.md`：App API / 纯逻辑 / hook / 迷你图 / `/trends` 页（组头日期 + 行数）/ 首页入口 / 单测。
- `04-trend-detail-sheet.md`：完整图 / 详情 Modal / 点卡 / 逐点列表 / 选中点放大 / 图例。
- `05-app-e2e-and-docs.md`：E2E seed / replay / 用例 / 文档计数 15/15。

> 仓库规则「一个需求保持 1 次 commit」：以上 5 张票是实施拆分，不单独合并；最终一个 commit 同时包含 01–05 的代码与测试。票 03 单独实现时不渲染 `查看趋势`，票 04 接上详情层后再出现入口，避免 dead link。
