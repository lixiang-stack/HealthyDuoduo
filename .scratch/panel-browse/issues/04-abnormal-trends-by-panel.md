# 04: 异常指标按项目分组

**Spec:** ../spec.md（§8.5）

**What to build:** 「异常指标」页从按 `report_type` 分组改成按项目组分组：只列每个项目最近一次结果里的 high / low / abnormal 指标；点行打开趋势层；无异常 / 无报告 / 错误态与现有页面同口径。

**Blocked by:** 02（报告项目组纵切）、03（按项目浏览）

**Status:** ready-for-agent

- [ ] 页面按项目分组，组头显示项目 label、最新报告日期、异常项数；不再出现 `report_type` 分组。
- [ ] 只列 latest flag 为 high / low / abnormal 的指标；normal / unknown / not_tested 不列。
- [ ] 行内显示最新值、单位、参考范围、flag、低置信标签与迷你趋势；点行打开趋势层，使用已加载的点，不额外请求。
- [ ] 无异常时显示「这位成员暂时没有异常指标。」+ 查看全部档案；无报告 / 错误 / 未配置 / 未选成员态与现有同口径。
- [ ] 现有趋势单位门控、信任标记、趋势详情层行为不回归。
- [ ] 趋势页 E2E 更新；`npm test`、`npm run typecheck`、`npx expo lint` 全绿。
