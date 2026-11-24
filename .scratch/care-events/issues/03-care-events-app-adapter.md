# 03: App HTTP adapter 与跨设备刷新

**What to build:** 把 App 的护理事件数据源从内存 adapter 换成 HTTP：`api/careEvents.ts`、`HttpCareEventRepository`、根布局显式注入；今天 / 统计页保留 focus 重载并增加下拉刷新；RNTL 覆盖 loading / error / 刷新 / 创建修改删除 / 孩子隔离。

**Blocked by:** 02、care-log 的睡眠补记票（跨 feature，避免 A1 先用旧 UI 持久化进行中睡眠）。

跨 feature 依赖：[09-sleep-completion](../../care-log/issues/09-sleep-completion.md)。

**Status:** ready-for-agent

## 验收清单

- [ ] `apps/mobile/src/api/careEvents.ts`：list / create / update / delete 四个封装，wire 类型来自 codegen，映射到 `care/careEvents.ts` 的手写领域类型。
- [ ] `HttpCareEventRepository` 实现现有 `CareEventRepository`：每次调用 `resolveApiClient()`；未配置后端抛可识别错误（「尚未配置后端地址」）；服务端错误转 `Error(message)`。
- [ ] `CareEventProvider` 的 `repository` 必填：生产根布局注入 HTTP repository，测试 `renderScreen` 注入 `InMemoryCareEventRepository`；删除生产默认内存仓储。
- [ ] 今天页时间线、统计页滚动容器增加下拉刷新；focus 重载保留；刷新失败保留旧数据并显示现有错误态。
- [ ] 创建 / 修改 / 删除后仍刷新今天时间线、hero、摘要；跨孩子事件仍按 `member_id` 隔离。
- [ ] 无本地缓存、乐观更新、离线队列；后端不可达时显示错误，不静默回落到内存 / fixture。
- [ ] RNTL：HTTP repository 未配置错误、服务端错误、下拉刷新调用 reload、创建修改删除、孩子切换、fixture 不进入生产默认。
- [ ] `npm run typecheck`、`npm test -- --runInBand`、`npm run lint -- --max-warnings=0`、`npx expo export --platform android` 全绿。
- [ ] 不改 Go / Python / DB / OpenAPI（票 02 已完成契约）。

## 实现提示

- 保持 `CareEvent` 为 UI / 领域 seam，不把生成类型的 union 渗进 `TodayCareLog` / `StatsCareLog` / `RecordSheet`。
- `HttpCareEventRepository` 建议构造参数 `resolveClient = resolveApiClient`，便于测试注入 fake；不要让 provider 在模块加载时解析 Base URL。
- 今天页的 `ScrollView testID="today-timeline"` 加 `refreshControl`；统计页的整页滚动同理。刷新指示器颜色沿用主题。
- 失败文案复用现有 `今日` / `统计` 错误态；未配置后端由 `DailyPlaceholder` 的成员加载先呈现，care 层不要再发明第二套配置引导。
- 根布局注入后，`CareEventProvider.test.tsx` 需要改为显式 repository；`renderScreen` 已支持注入，保持测试默认空仓储。
- 提醒重排在票 04，不在本票改系统通知；本票只保证数据读取正确。
- 真机 smoke 放票 05；本票至少跑 `expo export` 与 RNTL。
