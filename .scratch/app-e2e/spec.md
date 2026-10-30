# App E2E 接入 spec

**Status:** resolved（历史；Web E2E 回路已由 ADR-0013 移除，2026-10-30）

## 为什么做

- App 层没有屏幕级测试：Jest 只覆盖纯函数；导航、API 调用、上传/核对/自愈 UI 全靠手工。
- PR #22（报告自愈）自述「浏览器交互验收未跑（本机无浏览器操作通道）」——新功能的 UI 验收缺口已经发生。
- 共享 `deploy` 栈是家庭真实数据，App 用例会写 members/reports/items，现有 seed/clean 台账覆盖不了，测试间也无隔离。

## 做什么（拷问会定稿，2026-10-18）

- 价值档位取 **A（确定性 App 回归）**：无 agent/模型调用；B（定时 agent 探索）稳定后再议，C（原生双端）等工具链。
- 目标载体 **Expo Web 先行**（本机无 Xcode/模拟器；同一套 React 流程可覆盖）。
- 被测后端用 **专用 compose 栈**（`hdd-e2e`，独立端口/卷，每轮重建，CORS 仅此栈打开）；共享栈不碰。
- 首版覆盖：设置/健康检查、成员创建切换、时间线过滤、上传→时间线、核对确认/修正、改归属/删除/同图重传、后端不可达错误态。
- 关键控件加**最小 testID**（RNW `data-testid`）；测试代码归 `apps/mobile/e2e/`；术语区分 **App E2E** 与 **Pipeline E2E**。
- 门禁：先本地；连续 10 次全绿后升级为路径过滤的 PR 必过 job。

## 怎么实施

- `apps/mobile/e2e.config.ts`：web target（390×844 手机视口）+ 由 runner 启动 Expo（端口 0），无 agents。
- `apps/mobile/e2e/run.sh`：专用栈起停（先 `down -v`）、健康等待、参数透传；`E2E_KEEP_STACK=1` 保留现场。
- `apps/mobile/e2e/support.ts`：契约类型复用 App 生成物；API/样张/testID/storage 键集中；`stubFileChooser` 处理 headless 文件选择器。
- `apps/mobile/e2e/tests/app.e2e.ts`：一条 serial 核心链路 + 一条错误态；seed 用真实样张（`cbc_03` 核对、`cbc_04` 自愈）。
- 文档：`docs/agents/app-e2e.md`（操作）；`docs/adr/0007-app-e2e-isolated-stack.md`（为什么独立栈）。

## 怎么验收

- `npm run e2e` 在专用栈上 7/7 通过，单次 <10 分钟（实测 ~1 分钟，含栈启动），零共享栈写入。
- `npm run typecheck` 同时覆盖 App 与 e2e 测试代码；Jest 既有 70 例保持全绿。
- 连续 10 次全绿（本地 soak）；随后加 PR 必过 job（路径过滤 `apps/mobile/**`、`cmd/hdd/**`、`internal/**`、`schemas/**`）。
- 失败时 `.e2e/report.json` / `.e2e/results/<test>/trace.md` / `.e2e/logs/expo.log` 能定位到步骤。
