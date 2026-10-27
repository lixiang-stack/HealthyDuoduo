# 07: E2E 全链路 + 文档 + 回归

**Spec:** ../spec.md（§9.3、§10、§13）

**What to build:** 把项目组 / 按项目 / 家庭首页的跨单链路收口：补齐 e2e replay 样本、App E2E 全量、`app-e2e` 文档与手工验收清单；确保核对、趋势单位门控、上传、多图上传等既有回路全绿。

**Blocked by:** 02（报告项目组纵切）、03（按项目浏览）、04（异常指标按项目分组）、05（存量回填项目组）、06（家庭首页 + 双 Tab）

**Status:** ready-for-agent

- [ ] e2e replay 样本覆盖 lft_02 / lft_06 / lft_08 / lft_09 / tft_01 / ua_05 / us_01 / coag_01 / fibrosis_01 / bloodtype_01 / unknown_01 / 失败报告。
- [ ] App E2E 覆盖家庭首页、项目列表 / 详情、`?panel=` 筛选、报告详情分组、异常指标、影像 / 失败 / 其他、空态。
- [ ] 既有核对 / 修正 / 改派 / 删除 / 上传 / 多图上传 / 趋势 E2E 全绿。
- [ ] `docs/agents/app-e2e.md` 更新项目组 / 家庭首页 / replay 说明。
- [ ] 全命令：`go test ./...`、`cd recognizer && pytest`、`npm test`、`npm run typecheck`、`npx expo lint`、`npm run e2e` 全绿。
- [ ] 手工验收清单：3–5 位家长 4 次点按内找到肾功 / 肝功 / 甲状腺；LAN 首屏 <2s；部署 runbook（识别 → 迁移 → 回填 → App）。
