# App E2E

家长侧 App 的 UI 层端到端回归：Expo Web（同一套 React 组件与数据流）跑在**专用 compose 栈**上，
用确定性 locator/expect 校验核心链路。报告内容由识别端的 **golden 回放**提供，不依赖真实 OCR 的
当次输出；真实 OCR 只保留一张粗粒度 smoke。与 `docs/agents/e2e.md` 的 Pipeline E2E
（recognizer/CLI 数据链路、共享栈 + 快照）是两条互不替代的回路；决策见
`docs/adr/0007-app-e2e-isolated-stack.md`。

## 跑一条回路

前置：Docker 可用；Node 22.22.3+；在 `apps/mobile` 下 `npm install` 一次。首次运行会构建
hdd/recognizer 镜像并下载 Chromium（各一次，之后走缓存）。

```bash
cd apps/mobile
npm run e2e                    # 起专用栈 → seed → 跑用例 → 拆除栈
npm run e2e -- --headed        # 参数透传给 e2e run(如 --headed/--grep)
E2E_KEEP_STACK=1 npm run e2e   # 跑完保留栈,便于手工看 Web/API(下次运行前仍会重建)
```

完成判据：21/21 通过；失败时看 `.e2e/results/<test>/trace.md` 与应用日志 `.e2e/logs/expo.log`。

## 专用栈

| 宿主端口 | 服务 | 说明 |
| --- | --- | --- |
| 58080 | hdd | API `http://127.0.0.1:58080/api/v1`；CORS 仅此栈打开 |
| 58000 | recognizer | `/healthz` |
| 55432 | postgres | 临时卷 `hdd-e2e_pgdata` |
| 58333 | seaweedfs S3 | 临时卷 `hdd-e2e_seaweeddata` |

每轮 `run.sh` 先 `down -v` 再 `up`，并叠加载 `e2e/compose.yml`：识别服务通过
`RECOGNIZER_REPLAY_SAMPLES` 对这些样本**按图像 sha256 回放** `samples/expected/{ocr,report}`
下的 golden，不再跑真实 OCR：`cbc_02`（生命周期）、`cbc_03`（核对）、`cbc_04`（自愈）、
`us_01` / `us_03`（影像段落）、`glu_02`（单图上传）、`glu_01` / `glu_03`（多图上传）、
`lft_02` / `lft_03` / `lft_04` / `lft_05`（趋势与按项目）、`tft_01` / `ua_05`（按项目）。
seed 仍经真实上传端点入档；`duplicate:true` 即视为栈未重置并失败。共享 `deploy` 栈（8080 等）
不加载该覆盖、不暴露回放入口。

不在回放名单里的样本仍走真实 OCR，供 `ocr-smoke.e2e.ts` 使用（`cbc_01` / `us_02`）。
该 smoke 只断言上传成功、报告非空、类型/日期/段落合理，不驱动改归属/筛选/核对等 UI 行为；
逐项识别质量继续由 Pipeline E2E/golden 覆盖。失败的按项目用例用一张 98 byte 无文本白图
（真实 OCR 返回 `failed`、`panels=[other]`、无指标项），不新增 golden。

## 用例与选择器

- `app.e2e.ts`（13 条，serial）：设置/健康检查、成员、时间线过滤、概览、档案筛选、上传（单图/多图）、
  两张影像单、核对确认/修正、可补录未知、高置信未知原图定位、影像按项目行直接进报告；无 agent/模型调用。
- `panels.e2e.ts`（2 条）：项目列表 / 详情 / 趋势层 / `?panel=` 筛选；尿常规不串肝功 / 血糖；
  failed 报告不进项目视图与筛选。
- `lifecycle.e2e.ts`（1 条）：改归属 / 删除 / 同图重传。独立成员 + `cbc_02` fixture，与主链路
  不共享数据；破坏性操作失败不连坐跳过其他文件。
- `error-state.e2e.ts`（1 条）：后端不可达的首页失败态；不依赖 seed/栈数据。
- `ocr-smoke.e2e.ts`（1 条）：真实 OCR smoke（2 张样张），只做粗粒度 API 断言。
- `upload-ownership.e2e.ts`（1 条）：持久化的当前成员已不存在时要求重新选择，不允许提交孤儿上传。
- `trends.e2e.ts`（2 条）：异常指标趋势列表 / 详情 / 单点 / 无异常 / 无报告；混合报告按项目浏览
  与 `?panel=` 筛选。独立成员 + `lft_02` / `lft_03` / `lft_04` / `lft_05` fixture，不依赖主链路 seed。
- 契约类型复用 `src/api/schema.d.ts`；API/样张/testID 约定集中在 `e2e/support.ts`。
- 关键控件带 `testID`（RNW 渲染为 `data-testid`）：成员 chip/行、上传选择/提交/逐行状态/移除/重试、报告卡、
  报告徽标（偏高/偏低/异常/未知/待核对）、检查项目/段落卡/段落缩放、核对项/状态/原因/确认/修正、
  趋势分组/指标行/迷你图/详情层/逐点行、详情管理卡（改归属/删除/确认）、档案分段/`?panel=` 横幅、
  项目行/项目详情指标行/迷你趋势/「包含该项目的报告」/最新失败报告提示。改控件时同步改
  `support.ts` 的 `testID`。
- 用例断言优先用 testID 与 fixture 派生值；`isBackfillItem` / `flagReasonText` /
  `reviewStatusLabel` / `panelItemIsAbnormal` 复用 `src/` 规则，不复制规则或原因文案。
- Web 相册通道在 headless 下会打开原生 chooser 并被取消（expo-image-picker 随即移除 input）；
  `stubFileChooser` 屏蔽该次 click，测试再用 `setInputFiles` 注入文件。

## 边界与升级

- 只测 Web：相机、权限、原生文件/键盘/网络是盲区；原生目标后续用同一框架的 mobile 引擎接入。
- golden 回放只固定识别产物；上传/存储/API/DB/UI 仍走真实链路。`samples/expected` 重生成后若
  用例前提变化，`seedData` / `seedTrendData` / `seedLifecycleData` 的显式断言会在 `beforeAll`
  点名 fixture 与缺口。
- 已接入路径过滤的 PR 必过 job（`.github/workflows/app-e2e.yml`，覆盖 `apps/mobile/**`、
  `cmd/hdd/**`、`internal/**`、`schemas/**`、`recognizer/**`、`deploy/**`、`samples/**`）。
- 框架 pre-1.0：升级 `e2e`/`@e2e-dev/web` 作为独立改动，跑一条回路验证后再提交。
- 遥测：`run.sh` 已设 `E2E_TELEMETRY_DISABLED=1`。
