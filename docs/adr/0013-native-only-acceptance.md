# 0013 · App 验收改为 RNTL + Android 真机 smoke，移除 Web E2E

**状态：** 采纳（2026-10-30）。

## 背景

- Web E2E 回路（ADR-0007）是在没有原生自动化、没有真机流程时引入的：它用 Expo Web + Playwright 覆盖 24 条 UI 流程，并自起 `hdd-e2e` 栈 + 识别端 golden 回放。
- 该回路维护一整条并行工具链：`@e2e-dev/web`、Playwright、Chromium、专用 compose、回放 fixture、CORS 验证开关与一份独立 E2E 手册。产品实际验收目标是 Android 真机，Web 不是对外入口。
- 原生 E2E 的 CI 成本（模拟器启动 + 运行）与 flaky 风险高于当前收益；本地 Maestro 需要持续维护 flow，暂不作为默认门禁。

## 决策

1. **移除 App 的 Web 验收回路**：删除 `apps/mobile/e2e/**`、`e2e.config.ts`、App E2E CI workflow、Playwright/E2E 依赖、replay adapter、`hdd-e2e` CORS 开关与相关文档。
2. **自动化回归放在 RNTL 屏幕测试**：`apps/mobile/src/test/screens/` 渲染真实屏幕，只在 `src/api/resolveClient` 处替换 API client；覆盖家庭、档案/项目、报告详情、核对、趋势。
3. **真机验收保留为人工 smoke**：`apps/mobile/ANDROID.md` 的清单覆盖相机/相册、权限、键盘、Android 返回、safe area、滚动性能与视觉文案；合并 UI 改动前执行。
4. **Pipeline E2E 不变**：`docs/agents/e2e.md` 继续覆盖 recognizer/CLI 数据链路、golden 与共享栈。
5. **保留 testID 与 `samples/expected`**：前者仍是 Android `uiautomator` / 未来原生自动化的稳定选择器；后者是识别侧 golden，不属于 Web 回路。

## 后果

- UI 集成层不再有每个 PR 的自动化门禁；这是明确接受的代价。若同类回归反复漏过人工 smoke，再单独评估原生 E2E（Maestro/Detox）并以新的 ADR 记录。
- RNTL 不覆盖真实导航动画、相机/相册、权限、键盘、safe area 与真实布局；这些仍由真机清单负责。
- Web 渲染器、CORS 与 replay 不再是生产/验收路径；后续如需浏览器诊断，应按当时需求重新引入，而不是恢复整条 E2E 回路。
- 历史决策保留在 ADR-0007；本 ADR 取代其“Web 先行”部分。
