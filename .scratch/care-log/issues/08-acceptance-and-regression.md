# 08: 跨票验收与契约回归

**What to build:** 对 01–07 做一次完整验收：B3 导航、当前孩子、记录弹层、重复提示、提醒、统计图表；跑 RNTL / typecheck / Android 真机 smoke，检查没有引入账号 / 记录人归属 / 全局 active child 等越界字段。

**Blocked by:** 01, 02, 03, 04, 05, 06, 07。

**Status:** resolved

## 验收清单

- [x] RNTL 全绿：导航壳、当前孩子、记录目标、重复提示、提醒、统计图表。
- [x] `npm run typecheck`、`npm test` 全绿；按 `apps/mobile/ANDROID.md` 完成真机 smoke。
- [x] 双设备模拟：设备 A 选豆豆、设备 B 选老二，互不覆盖；各自记录带正确 `member_id`。
- [x] 重复提示：同孩子 5 分钟内提示；仍然添加 / 返回修改；跨孩子不误报。
- [x] 提醒：默认关闭、调度、取消、统计页无卡片；不承诺准时。
- [x] 提醒权限拒绝路径：RNTL 覆盖；票 06 真机 smoke 覆盖系统权限弹窗，本票未重复弹窗。
- [x] 统计：4 指标 × 3 图表、空态、单点、孩子切换；无报告数据混入。
- [x] 契约检查：不出现 `recorded_by` / `device_id` / 全局 active child；护理事件不写入 `reports` / `report_items` / 趋势 / 分析快照。
- [x] 若 01 引入数据库迁移：跑 `bash scripts/test-db.sh`；若仅客户端契约，记录不适用。→ 票 01 仅客户端契约，无迁移，不适用。
- [x] `python3 scripts/check-docs.py` 全绿；`CONTEXT.md` 已补「育儿 / 健康档案 / 当前孩子」。
- [x] `docs/adr/0019-care-log-navigation-child-context.md` 与 `../spec.md` 的实现状态 / 差异已更新。
- [x] 无账号 / 邀请 / 云同步 / 远程推送新增；ADR-0006 边界未被突破。

## 实现提示

- 验收以 `../spec.md` §10 为准；本票不新增功能，只做集成、回归与文档收尾。
- Android 真机清单重点：appbar 域 tab、孩子选择器、记录弹层目标、重复提示、底部 tab 状态保留、返回键、统计图表、提醒权限。
- 若 01 的契约仍停留在 adapter / fixture，明确记录「护理事件正式 API 未落地」，不要把 fixture 当生产实现。
- 发现票间口径冲突时，先回写 spec / ADR，再改代码；不要在单票里临时改契约。

## Comments

**验收（2026-11-24，分支 `care-log-acceptance`，基于 `origin/main` `0e549b3`）**

- 自动化：`TZ=UTC npm test -- --runInBand` 57 suites / 447 tests 全绿；`npx tsc --noEmit`、`npm run lint -- --max-warnings=0`、`npx expo export --platform android`、`npm run codegen`（schema 无 diff）通过。
- 文档 / 契约：`python3 scripts/check-docs.py` 159 个文件 OK；`python3 scripts/check-path-case.py` 与 `bash -n scripts/*.sh scripts/git/*.sh` 通过；`CONTEXT.md` 已含护理事件 / 当前孩子 / 育儿 / 健康档案。
- 契约扫描：`rg 'recorded_by|device_id|caregiver|active_child|activeChild' apps/mobile/src --glob '!*.test.*' --glob '!*.d.ts'` 仅命中 `careEvents.ts` 的「刻意不设归属字段」注释；`reports/` 与 `analysis/` 无 `care` 导入；`git diff origin/main -- internal cmd recognizer schemas migrations` 为空。票 01 未引入数据库迁移，`scripts/test-db.sh` 不适用。
- RNTL 覆盖映射：导航壳 / 二级 tab 记忆（`domainTabsLayout`、`DomainTabs`、`DomainNavigationContext`）、当前孩子（`dailyPlaceholder`、`CurrentMemberContext`、`currentChild`）、记录目标（`RecordSheet`、`todayCareLog`）、重复提示（`RecordSheet`）、提醒（`reminder`、`todayCareLog`）、统计 4×3 / 空态 / 单点 / 孩子切换（`statsCareLog`、`careStats`）、无记录人 UI（`todayCareLog`、`RecordSheet`）。

**验收修正：生产 provider 曾把 fixture 当真实记录**

- 真机首次进入今天页时，`CareEventProvider` 默认注入 `CARE_EVENT_FIXTURES`，空数据库也显示「上次 14:05 · 奶粉 120 ml」；冷启动后 fixture 重新出现，违反本票「不要把 fixture 当生产实现」。
- 已先补 `CareEventProvider.test.tsx` 复现（默认仓储应返回 0 条），再把生产默认改为空 `InMemoryCareEventRepository`；fixture 仅保留给测试 / 显式注入。今日空态、冷启动清空与「上次喂养」回落恢复正确。
- 同步更新 `careEventFixtures` 注释、`renderScreen` 注释、`docs/design/2026-11-23-care-event-contract.md`、spec §15 与 ADR-0019 实现状态。

**Android 真机 smoke（隔离验收栈，未触碰共享数据）**

- 准备：一次性 DB / bucket + worktree Metro；创建两个孩子与一名成人；App Base URL 指向隔离栈并通过健康检查。
- 导航：育儿 / 健康档案 appbar 切换与下划线；健康档案底部报告 / 趋势、育儿底部今天 / 统计；切到健康档案再回育儿保留「统计」，再点育儿回「今天」；提醒 / 今日记录 / 家庭成员为栈页且有返回。健康档案待核对圆点本轮未复测（票 02 真机 smoke 已覆盖；RNTL 覆盖圆点条件）。
- 孩子：多孩子首次进入只列孩子并强制选择；appbar 无孩子入口，选择器在育儿页左上角；切孩子后今天 / 统计立即刷新且事件按 `member_id` 隔离；删除一个孩子后自动锁定剩下的单孩子（无箭头）；删除全部孩子后育儿页显示「还没有孩子成员」空态；成人不进入护理目标。
- 记录：喂奶 / 尿布弹层显示「记录给」目标；多孩子 chips 只列孩子；保存后 hero、摘要、时间线刷新；豆豆的喂养不出现在老二时间线，老二的尿布也不出现在豆豆统计。
- 重复提示：5 分钟内第二条喂养弹「刚刚已有一条喂养记录（16:46 · 奶粉 120 ml），仍然添加？」；「返回修改」保留表单且不落库；「仍然添加」落两条；9 分钟后的喂养不再提示。
- 提醒：今天页卡片可进设置；开关、2 / 3 / 4 小时间隔、上次 / 下次时间与通知预览正常；切换 4 小时后 AlarmManager 出现 20:48 的本地通知；关闭后活动闹钟消失；文案只陈述时间。权限弹窗本轮未复测（设备此前已授权，票 06 smoke 覆盖系统弹窗；拒绝路径由 RNTL 覆盖）。
- 统计：4 指标切换；柱状 / 堆叠 / 折线切换；今天高亮；无数据显示「还没有记录」且不给假图；同一数据点照常渲染；统计页无提醒卡片。窄屏 / 长标签布局沿用票 07 真机 smoke；本轮验证空态与图表切换不溢出。
- 冷启动：当前孩子与提醒设置保持；内存护理事件清空，hero 回到空态；收尾恢复 Base URL 为 `http://localhost:8080/api/v1`，停止隔离栈、删除一次性 DB / bucket、移除 adb reverse。

**已知差异 / 未覆盖**

- 双设备“互不覆盖”由 `CurrentMemberContext.test.tsx` 的两个 provider 实例模拟，未使用两台物理设备；`member_id` 隔离由 RNTL 与单台设备切换孩子验证。
- `/today-records` 仍是未接线的占位栈页，「全部记录」入口未实现；今天页时间线承担记录浏览。已在 spec §15 / ADR-0019 记录，建议另开票。
- 护理事件仍为内存 adapter：正式表 / API / 迁移未落地；重启后事件清空，提醒设置保持，下一次记录触发重排。
- 提醒为本机 best-effort，不承诺准点；Expo Go 的浮动 Tools 按钮会遮挡 dev 页面右上角，release 无此浮层。

**后续修复（2026-11-24）：** 育儿域路由由 `/daily` 改为 `/care`，内部域标识同步改为 `care`；`domainNavigationStore` 会把旧的 `/daily` 本地偏好迁移到 `/care`。票 02 / 03 中的 `/daily` 为当时的历史记录，以 spec §9 / §15 与 ADR-0019 为准。真机冷启动后 `/care/today` 正常进入今天页，深链 `/care/stats` 直接打开统计页。
