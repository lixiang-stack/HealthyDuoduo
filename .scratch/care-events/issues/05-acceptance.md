# 05: 跨票验收与契约回归

**What to build:** 对 care-events 01–04 与 care-log 09 做完整验收：DB / storage / HTTP 契约 / App adapter / 提醒重排；跑自动化、双设备真机 smoke 与契约扫描，确认没有越过 ADR-0006 / ADR-0019 的边界。

**Blocked by:** 01、02、03、04、care-log 的睡眠补记票（跨 feature）。

跨 feature 依赖：[09-sleep-completion](../../care-log/issues/09-sleep-completion.md)。

**Status:** ready-for-agent

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
