# 09: 睡眠改为结束后补记

**What to build:** 把睡眠记录从“开始时间 + 可选时长（空 = 进行中）”改为“结束后补记”：结束时间默认现在、时长必填、开始时间由两者推导。生产 UI 不再创建进行中睡眠；契约保留 `ended_at = null` 语义给 fixture / 未来实时计时。每次睡眠一条，夜醒后再次入睡可另记一条，不强制分段。

**Blocked by:** None。

**Status:** ready-for-agent

## 验收清单

- [ ] 睡眠弹层显示「结束时间」（默认现在）与「睡了多久」（必填，正整数分钟）；开始时间根据两者推导并显示预览（如「开始 13:00」）。
- [ ] 跨午夜推导正确：结束 00:20 + 30 分钟 → 开始昨天 23:50；结束 23:50 + 30 分钟 → 开始当天 23:20。
- [ ] 时长校验：必须 > 0 且 ≤ 24 小时；非整数 / 空值 / 非法结束时间有提示；不创建零时长睡眠。
- [ ] 生产保存的 sleep 事件 `ended_at` 非空、`duration_min > 0`、`started_at = ended_at - duration`；不再产生 `ended_at = null` 的新事件。
- [ ] 入口 / 标题 / 提示让用户理解为“记录已结束的睡眠”，不暗示开始计时；文案提示夜醒后可另记一条。
- [ ] 编辑旧的进行中睡眠（legacy / fixture）时，按已有 `started_at` 与当前时间预填结束 / 时长，保存后完成该事件。
- [ ] 时间线保留进行中渲染以兼容 fixture / legacy；生产路径不再进入该分支。
- [ ] RNTL 覆盖：结束 + 时长、跨午夜、必填 / 非法时长、保存后 `started_at` / `ended_at` / `duration_min`、文案、legacy 编辑完成。
- [ ] `npx tsc --noEmit`、`npm test -- --runInBand`、`npm run lint -- --max-warnings=0`、`npx expo export --platform android` 全绿。
- [ ] Android 真机 smoke：睡眠弹层字段、跨午夜预览、保存后时间线 / 统计正确；不新增进行中事件。

## 实现提示

- 当前 `RecordDraft` 的 `date` / `time` 是开始时间；建议为 sleep 增加 `endTime`（HH:MM），`date` 表示结束日期。其他三类继续用 `time` 作为事件时间。
- `recordDraftToBody` 的 sleep 分支：先由 `date + endTime` 得到 `ended_at`，再减 `durationMin` 得到 `started_at`；两者都写入，`duration_min` 写正数。
- `editRecordDraft`：已完成睡眠用 `ended_at` 预填结束时间、`duration_min` 预填时长；进行中睡眠用 `now` 预填结束、`now - started_at` 预填时长（允许用户改）。
- `recordDraftErrors`：sleep 时长必填，正整数且 ≤ 1440；结束时间沿用 `HH:MM` 校验；开始时间不需要单独输入，但预览要能区分昨天 / 今天。
- `RecordFields` 只对 sleep 换标签；feed / diaper / solid 保持现有字段与默认值，避免机械改动。
- 统计与时间线不需要改口径：它们已按 `started_at` / `ended_at` 处理；本票只是不再产生 open-ended 事件。
- 跨天补录（记录结束于更早日期）仍不在本票；需要时另开票，不要用“未来时间”兜底。
- 文案示例：快捷动作「睡眠」；弹层标题「记录睡眠」；提示「记录已经结束的睡眠；夜醒后再次入睡可另记一条」。不要写喂养建议。
