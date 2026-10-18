// App E2E:Expo Web 上的确定性 UI 回归(无 agent/模型调用)。
// 专用 compose 栈(hdd-e2e)与 seed 由 e2e/run.sh 负责;本配置只驱动 Expo Web。
// 为什么 Web 先行/专用栈,见 docs/adr/0007-app-e2e-isolated-stack.md。
import type { E2EConfig } from 'e2e';
import { web } from '@e2e-dev/web';

export default {
  projectId: 'healthyduoduo-app',
  targets: [
    {
      name: 'web',
      // 手机视口:MVP 的主战场是窄屏,报告详情/核对卡在窄屏下的滚动与可点性同属回归面。
      engine: web({ viewport: { width: 390, height: 844 } }),
      app: {
        // 端口 0:每次 run 用空闲端口启动 Expo,避免与手工 `expo start` 冲突;{port} 由 runner 代入。
        url: 'http://127.0.0.1:0',
        command: {
          executable: 'npx',
          args: ['expo', 'start', '--web', '--port', '{port}'],
          env: { CI: '1', EXPO_NO_TELEMETRY: '1' },
          log: '.e2e/logs/expo.log',
          startupTimeout: 180_000,
        },
      },
    },
  ],
  tests: ['e2e/tests/**/*.e2e.ts'],
  // App/后端共享同一套临时栈:串行执行,避免用例互相写库。
  workers: 1,
  // 失败即失败:首版目标是连续 N 次全绿,重试只会掩盖 flake。
  retries: 0,
  timeout: 300_000,
  actionTimeout: 60_000,
  assertionTimeout: 60_000,
  reporters: ['list', 'junit'],
  trace: 'retain-on-failure',
  // 无 agent.act 步骤,重放缓存无用;关掉避免 .e2e/cache 噪音。
  cache: 'off',
} satisfies E2EConfig;
