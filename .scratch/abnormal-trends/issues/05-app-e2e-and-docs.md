# 05: App E2E 与文档收口

**What to build:** 给异常指标趋势补确定性 App E2E（独立 seed + 专用栈回放），覆盖多点 / 单点 / 无异常 / 无报告四条状态，并同步 `docs/agents/app-e2e.md` 完成判据与用例清单。只改测试/文档/回放配置，不改生产逻辑。

**Blocked by:** 03、04。

**Status:** ready-for-agent

## 验收清单

### Seed 与回放

- [ ] `apps/mobile/e2e/support.ts` 新增 `seedTrendData()`，独立于 `seedData()`：
  - 多点成员（新建，如 `E2E 趋势`）：上传 `lft_05.jpeg` + `lft_02.jpg`。
  - 单点成员（新建，如 `E2E 趋势单点`）：上传 `lft_04.jpeg`。
  - 无异常成员（新建，如 `E2E 趋势无异常`）：上传 `lft_03.png`。
  - 无报告成员（新建，如 `E2E 趋势空`）：只建成员，不上传。
- [ ] `seedTrendData()` 在 beforeAll 显式断言 fixture，失败时点名样本与缺口：
  - `lft_05`：`report_type=生化`、`report_date=2017-11-18`、丙氨酸转氨酶 `27.40 U/L` normal。
  - `lft_02`：`report_type=生化`、`report_date=2021-08-02`、`flag_counts` 的 high+low+abnormal = 6、丙氨酸转氨酶 `835 U/L` high。
  - `lft_04`：`report_type=生化`、`report_date=2011-07-02`、3 项异常、总胆红素 `41.00 μmol/L` high。
  - `lft_03`：`report_type=生化`、0 项异常。
- [ ] `apps/mobile/e2e/compose.yml` 的 `RECOGNIZER_REPLAY_SAMPLES` 追加 `lft_02,lft_03,lft_04,lft_05`；这些样本的 golden 已存在。
- [ ] 不修改主 `app.e2e.ts` 的 seed/断言；新用例独立文件、独立成员，不与主链路共享图片。

### E2E 用例

- [ ] 新增 `apps/mobile/e2e/tests/trends.e2e.ts`，一条串行用例（5 组断言、8 个步骤）：
  1. 用多点成员打开 `/`，点 `overview-trends` 进入 `/trends`。
  2. 断言分组 `生化 · 2021-08-02 · 6 项异常`；行「丙氨酸转氨酶」「835 U/L」「偏高」「2 份报告」；`trend-indicator-0` / `trend-sparkline-0` 可见。
  3. 点该行打开详情；断言 `trend-detail`、`trend-detail-chart`、`最新：生化 · U/L`、`方法：—`（lft_02/04/05 的 method 为空；lft_03 有 method，但本用例不用）；逐点列表 2 行。
  4. 点 `trend-point-0`（2017-11-18）；断言点卡切换为 `27.40 U/L`、`正常`。
  5. 点 `trend-detail-close`；断言回到列表。
  6. 切到单点成员；断言 `生化 · 2011-07-02 · 3 项异常`、`仅 1 份报告`；打开详情，逐点列表 1 行。
  7. 切到无异常成员；断言「这位成员暂时没有异常指标。」。
  8. 切到无报告成员；断言「这位成员还没有检查单。」。
- [ ] 断言正常项不出现（如 lft_02 的「总胆红素」正常，不在列表中）；assert testID 数量/文本，避免只断言页面标题。
- [ ] 不依赖 `app.e2e.ts` 的成员/报告（避免文件间顺序与图片 sha256 去重冲突）。

### 文档

- [ ] `docs/agents/app-e2e.md`：
  - 完成判据 14/14 → 15/15。
  - 「用例与选择器」增加 `trends.e2e.ts`（1 条）与说明：独立成员 + lft_02/lft_04/lft_05/lft_03 fixture。
  - 回放样本列表更新。
- [ ] 本 spec 中的 E2E 样本名与实际一致。
- [ ] 不新增票据之外的依赖；不改 CI workflow（路径过滤已覆盖 app/e2e）。

### 验证

- [ ] `npm run e2e` 全绿（15/15）；首次会重建专用栈，属预期。
- [ ] 失败时 `.e2e/results/<test>/trace.md` 与 `.e2e/logs/expo.log` 可定位。
- [ ] `npm run typecheck`（含 e2e tsconfig）、`npm test` 全绿。

## 实现提示

- 独立成员是必须的：图片按 sha256 全局去重，同一张 `lft_02.jpg` 不能再属于另一个成员；主 `app.e2e.ts` 不使用 lft 样本，正好可独立。
- `uploadSample` 在 `duplicate=true` 时已直接失败并提示「栈未重置」；新 seed 沿用即可。
- 多点成员的最新报告是 `lft_02`（2021-08-02）；`lft_05`（2017-11-18）是趋势历史点。
- `lft_02`/`lft_04`/`lft_05` 的 `method` 为空，详情断言 `方法：—`；`lft_03` 的 method 有值，但本用例不用；method 有值的渲染路径由 storage/handler 字段测试覆盖，手工用真实数据再验。
- 点列表默认选中最新报告点；`trend-point-0` 是日期升序的第一行（2017-11-18）。
- 详情图的选中点放大是视觉断言，E2E 只断言点卡切换；`selectedIndex` 的图形几何由 `trends.test.ts` 覆盖。
- 测试前显式断言 `lft_02` 的 6 项异常，避免 golden 变化时用例在页面里以 undefined/0 行失败。
- 不改 `app.e2e.ts` 的 11 条计数；新增文件只加 1 条（总 15/15）。

## Comments

（待实现）
