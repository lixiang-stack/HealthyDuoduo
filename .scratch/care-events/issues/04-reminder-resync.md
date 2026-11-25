# 04: 提醒跟随当前孩子并重排

**What to build:** 提醒设置仍是每设备一份，但排程必须跟随当前孩子：切换孩子、冷启动后按当前孩子的最新喂养重排；没有喂养则取消已排通知；关闭时取消。统计页切换孩子也要生效，不能只在今天页触发。

**Blocked by:** 03。

**Status:** ready-for-agent

## 验收清单

- [ ] 切换当前孩子后，系统已排通知按新孩子的最新喂养时间重排；旧孩子的通知不残留。
- [ ] 冷启动后按持久化事件重排；最近 7 天无喂养时取消全部已排通知；提醒关闭时不调度。
- [ ] 今天页与统计页都能触发重排（共享育儿域层），不依赖今天页是否挂载。
- [ ] 重排有去重键（如 `childId + lastFeed.started_at`），不会因每次 render / focus 重复调度。
- [ ] 当前 `syncReminderToLatestFeed.ts` 里“mount / child switches do not touch the OS schedule”的注释与实现一起更新，不保留相反说明。
- [ ] RNTL：切换孩子重排、冷启动重排、无喂养取消、关闭不调度、去重；mock `expo-notifications`。
- [ ] Android 真机 smoke（可与票 05 合并）：开启提醒后切换孩子，AlarmManager 中旧通知消失、新时间出现；冷启动不残留旧孩子通知。
- [ ] 文档写清“每设备一份设置、跟随当前孩子”，不承诺准时；不申请精确闹钟、不接远程推送。

## 实现提示

- 复用 `syncReminderToLatestFeed` 的“读 settings + 读最新喂养 + sync”路径；把调用点从“保存记录后”扩到“当前孩子变化 / 冷启动首次加载”。
- 放在 `DailyPlaceholder` 或育儿域共享 hook 中，确保今天 / 统计切换孩子都触发；不要在 `ReminderCard` 的 render 中直接调度。
- 冷启动读取依赖票 03 的 HTTP 数据源。失败语义按 spec §7.5：切换孩子时先取消旧排程，新孩子读取失败保持取消（宁缺勿错）；冷启动读取失败保留既有通知（同一孩子），下次成功加载再校正。两种路径都要写测试。
- `cancelAllScheduledNotificationsAsync` 会取消本 App 全部已排通知；当前只有喂养提醒，可接受；若未来增加通知类型，改为按 identifier 取消。
- 没有喂养时卡片显示「开启后从上次喂养开始计时」，系统通知也应为空；不要用“现在 + 间隔”伪造。
- 提醒文案保持只陈述时间，不写“该喂了”。
