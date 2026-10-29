# 07: E2E 全链路 + 文档 + 回归

**Spec:** ../spec.md（§9.3、§10、§13）

**What to build:** 把项目组 / 按项目 / 家庭首页的跨单链路收口：补齐 e2e replay 样本、App E2E 全量、`app-e2e` 文档与手工验收清单；确保核对、趋势单位门控、上传、多图上传等既有回路全绿。

**Blocked by:** 02（报告项目组纵切）、03（按项目浏览）、04（异常指标按项目分组）、05（存量回填项目组）、06（家庭首页 + 双 Tab）

**Status:** ready-for-human

- [x] e2e replay 样本覆盖 lft_02 / lft_06 / lft_08 / lft_09 / tft_01 / ua_05 / us_01 / coag_01 / fibrosis_01 / bloodtype_01 / unknown_01 / 失败报告。
- [x] App E2E 覆盖家庭首页、项目列表 / 详情、`?panel=` 筛选、报告详情分组、异常指标、影像 / 失败 / 其他、空态。
- [x] 既有核对 / 修正 / 改派 / 删除 / 上传 / 多图上传 / 趋势 E2E 全绿。
- [x] `docs/agents/app-e2e.md` 更新项目组 / 家庭首页 / replay 说明。
- [x] 全命令：`go test ./...`、`cd recognizer && pytest`、`npm test`、`npm run typecheck`、`npx expo lint`、`npm run e2e` 全绿。
- [ ] 手工验收清单：3–5 位家长 4 次点按内找到肾功 / 肝功 / 甲状腺；LAN 首屏 <2s；部署 runbook（识别 → 迁移 → 回填 → App）。

## Comments

**实现（2026-10-28，分支 `e2e-regression-impl`，叠在票 06 `family-tabs-impl` 上）**

- Replay 覆盖：`e2e/compose.yml` 新增 `lft_06` / `lft_08` / `lft_09` / `coag_01` / `fibrosis_01` / `bloodtype_01` / `unknown_01`；另补 `glu_04` / `infectious_01` 覆盖血糖 / 感染筛查项目行。连同既有 `lft_02` / `tft_01` / `ua_05` / `us_01` 与白图 failed 覆盖票内清单。
- 无原图样本：掩码 OCR 语料没有原图，不能按真实图片回放。`Replay` 构造函数增加显式 `image_dirs`，`from_env` 读取 `RECOGNIZER_REPLAY_IMAGES`；`apps/mobile/e2e/replay/` 放 9 张 64×64 替身图（字节唯一，避免 sha 去重），compose 挂到 `/e2e-replay`。替身图只提供 sha，仍返回 `samples/expected` 同一份 golden，不参与 OCR 质量判断、不进 `samples/`；单测直接校验提交的替身图 + `from_env` 目录。
- App E2E：`seedPanelData` 新增混合生化 / 其他 / 凝血 / 感染成员；`panels.e2e.ts` 新增矩阵用例，覆盖 lft_08 报告详情四项目分组（组顺序与「N 项 · M 项异常」由 golden 派生）、血糖 / 电解质项目行、`?panel=lipid` 过滤、`other` 多来源字面量断言、凝血 / 感染项目；`report-panel-group-*` / `panel-source-label` testID 进 `support.ts`。
- 文档：`docs/agents/app-e2e.md` 更新 23 条用例、replay 清单与替身图边界。
- 验证：`go test ./...` 全绿；`cd recognizer && uv run --frozen pytest` 332 passed / 28 skipped；`npm test` 22 suites / 197 tests；`npm run typecheck`；`npx expo lint`（0 error，仅既有 `review/items.test.ts:212` warning）；`npm run e2e` 7 files / 23 tests 全绿；`python3 scripts/check-docs.py` 88 files OK。

**待人工（本票置 ready-for-human）：**

- 家长可用性测试与 LAN 首屏实测按手工清单执行、填写签收表。
- 部署 runbook 需在真实局域网栈演练一次（备份 → 识别 → 迁移 → `hdd backfill-panels` → App → 回滚）。
- Pipeline E2E 共享栈未动：本票仅跑 App E2E 专用栈；动共享库前按 `docs/agents/e2e.md` snapshot。
