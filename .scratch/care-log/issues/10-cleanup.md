# 10: 收尾清理（占位路由与时间线滚动 smoke）

**What to build:** 删除没有入口的 `/today-records` 占位栈页与 `_layout` 注册，更新引用；补票 04 欠的“今天页首屏 / 时间线独立滚动”真机 smoke。本期不实现“全部记录”页。

**Blocked by:** None。

**Status:** resolved

## 验收清单

- [ ] 删除 `apps/mobile/src/app/today-records.tsx`，移除 `_layout.tsx` 的 `Stack.Screen name="today-records"`；`rg 'today-records|today_records' apps/mobile/src` 无代码引用（文档中的历史记录可保留）。
- [ ] 更新 `.scratch/care-log/spec.md` §15.8 与 `apps/mobile/ANDROID.md`：占位页已移除；今天页时间线承担浏览；需要“全部记录”时另开票。
- [ ] 不新增“全部记录”入口、不新增底部 tab、不改今天页布局（除非 smoke 发现真实缺陷）。
- [ ] RNTL / `npx tsc --noEmit` / `npm run lint -- --max-warnings=0` 全绿；如有路由测试，删除或更新断言。
- [ ] Android 真机 smoke：今天页孩子入口、hero、四个快捷动作、今日摘要、提醒卡片固定在首屏；时间线在剩余高度内独立滚动；首屏可见行数符合预期；空态仍可滚动。
- [ ] smoke 记录设备 / Android 版本 / Expo Go 版本与“首屏可见几行”；若发现独立滚动或安全区问题，另开 bug 票，不在本票顺手改布局。

## 实现提示

- 删除占位路由是明确决策：它没有入口，spec §15.8 已说“需要时另开票”；不要为了占位而补一个假入口。
- 票 04 的 Comments 记录过“设备断开，待重新连接后补验首屏可见行数与独立滚动”；本票只补这项 smoke，不重做时间线视觉。
- 今天页当前结构是“固定上方 + `today-timeline` 独立 ScrollView”；smoke 重点看键盘收起后、空态、多条事件时是否仍固定。
- 如果 `rg` 命中 docs / spec 中的历史路径，保留；只清代码与测试引用。
- Expo Go 的浮动 Tools 按钮会遮右上角，属于 dev-only；不要为此改产品布局。

## Comments

**实现（2026-11-25）：**

- 删除 `apps/mobile/src/app/today-records.tsx` 与 `_layout.tsx` 的 `Stack.Screen name="today-records"`；`rg 'today-records|today_records' apps/mobile/src` 无代码引用，仅 spec 保留历史说明。
- `.scratch/care-log/spec.md` §15.8 / §9 / §15 与 `apps/mobile/ANDROID.md` 已移除「今日记录」栈页描述，明确时间线承担浏览、需要独立「全部记录」页时另开票。
- 未新增入口 / 底部 tab，未改今天页布局。
- 验证：`npx tsc --noEmit`、`npm run lint -- --max-warnings=0`、`TZ=UTC npm test -- --runInBand`（60 suites / 483 tests）全绿；无路由测试需要更新。

**Android 真机 smoke（2026-11-25，隔离栈）：**

- 设备：Android 16（SDK 36）物理真机，Expo Go 58.0.5；隔离栈 + 12 条当天护理事件。
- 首屏固定：孩子入口 / hero / 四个快捷动作 / 今日摘要 / 提醒卡片都在时间线之上；时间线 `today-timeline` 占 `[66,1485]–[1014,2156]`（1080×2400）。
- 首屏可见：单孩子下时间线内 **3 条完整 + 1 条部分**（11:40 / 11:20 / 11:00 完整，10:40 部分）。
- 独立滚动：在时间线内上滑后，hero / 快捷动作 / 摘要 / 提醒卡 / 时间线 bounds 完全不变，事件行切到 11:00–08:40 区间；确认只有时间线滚动。
- 空态：切到无事件的老二后 `today-empty` 仍显示，时间线继续占满剩余高度；下拉手势不移动固定区，空态仍可刷新。
- 未发现独立滚动或安全区缺陷；不需要另开 bug 票。Expo Go 浮动 Tools 按钮遮挡右上角为 dev-only，不计入。
