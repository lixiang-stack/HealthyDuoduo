# 0007 · App E2E 独立栈先行：Expo Web + 确定性用例，不碰共享 deploy 栈

**状态：** 采纳（2026-10-18，需求拷问会定稿）。

家长侧 App 的 UI 回归由新回路 **App E2E** 承担：**专用 compose 项目 `hdd-e2e` + 独立端口/卷、每轮重建**，目标先落在 **Expo Web**（`@e2e-dev/web` 的 Playwright 引擎），用例全部是**确定性 locator/expect**（无 agent/模型调用）。首版本地运行，连续全绿后升级为路径过滤的 PR 必过 job；与 Pipeline E2E（`docs/agents/e2e.md`，共享栈 + 快照）并存、互不替代。

## 背景

- 识别质量与数据不变量已有 Pipeline E2E 覆盖；App 层没有屏幕级测试、没有 testID。PR #22（报告自愈）在验收里自述「浏览器交互验收未跑（本机无浏览器操作通道）」。
- App E2E 会**写** members/reports/items（建成员、上传、核对、改派、删除）；共享 `deploy` 栈是家庭真实数据，现有 seed/clean 台账只覆盖 samples 种子行，覆盖不了 App 写入，测试之间也没有隔离。
- 本机无 Xcode/模拟器（`simctl` 不可用），原生 iOS/Android 目标当前跑不起来；Expo Web 实测可打包，且渲染同一套 React 组件与数据流。
- tester-army/e2e 的 `agent.act/assert` 是模型判断（成本 + 波动），不适合做质量门禁；确定性 locator 才是门禁主体。

## Considered Options

- **共享栈 + snapshot/restore**（Pipeline E2E 的既有做法）：App 写入不在台账、测试间无隔离，恢复动作需人工 yes；把家庭真实数据卷进自动化回路。否。
- **假 API / route mock**：快、稳定，但抓不到 App↔hdd 契约漂移与真实 OCR 链路；与「App E2E」的定位冲突。否。
- **原生模拟器先行**：更接近真实使用，但需先装 Xcode + `expo-dev-client` + dev build，首版收益被环境搭建吃掉，而 Web 已能覆盖同一套 React 流程。延期——原生作为后续目标（同一框架的 mobile 引擎）。
- **专用栈 + 每轮重建 + Web 先行 + 确定性用例**：隔离、可重复、CI 友好；代价是每轮 compose 启动与真实 OCR（数秒到数十秒）。采纳。

## Consequences

- `npm run e2e`（apps/mobile）自带栈的起停与清库，失败证据在 `apps/mobile/.e2e/`；默认不污染共享栈。
- 首版覆盖：设置/健康检查、成员创建切换、时间线过滤、上传→时间线、核对确认/修正、改归属/删除/同图重传、后端不可达错误态。
- 已知盲区：相机、权限、原生文件/键盘/网络；需要后续原生目标，不假装已覆盖。
- 首版仅本地；「连续 10 次全绿」后升级为路径过滤的 PR 必过 job（操作见 `docs/agents/app-e2e.md`）。
- 框架仍在 1.0 前，minor 版本可能改变配置/API；升级按独立改动处理。
- Web 文件选择器在 headless 下会被自动取消（expo-image-picker 随即移除 input），测试用 init script 屏蔽原生 chooser 再 `setInputFiles`；这是测试挂具，不改 App 行为。
