# App E2E

家长侧 App 的 UI 层端到端回归：Expo Web（同一套 React 组件与数据流）跑在**专用 compose 栈**上，
用确定性 locator/expect 校验核心链路。与 `docs/agents/e2e.md` 的 Pipeline E2E（recognizer/CLI 数据链路、
共享栈 + 快照）是两条互不替代的回路；决策见 `docs/adr/0007-app-e2e-isolated-stack.md`。

## 跑一条回路

前置：Docker 可用；Node 22.22.3+；在 `apps/mobile` 下 `npm install` 一次。首次运行会构建
hdd/recognizer 镜像并下载 Chromium（各一次，之后走缓存）。

```bash
cd apps/mobile
npm run e2e                    # 起专用栈 → seed → 跑用例 → 拆除栈
npm run e2e -- --headed        # 参数透传给 e2e run(如 --headed/--grep)
E2E_KEEP_STACK=1 npm run e2e   # 跑完保留栈,便于手工看 Web/API(下次运行前仍会重建)
```

完成判据：11/11 通过；失败时看 `.e2e/results/<test>/trace.md` 与应用日志 `.e2e/logs/expo.log`。

## 专用栈

| 宿主端口 | 服务 | 说明 |
| --- | --- | --- |
| 58080 | hdd | API `http://127.0.0.1:58080/api/v1`；CORS 仅此栈打开 |
| 58000 | recognizer | `/healthz` |
| 55432 | postgres | 临时卷 `hdd-e2e_pgdata` |
| 58333 | seaweedfs S3 | 临时卷 `hdd-e2e_seaweeddata` |

每轮 `run.sh` 先 `down -v` 再 `up`，seed 经真实上传端点入档 `samples/cbc_03.png`（核对）、
`samples/cbc_04.png`（自愈）与影像单 `samples/us_01.webp` / `samples/us_03.jpeg`（报告段落
三段 / 无检查所见锚）；`duplicate:true` 即视为栈未重置并失败。共享 `deploy` 栈（8080 等）不受影响。

## 用例与选择器

- 用例在 `apps/mobile/e2e/tests/app.e2e.ts`：一条 serial 核心链路 + 一条错误态；无 agent/模型调用。
- 契约类型复用 `src/api/schema.d.ts`；API/样张/testID 约定集中在 `e2e/support.ts`。
- 关键控件带 `testID`（RNW 渲染为 `data-testid`）：成员 chip/行、上传选择/提交/结果、报告卡、
  报告检查项目/段落卡/段落缩放、核对项/确认/修正、详情管理卡（改归属/删除/确认）。改控件时
  同步改 `support.ts` 的 `testID`。
- Web 相册通道在 headless 下会打开原生 chooser 并被取消（expo-image-picker 随即移除 input）；
  `stubFileChooser` 屏蔽该次 click，测试再用 `setInputFiles` 注入文件。

## 边界与升级

- 只测 Web：相机、权限、原生文件/键盘/网络是盲区；原生目标后续用同一框架的 mobile 引擎接入。
- 首版本地运行；连续 10 次全绿后升级为路径过滤（`apps/mobile/**`、`cmd/hdd/**`、`internal/**`、
  `schemas/**`）的 PR 必过 job。CI job 形态参考 `npx e2e guide running` 与官方 GitHub Actions 文档。
- 框架 pre-1.0：升级 `e2e`/`@e2e-dev/web` 作为独立改动，跑一条回路验证后再提交。
- 遥测：`run.sh` 已设 `E2E_TELEMETRY_DISABLED=1`。
