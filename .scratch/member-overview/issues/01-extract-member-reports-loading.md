# 01: 抽出成员档案的共享加载（prefactor）

**What to build:** 档案页和即将出现的成员概览页共用同一条数据加载路径：解析后端地址 → 取成员列表 → 取当前成员的报告摘要 → 聚焦时重载 → 失败可重试。用户看到的界面、文案、路由都不变；这张票只把现有加载逻辑从档案页抽出来，让后续两张票只关注新行为，不在两个页面里各写一份加载。

**Blocked by:** None（可立即开始）。

**Status:** resolved

## 验收清单

- [x] 档案页改用共享加载后，成员切换、回到页面时重载、未配置后端、加载失败 + 重试与改前一致。
- [x] 加载逻辑只有一个实现；概览页后续直接复用，不在页面里复制。
- [x] 不新增用户可见文案、路由、依赖或数据请求。
- [x] 现有单测与 `npm run typecheck` 全绿；App E2E 9/9。
- [ ] `npx expo lint` 全绿（阻塞于仓库基线：无 committed ESLint 配置；见 Comments）。

## 实现提示

- 状态沿用档案页现有的 `loading | unconfigured | error | ready`；`ready` 暴露成员列表、当前成员报告摘要与 `hasCurrentMember`（页面据此判定是否选中，避免与取报告的判定重复）。
- `reload` 仍供页面「重试」使用；聚焦重载留在共享加载里。
- 改动面只有加载逻辑与档案页接线，不动卡片、排序、文案。

## Comments

**实现（2026-10-21，票 01）**

- 新增共享加载 `reports/useMemberReports.ts`，档案页接线；`ready` 额外返回 `hasCurrentMember`，页面不再重复判定当前成员是否在列表中（code review 的 seam 建议）。
- 验证：`npm run typecheck` 通过；`npm test -- --runInBand` 13 suites / 80 tests 全绿；`npm run e2e` 9/9 通过。
- Lint 基线：`apps/mobile` 没有 committed ESLint 配置。`npx expo lint` 会自动安装 ESLint 并生成配置，然后报 3 个未改动文件的 pre-existing errors（`upload.tsx`、`LineZoom.tsx`、`review/components.tsx` 的 `react-hooks/set-state-in-effect`）。本次改动文件无 lint 报错；自动生成的配置与依赖已还原，未引入新依赖。若 lint 要成为门禁，需另立票清理基线并提交配置。
