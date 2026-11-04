# 05: 状态与操作补齐（P1）

**What to build:** 结果页能说明分析有多旧、哪些状态影响了什么，并提供重新分析、重试解读、切换时间范围等下一步动作。

**Blocked by:** 01；`created_at` / `entry_updated` / 部分拒绝如果需要改契约，先确认

**Status:** needs-triage

- [ ] 显示分析生成时间；旧分析可见
- [ ] `stale=true` 提供“基于最新数据重新分析”，沿用原 panel/window，创建新快照
- [ ] 结果页可切换 6/12/24 个月并创建新分析，不覆盖旧快照
- [ ] 云端失败显示“重试解读”；不要每次重试都新建分析（若后端无端点，先记录契约缺口）
- [ ] 部分 section 被拒绝时显示“有 N 段解读未通过校验，已省略”
- [ ] 截断状态保留提示，并说明只影响解释层
- [ ] “这不对”改为明确范围：结果级 / 条目级；条目级反馈携带 `entry_id`
- [ ] 8 个状态各有文案、影响范围、下一步
- [ ] 如需 `created_at` / `entry_updated` / `partial_rejected`，先改 OpenAPI，再同步 Go/App/codegen/fixtures

**依据：** `.scratch/analysis-result-ux/spec.md` §2.6；`research.md` B3、B5。
