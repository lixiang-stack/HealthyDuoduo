# 02: 报告项目组纵切（数据 → API → 时间线卡片 / 报告详情）

**Spec:** ../spec.md（§7、§8.3、§8.6）

**What to build:** 新上传的报告把项目组写入存储；报告摘要与详情 API 返回 `panels` / `panel`；App 时间线卡片显示前 2 个项目组 + `+N`，报告详情按项目组分组并展示全部指标。失败报告与未回填旧报告回退 `report_type`，不崩、不空。

**Blocked by:** 01（识别端输出项目组与医学依据）

**Status:** ready-for-agent

- [ ] 迁移后旧行有安全默认；新报告的 `panels` / `panel` 正确写入，摘要与详情读回一致。
- [ ] 时间线卡片显示「肝功 · 肾功 +1」这类主 + 次项目组；只有 1 个项目时不加 `+N`；失败报告回退 `report_type`。
- [ ] 报告详情按 `panels` 顺序分组，组内展示报告全部指标；混合报告（肝功 + 肾功 + 尿酸）三组都出现；核对入口仍可用。
- [ ] 未回填旧报告 `panels=[]` 时卡片 / 详情回退 `report_type`，不出现空标题。
- [ ] 契约测试覆盖 summary / detail / failed / imaging / upload 的 `panels` / `panel`；App 类型与测试工厂同步。
- [ ] `go test ./...`、`npm test`、`npm run typecheck`、`npx expo lint` 全绿；现有上传 / 核对 / 趋势 E2E 不回归。
