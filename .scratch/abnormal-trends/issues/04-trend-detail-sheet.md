# 04: 趋势详情层（完整图 + 逐点数值 + method + 信任标记）

**What to build:** B 的第二半：点 `/trends` 行的「查看趋势」打开全屏详情层，展示完整趋势图（日期轴、参考带、实心/空心）、选中点卡片（日期/值/参考范围/flag/method/信任）、逐点列表与图例。复用列表已加载的趋势点，不新增请求；不做独立路由/深链。

**Blocked by:** 03。

**Status:** ready-for-agent

## 验收清单

### 详情层

- [ ] `apps/mobile/src/reports/TrendDetailSheet.tsx`：
  - `Modal`（`animationType="slide"`、`transparent={false}`、`onRequestClose` 关闭）+ `SafeAreaView`（`react-native` 的，或补 `SafeAreaProvider`）；详情层根 testID `trend-detail`。
  - 头部返回 `‹`（testID `trend-detail-close`）+ 指标名；元信息 `最新：{report_type} · {unit ?? '无单位'}`。
  - 完整图（testID `trend-detail-chart`）：日期轴首尾标签；3 个取整 y 刻度；参考带分段；实心/空心；选中点放大（默认最新）；不可画时 `暂无可用数值趋势` + 中性原因（最近一次非数值 / 缺日期 / 无可绘制点）。
  - 选中点卡片：日期或 `日期未知`；值 + 单位；`参考 ...` 或 `无参考范围`；flag 文案；`方法：{method ?? '—'}`；信任标签 `未核对低置信（空心）` / `已核对（实心）` / `高置信（实心）`。
  - 逐点列表（testID `trend-point-${index}`）：同单位、日期升序、dateless 沉底、按 report_id 去重；每行实心/空心点 + 日期 + 值/单位 + flag；点行切换选中点；默认选中候选指标对应的最新报告点；图同步放大选中点。
  - 图例：`实心 = 高置信 / 已核对`、`空心 = 未核对低置信`。
  - 元信息补充 `另有 N 份缺日期` / `另有 N 个非数值指标项` / `另有 N 份单位不同（未连线）`。
  - 不做跨单位换算/连线；只做中性事实计数；不做诊断/建议；不显示「未识别」等推断文案。
  - 原型底部的「跨单位族不连线、不提示」是开发术语，家长侧不展示；只留图例和点按提示。
  - `onRequestClose` 处理 Android 返回与 Web ESC；Web 浏览器后退离开 `/trends`，不关层（本期接受）。
- [ ] `/trends` 行 onPress 打开详情层；关闭后回到列表原滚动位置（Modal 不卸载列表）。
- [ ] 详情层不额外请求：直接消费 `useAbnormalTrends` 已加载的 `series/listPoints`。

### 完整图

- [ ] `TrendChart.tsx` 增加 `TrendDetailChart`（约 180 高），与迷你图共用 `chartLayout`：
  - x 按日期；同日期点微偏移；单点居中。
  - y 域含 ref；非负从 0 起；轴刻度取整（0 / 中 / 上）；单侧 / 标量 ref 复用 `parseRefRange` / `stripLayout` 口径。
  - 参考范围按点分段；无范围的分段不画。
  - 线中性灰；点 flag 色；放大选中点（默认最新）。
  - 首尾日期标签；单点显示一个居中日期。
- [ ] `chartLayout` 的详情模式测试：刻度取整、同日期微偏移、单点、band 分段、单侧 / 标量 ref、实心/空心、`selectedIndex` 放大。

### 测试

- [ ] 纯逻辑单测扩展到详情模式（`axes: true`、`selectedIndex`）。
- [ ] `npm run typecheck`、`npx expo lint`、`npm test` 全绿。
- [ ] 手工（Expo Web + Expo Go）：打开/关闭、Android 物理返回、Web ESC、Web 浏览器后退（离开 `/trends`）、点列表切换、选中点放大、method 有值/为空、低置信空心、非数值/缺日期、跨单位中性计数、长列表滚动。
- [ ] 不新增依赖；不新增路由；不改后端。

## 实现提示

- 复用 `selectSeries` 的 `listPoints` / `chartPoints` / `selectedPoint`；详情层不再请求 `getTrend`。
- 点列表是主要选点方式；不要给 SVG 小圆点做点击热区（移动端误触、无障碍差）。图表本身静态，点列表可点；`selectedIndex` 驱动图和点卡。
- `Modal` 的 `onRequestClose` 在 Android 物理返回时触发关闭；RNW 的 `Modal` 已在 ESC 时调用它；iOS 用头部返回。浏览器后退不关层，本期接受。
- `SafeAreaView`：`react-native-safe-area-context` 已在依赖里但没有 `SafeAreaProvider`；用 `react-native` 的 `SafeAreaView`，或在 `_layout.tsx` 补 provider。不要新增依赖。
- 完整图的 y 轴取整：非负数据从 0 起；极端值（如 27.4 → 835）用 0/500/1000 这类刻度，参考带可能很薄，这是可接受的事实呈现；判据是「参考范围文本始终可见」，不是「带不被压没」。
- 逐点列表包含非标量点；只有 `chartPoints` 参与画线。非标量点仍显示原文值。
- 信任标签不要合并成「高置信 / 已核对」：`reviewed=false, low_confidence=false` 是「高置信（实心）」，不是「已核对」。
- 详情层不做深链、不做分享、不做 PDF；完整 M4 独立页留后续。

## Comments

（待实现）
