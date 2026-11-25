# 05: 跨票验收与契约回归

**What to build:** 对 care-events 01–04 与 care-log 09 做完整验收：DB / storage / HTTP 契约 / App adapter / 提醒重排；跑自动化、双设备真机 smoke 与契约扫描，确认没有越过 ADR-0006 / ADR-0019 的边界。

**Blocked by:** 01、02、03、04、care-log 的睡眠补记票（跨 feature）。

跨 feature 依赖：[09-sleep-completion](../../care-log/issues/09-sleep-completion.md)。

**Status:** resolved

## 验收清单

- [ ] `go test ./...`、`go vet ./...` 全绿；care 领域校验、storage、handler 测试覆盖四类型与非法路径。
- [ ] `bash scripts/test-db.sh` 全绿，含 migration 0021 up/down 与 `care_events` 约束 / 级联测试。
- [ ] `npm run codegen` 无 diff；`npm run typecheck`、`npm test -- --runInBand`、`npm run lint -- --max-warnings=0`、`npx expo export --platform android` 全绿。
- [ ] `internal/httpapi/contract_test.go` 冻结端点集合包含四个护理事件端点；fixtures 双向对账通过。
- [ ] 契约扫描：`rg 'recorded_by|device_id|caregiver|active_child|activeChild' apps/mobile/src internal cmd --glob '!*.test.*'` 仅命中契约注释；护理事件不被 `reports` / `analysis` / 趋势导入。
- [ ] Android 双设备（或两个 App 实例）指向同一隔离栈：
  - A 创建喂奶 / 尿布 / 睡眠 / 辅食；B 下拉刷新后看到同一批事件，`member_id` 隔离正确。
  - B 在 5 分钟内再记同孩子喂养，触发重复提示；「仍然添加」落两条；「返回修改」不落库。
  - A 修改 / 删除；B 下拉刷新后同步。
  - 冷启动：事件仍在；当前孩子保持；提醒设置保持。
  - 提醒：切换孩子后系统通知按新孩子重排；冷启动不残留旧孩子通知；无喂养时取消。
  - 后端不可达 / 未配置：显示错误态，不静默使用内存或 fixture。
- [ ] 收尾恢复 Base URL、停止隔离栈、删除一次性 DB / bucket；共享 deploy 栈未改。
- [ ] 文档：`docs/design/2026-11-23-care-event-contract.md`（HTTP 落地状态）、`docs/agents/contracts.md`（mirror 行）、`docs/adr/0019-...`（实现状态）、`.scratch/care-log/spec.md` §15（持久化 / 睡眠 / 剩余差异）、`apps/mobile/ANDROID.md`（smoke 清单）更新；`python3 scripts/check-docs.py` 全绿。
- [ ] 无账号 / 邀请 / 云同步 / 远程推送新增；ADR-0006 边界未突破。

## 实现提示

- 以 `../spec.md` §10 为准；本票不新增功能，只做集成、回归、真机验收与文档收尾。
- 双设备优先用两台物理设备；没有第二台设备时，用两个 App 实例 / 两台模拟器并明确记录降级事实，不要把“context 模拟”写成“双设备真机”。
- 提醒的 AlarmManager 检查沿用票 06 / 08 的方法；权限弹窗若设备已授权，记录“未重复弹窗”的事实。
- 发现票间口径冲突时，先回写 spec，再改代码；不要在验收票里临时改契约。
- 护理事件持久化后，重启不再清空；确认 fixture 不会进入生产默认（票 03 已改 provider，仍需真机冷启动验证）。

## Comments

**自动化验收（2026-11-25）：**

- Go：`go test ./...` / `go vet ./...` 全绿。`internal/care` 覆盖四类型合法形状、类型专属字段互斥、数值 / 时间顺序、note / food 规范化；`TestCareEventStoreSmoke` 覆盖四类型、范围重叠（含跨午夜 / 进行中睡眠）、倒序、修改保身份、删除语义与成员级联；`migrate_test` 覆盖 migration 0021 up/down 与 `care_events` CHECK；`serve_care_test` 覆盖创建 / 列表 / 修改 / 删除 / 成人拒绝 / 未知成员 / 未知事件 / 范围 / 非法 body / 存储错误。
- DB：`bash scripts/test-db.sh` 14 个 storage 测试全绿，含 `TestMigrationSchema`、`TestCareEventStoreSmoke`。
- 契约：`internal/httpapi/contract_test.go` 冻结端点包含四个护理事件端点；fixtures 双向对账与 Go mirror round-trip 通过；`npm run codegen` 无 diff。
- 扫描：`rg 'recorded_by|device_id|caregiver|active_child|activeChild' apps/mobile/src internal cmd --glob '!*.test.*'` 仅命中 `apps/mobile/src/care/careEvents.ts` 的契约注释；`reports` / `analysis` / `reports/trends` 无护理事件导入。未新增账号 / 邀请 / 云同步 / 远程推送。
- App：`npx tsc --noEmit`、`npm run lint -- --max-warnings=0`、`TZ=UTC npm test -- --runInBand`（60 suites / 483 tests）、`npx expo export --platform android` 全绿；`python3 scripts/check-docs.py` 167 个文件 OK。

**文档收尾：** `docs/design/2026-11-23-care-event-contract.md`（App adapter / provider 状态）、`docs/agents/contracts.md`（mirror 行）、`docs/adr/0019-...`（实现状态）、`.scratch/care-log/spec.md` §15（2026-11-25 更新）、`apps/mobile/ANDROID.md` §6.2 均已更新；护理事件 spec 标记为 implemented / known gaps。

**Android smoke（2026-11-25，隔离栈）：**

- 设备：A = Android 16 物理真机（Expo Go 58.0.5）；B = `sdk_gphone64_arm64` AVD（Expo Go 58.0.6）。降级事实：B 是模拟器，不是第二台物理真机；两端都通过 `adb reverse` 指向同一隔离栈。
- 跨设备：A 创建喂奶 / 尿布 / 睡眠 / 辅食（id 1–4）；B 今天页下拉刷新后看到同一批四条，且只在豆豆下；老二 / 老三显示空态（`member_id` 隔离）。
- 重复提示：B 先为老二记 13:55 奶粉 90 ml；A 在 5 分钟内再记 60 ml 时出现「刚刚已有一条喂养记录」；「返回修改」后老二仍只有 1 条；「仍然添加」后 2 条（id 5 / 6）。
- 修改 / 删除：A 把 id 6 从 60 ml 改为 80 ml，并删除 id 5；B 下拉刷新后只剩 13:57 · 奶粉 80 ml。
- 冷启动：两端 `am force-stop` 后重开；当前孩子、事件、提醒开关 / 间隔均保持；A 的提醒按 13:57 + 4h 重新出现在 AlarmManager。
- 提醒：A 开启后 AlarmManager 出现 `expo.modules.notifications.NOTIFICATION_EVENT` 于 17:52（豆豆 13:52 + 4h）；切到老二（B 刚记的 13:55）后只剩 17:55；切到无喂养的老三后无活动 Expo 闹钟；切回老二恢复 17:55；修改 / 删除后变为 17:57；关闭提醒后无活动 Expo 闹钟。设备此前已授权通知，本轮未重复弹窗（拒绝路径由 RNTL 覆盖）。
- 后端不可达：A 临时指向 `127.0.0.1:18082`，今天页显示「加载失败: … ConnectException」，没有出现内存事件或 fixture；改回隔离栈后数据恢复。
- 睡眠 UX（care-log 09）：弹层显示「结束时间 / 睡了多久（必填）」与「记录已经结束的睡眠…」提示；结束 00:20 + 30 分钟预览「开始 昨天 23:50」，保存后 API 为 `started_at=11-24T23:50+08`、`ended_at=11-25T00:20+08`、`duration_min=30`，时间线显示「昨天 · 跨夜 · 睡眠 30 分钟」，统计睡眠 KPI / 今天柱状正常。API 预置 legacy 进行中睡眠（08:00，ended_at/duration null）在时间线显示「进行中」；编辑时预填结束 14:07 / 时长 367 / 「开始 08:00」，保存后 `ended_at=14:07`、`duration_min=367`，时间线显示 6 小时 7 分钟，列表不再有进行中事件。
- 收尾：物理设备 Base URL 已恢复为 `http://localhost:8080/api/v1`（该设备原本也未 `adb reverse` 8080，健康检查同样不可达，与验收前一致）；停止隔离栈后 18000 / 18080 / 18081 无监听、无 `hdd-accept` 容器、无 adb reverse、一次性 DB / bucket 已删除；共享 deploy 栈未改。
- 未覆盖：通知在 17:57 实际弹出（只验证 AlarmManager 排程）；精确闹钟 / 省电策略下的到达时间不在承诺范围。
