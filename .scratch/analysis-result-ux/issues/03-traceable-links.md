# 03: 可追溯链接层（P1）

**What to build:** 来源链接可以打开/复制，报告日期、指标日期、病历可以进入报告详情，形成“数值 → 报告 → 标准”的核对链。

**Blocked by:** 01（结果页布局）；复制方案需确认是否新增 `expo-clipboard`

**Status:** needs-triage

- [ ] 统一来源行：标准名 + 说明 + URL + 打开/复制
- [ ] 使用已安装的 `expo-linking` 打开 http(s) 链接
- [ ] 复制方案二选一：`expo-clipboard` 一键复制，或 `selectable` 兜底；确认后补依赖/重建 dev build
- [ ] URL 使用 `accessibilityRole="link"`，复制按钮 `accessibilityRole="button"`，触控区 ≥ 44×44
- [ ] 打开失败给可见反馈，不静默失败
- [ ] 报告日期、偏离卡日期、病历卡链接到 `/report/[report_id]`
- [ ] 返回分析页后保持原页面状态
- [ ] 框架层来源也使用同一来源行，而不是只显示标准名

**依据：** `.scratch/analysis-result-ux/spec.md` §2.5；`research.md` 发现 5、B2。
