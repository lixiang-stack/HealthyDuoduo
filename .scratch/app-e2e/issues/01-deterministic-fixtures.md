# 01: App E2E 非 OCR 用例改用确定性 fixture/seed

**What to build:** 把 App E2E 中「测 App 行为」的用例与真实 OCR 解耦：E2E 栈通过 fixture/replay 或专用 seed 命令注入固定报告，上传/存储/API/DB/UI 仍走真实链路；真实 OCR 只保留少量粗粒度 smoke。当前 seed 四份报告全部经真实识别，导致 `cbc_03`/`cbc_04` 的状态、flag、日期、段落文本都成了隐式测试前提，CI 与本地 OCR 稍有差异就会红。

**Blocked by:** None（可立即开始；需先定 fixture 注入方式）。

**Status:** resolved

## 背景

PR #31 的 `app-e2e` 曾在本地 13/13 通过、CI 失败：

- 用例 `可补录未知只给补录主操作,并显示原因` 固定查找 `name === '血小板' && flag === 'unknown'`；
- `cbc_04` 在 CI 的 OCR 结果与本地不完全一致，找不到该 item，`expected undefined to be defined`；
- 修复后改为按 `isBackfillItem` 同口径选任意可补录项，CI 已绿，但仍是「依赖真实 OCR 恰好产出某个 backfill 项」，没有根治。

本次失败证明：**同样的代码 + 同样的真实图片，本地与 CI 的 OCR 结果不保证一致**。只把固定指标名改成结构筛选，只是把概率降低，不是确定性。

## 当前受影响的用例

| 用例 | 依赖的真实 OCR 结果 | 风险 |
| --- | --- | --- |
| `档案按成员过滤报告` | `cbc_04` 必须 partial 且含 unknown/低置信；`cbc_03` 必须 success；`us_01` 日期必须为空 | 徽标数量/状态变化即失败 |
| `成员概览与待处理入口` | 报告状态、待核对项数量、日期排序 | 最新报告/空态/入口数量变化 |
| `全部档案与筛选` | `cbc_04` 必须 partial | 待核对筛选数量变化 |
| `上传样张后时间线出现新报告` | `glu_02` 识别成功且及时返回 | 上传超时/识别失败 |
| `影像单 us_03` | 日期 `2024-09-15`、检查项目名、无 findings | 日期/名称/段落差异 |
| `影像单 us_01` | 日期为 null、检查项目名、三段完整 | OCR 多出日期/段落缺失 |
| `核对低置信项并修正指标项` | `cbc_03` 有低置信项 + 另一个有 value 的项 | `pending`/`target` 为 undefined |
| `可补录未知只给补录主操作` | `cbc_04` 至少有一个 backfill unknown | 无补录项即失败 |
| `高置信未知项无需修改即可展开原图定位` | `cbc_04` 有高置信 unknown + 带 OCR 行的低置信项 | 找不到项/zoom 不渲染 |
| `改归属、删除与同图重传` | `cbc_04` 有 unknown；重传后 items 非空 | 无 unknown/重传 0 项 |

`apps/mobile/e2e/support.ts:150-167` 目前只校验了 `cbc_03` 有低置信项、两张影像单有 `impression`；其余前提都没断言。

## 验收清单

- [x] 选定 fixture 注入方式（推荐 E2E replay：按图片 SHA 返回预置 `ReportDetail`，只替换 recognizer 边界；备选：`hdd` 专用 seed 命令直插 DB/对象存储）。
- [x] 非 OCR 用例使用的报告改为固定 fixture：`cbc_03`（success、低置信项、至少两个可修正项）、`cbc_04`（partial、unknown、backfill、高置信 unknown、带 `ocr_lines` 的低置信项）、`us_01`/`us_03`（固定 dates/sections）。
- [x] fixture 只影响 E2E 栈（环境变量/专用命令），生产 API 与共享栈不暴露测试入口。
- [x] `seedData` 对所有测试前提做显式断言，失败在 `beforeAll` 给出具体原因，而不是各用例里的 `expected undefined`。
- [x] App 行为断言不再依赖 OCR 文本/日期/计数（日期/段落断言取 fixture 值）；徽标、原因、按钮优先用稳定 testID，不复制 UI 文案。
- [x] 去掉 E2E 内重复的 `isBackfillItem` 规则与 `BACKFILL_REASON_TEXT` 文案（当前在 `app.e2e.ts:23-50` 复制自 `src/review/items.ts`）；改为 fixture 显式字段或纯规则模块复用。
- [x] 真实 OCR 单独保留 smoke（1-2 张样张）：只断言上传成功、报告非空、类型/日期/段落至少合理，不驱动改归属/筛选/核对等 UI 行为。
- [x] 处理 serial + 共享 seed 的顺序耦合：至少让破坏性用例独立/最后执行，失败不连坐跳过；条件允许时每个用例独立 seed。
- [x] 文档 `docs/agents/app-e2e.md` 的 12/12 更新为当前实际条数，并写明 fixture/OCR smoke 的分工。
- [x] 本地 14/14 跑通；CI 由 PR 验证（不把「固定 `linux/amd64`」当作唯一修复）。

## 不做

- 不删除真实 OCR 回归；它继续由 Pipeline E2E/golden 覆盖。
- 不为 App E2E 引入 agent/模型调用。
- 不为了测试把 fixture 入口暴露到生产 API。

## Comments

**分析（2026-10-23，PR #31 CI 失败后）**

- 失败详情：`.github` App E2E job，用例 `可补录未知只给补录主操作,并显示原因`，`expected undefined to be defined`；本地同一 commit 13/13 通过。
- 根因不是 App 逻辑，而是 seed 把真实 OCR 的具体输出当 fixture：`apps/mobile/e2e/support.ts` 四份报告全部 `uploadSample()`；用例再按 OCR 名称/flag/日期/段落做断言。
- 当前临时修复只改了 `app.e2e.ts` 的选样条件，仍要求 `cbc_04` 产出至少一个 backfill 项；因此本票保留为后续根治项，不和 PR #31 的 bug fix 混在一起。

**实施（2026-10-23，分支 `app-e2e-fixtures`）**

- 注入方式：识别端回放。`apps/mobile/e2e/compose.yml` 仅在专用栈设置 `RECOGNIZER_REPLAY_SAMPLES`，`/report` 按上传图像 sha256 返回 `samples/expected/{ocr,report}` golden；未列出的样张仍走真实 OCR。
- 固定样本：`cbc_02`（生命周期）、`cbc_03`（核对）、`cbc_04`（自愈）、`us_01`/`us_03`（影像）、`glu_02`（上传）。`ocr-smoke.e2e.ts` 用 `cbc_01`/`us_02` 做真实 OCR 粗粒度 smoke。
- 用例拆分：`app.e2e.ts`（11 条 serial）、`lifecycle.e2e.ts`（1 条独立破坏性）、`error-state.e2e.ts`（1 条）、`ocr-smoke.e2e.ts`（1 条）；seed 前提在 `beforeAll` 显式断言。
- 去重：E2E 不再复制 `isBackfillItem` / 原因文案，统一复用 `src/review/rules.ts`；徽标/原因/入口改用稳定 testID。
- 验证：本地 App E2E 14/14；`npm run typecheck`、`npm test`、`uv run pytest`、`uv run ruff check .` 全绿。
