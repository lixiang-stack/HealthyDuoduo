# 06: 提醒入口与本地通知

**What to build:** 提醒不是 tab：今天页保留「喂奶间隔提醒」卡片，点进提醒设置页；默认关闭、用户设定 2 / 3 / 4 小时间隔、只在本机调度本地通知。统计页不放提醒卡片。不做云推送、不承诺准时、不提供喂养建议。

**Blocked by:** 01（护理事件契约）、02（导航壳）、03（当前孩子上下文）。

**Status:** ready-for-agent

## 验收清单

- [x] 今天页卡片显示开启状态与「上次/下次」；点击进入提醒设置；底部导航没有提醒 tab。
- [x] 统计页不出现提醒卡片（移除 / 不添加）。
- [x] 设置页：开关默认关闭；间隔 2 / 3 / 4 小时；从上次喂养开始计时；显示下次提醒时间。
- [x] 开启时请求通知权限；权限拒绝时保持关闭并说明原因；不静默失败。
- [x] 只调度本地通知（`expo-notifications` 或等价方案）；不接远程推送、不上传设备 token。
- [x] 记录新的喂养后取消旧通知并按新时间重排；关闭提醒时取消全部已调度通知。
- [x] 文案只陈述时间（「距上次喂养已 3 小时」），不写「该喂了」或喂养量建议。
- [x] 已知限制写清：不承诺准时；Android 精确闹钟 / 省电策略、iOS 权限都会影响到达。
- [x] RNTL：卡片 / 设置入口、默认关闭、间隔切换、权限拒绝、开关取消调度（mock notifications）。
- [x] Android 真机 smoke：权限、调度、取消、重启 App 后状态一致。

## 实现提示

- 现有仓库未安装 `expo-notifications`；本票按 Expo SDK 58 的能力接入，改 `apps/mobile` 依赖与 app config。
- 调度是本地能力，不需要 Go / Python 改动；不要把提醒写进后端或分析快照。
- 上次喂养时间来自 01 的读模型；没有记录时显示「开启后从上次喂养开始计时」，不伪造时间。
- 睡眠 / 尿布提醒不在本票；页面上可标「以后」，不要做成可点击的假入口。
- 如果权限 / 精确闹钟验证不通过，降级为「只显示距上次喂养」事实卡片，不承诺通知。


## Comments

**实现（2026-11-24）：**
- 新增 `apps/mobile/src/reminders/`：
  - `reminderSettings.ts`：本机 AsyncStorage 设置，默认关闭、间隔 3 小时，只接受 2 / 3 / 4；坏数据回默认。
  - `reminderSchedule.ts`：纯函数从最近喂养时间 + 间隔算 reminder time / overdue / no-feed；通知文案只写「距上次喂养已 N 小时」。
  - `reminderNotifications.ts`：`expo-notifications` 本地调度适配，按日期一次性触发；关闭 / 新喂养时 `cancelAllScheduledNotificationsAsync` 后重排；只创建 Android 通知渠道，不申请精确闹钟、不接远程推送 / token。
  - `useReminderSettings.ts`、`syncReminderToLatestFeed.ts`、`ReminderCard.tsx`。
- 今天页：今日摘要下新增喂奶间隔提醒卡片，显示关闭 / 上次 + 下次 / 已过提醒时间；点击进入栈页 `/reminder`，不是 tab。保存 / 修改 / 删除护理记录后，按最新喂养重排；统计页不渲染提醒卡片。
- 提醒设置页：开关默认关闭；开启先请求系统通知权限，拒绝时保持关闭并显示「通知权限未开启」原因；间隔 2 / 3 / 4 小时可切换，开启时立即重排；关闭时取消全部已调度通知；无喂养记录时只显示「开启后从上次喂养开始计时」，不伪造时间。
- 已过间隔不补发历史通知：设置页 / 卡片显示「已过提醒时间 HH:MM」，下一次喂养重新开始计时。
- 为让「上次喂养」在跨过一整天后仍能显示，`recentCareEventRange` 由「昨天 + 今天」扩为最近 7 天；今天时间线 / 摘要仍只取本机当天。
- Expo Go / Android 渠道创建可能失败；适配器容忍渠道创建失败并回退到默认渠道继续调度，不让权限或本地通知被渠道问题阻塞。真实构建仍优先使用 `feed-reminders` 渠道。
- App config 增加 `expo-notifications` 插件并显式 `enableRemoteNotifications: false`；不申请精确闹钟权限，已知限制在设置页写明。
- 已知限制：护理事件仍是内存 adapter，重启 App 后事件清空；提醒设置本身持久化，重启后「上次喂养」可能回到空态。冷启动不会主动取消已调度通知；下一次保存 / 修改 / 删除护理记录时按最新数据重排。正式 API 落地后上次喂养会随事件持久化。
- 验证：`npm test -- --runInBand`（56 suites / 445 tests）、`npx tsc --noEmit`、`npm run lint -- --max-warnings=0`、`npx expo export --platform android`。
- Android 真机 smoke（Expo Go）：系统通知权限弹窗；开启后状态与「上次 / 下次」正确；切换 2 小时立即重排；新喂养后重排；关闭后 AlarmManager 中不再有本应用提醒；重启 App 后开关与间隔保持。提醒文案只陈述时间，未出现喂养建议。
