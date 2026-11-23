# 08: 跨票验收与契约回归

**What to build:** 对 01–07 做一次完整验收：B3 导航、当前孩子、记录弹层、重复提示、提醒、统计图表；跑 RNTL / typecheck / Android 真机 smoke，检查没有引入账号 / 记录人归属 / 全局 active child 等越界字段。

**Blocked by:** 01, 02, 03, 04, 05, 06, 07。

**Status:** ready-for-agent

## 验收清单

- [ ] RNTL 全绿：导航壳、当前孩子、记录目标、重复提示、提醒、统计图表。
- [ ] `npm run typecheck`、`npm test` 全绿；按 `apps/mobile/ANDROID.md` 完成真机 smoke。
- [ ] 双设备模拟：设备 A 选豆豆、设备 B 选老二，互不覆盖；各自记录带正确 `member_id`。
- [ ] 重复提示：同孩子 5 分钟内提示；仍然添加 / 返回修改；跨孩子不误报。
- [ ] 提醒：默认关闭、权限、调度、取消、统计页无卡片；不承诺准时。
- [ ] 统计：4 指标 × 3 图表、空态、单点、孩子切换；无报告数据混入。
- [ ] 契约检查：不出现 `recorded_by` / `device_id` / 全局 active child；护理事件不写入 `reports` / `report_items` / 趋势 / 分析快照。
- [ ] 若 01 引入数据库迁移：跑 `bash scripts/test-db.sh`；若仅客户端契约，记录不适用。
- [ ] `python3 scripts/check-docs.py` 全绿；`CONTEXT.md` 已补「育儿 / 健康档案 / 当前孩子」。
- [ ] `docs/adr/0019-care-log-navigation-child-context.md` 与 `../spec.md` 的实现状态 / 差异已更新。
- [ ] 无账号 / 邀请 / 云同步 / 远程推送新增；ADR-0006 边界未被突破。

## 实现提示

- 验收以 `../spec.md` §10 为准；本票不新增功能，只做集成、回归与文档收尾。
- Android 真机清单重点：appbar 域 tab、孩子选择器、记录弹层目标、重复提示、底部 tab 状态保留、返回键、统计图表、提醒权限。
- 若 01 的契约仍停留在 adapter / fixture，明确记录「护理事件正式 API 未落地」，不要把 fixture 当生产实现。
- 发现票间口径冲突时，先回写 spec / ADR，再改代码；不要在单票里临时改契约。
