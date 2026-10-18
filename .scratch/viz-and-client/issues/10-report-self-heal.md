# 10: 自愈：改归属 / 删除纵切

**What to build:** 传错成员不必删档重传（内容哈希去重挡着重传的路）：报告详情菜单里改归属即刻生效；拍糊了的报告可以干净删除，原图与识别历史留痕保留、同图日后可重新归档。

**Blocked by:** 06（上传 + 时间线纵切）, 07（报告详情纵切）。

**Status:** resolved

- [x] 详情页菜单两项：改归属（选择目标成员后时间线立即移位；点选 → 「确认改派」两段式提交，维护者 2026-10-18 增补）与删除（需确认步骤防误触）
- [x] 删除后报告从时间线消失，同一张图可重新上传入档（内容哈希去重不挡重新归档）
- [x] 删除边界精确：报告、指标项、修正对随之清除；原始图像与 OCR 历史留痕保留（append-only 文化不破）
- [x] API 集成测试：改归属、删除级联范围、删除后重传路径
- [x] 验证环境门禁用例（API 真栈冒烟已过：传错成员改归属移位；删除后同图重传归档——浏览器交互点检见「披露」，待维护者复验）

## Comments

**实现（分支 `report-self-heal`，worktree `.worktree/report-self-heal`，单 commit，push/PR 待用户）**

- **Go API**（PATCH/DELETE `/reports/{report_id}` 契约 M0 已冻结，本票落地）：`cmd/hdd/serve_reports.go` 两端点——改归属（body `member_id` 必带整数；显式 null/未知字段/畸形 JSON 400，未知成员/未知报告 404，200 回刷新后的报告头部）与删除（204 无响应体；不存在/非 hex 404）。`pipeline.ReportStore` 增 `DeleteReport(ctx, sha) (bool, error)`；storage 实现 = 单条 `DELETE FROM reports`，清除边界由既有 FK 承担（report_items ON DELETE CASCADE、correction_pairs 随指标项级联），images/ocr_results 零触碰。改归属复用既有 `SetReportMember`（不新增写路径）。
- **App**：`src/api/reports.ts` 增 `reassignReport`/`deleteReport`；`src/reports/components.tsx` 增 `ReportManageCard`（详情页底部管理卡）——「改归属」展开成员选择（复用 `MemberChips` + `sortMembers`，孩子优先）：点选只改选择态（与全 app 成员 chips 的「选择」语义一致），确认行显式给出「从「A」改派给「B」?」（无主报告只给目标侧），「确认改派」才提交 PATCH；当前成员点按视为保持原样（清除选择态，不产生请求）；「取消」关闭面板零写入。「删除」展开确认步骤（文案声明清除边界 + 「确认删除」/「取消」防误触）。改归属成功就地并入响应（`useReportData` 增 `applyReassign`，只并 `member_id`）；删除成功 `router.dismissTo('/timeline')` 回时间线（聚焦重载即见卡片消失；深链直入时替换为时间线，同一出口）。
- **测试证据**：Go——`serve_reports_test.go` 新增 `TestServeReassignReport`（归属即刻生效、时间线移位与改回、除归属外零变化、重复改派幂等、未知成员 404 零写入）、`TestServeReassignConcurrentDelete`（摘要读出后并发删除的竞态路径回 404 而非 500）、`TestServeReassignValidation`（10 例：空 body/缺字段/null/字符串/浮点/未知字段 400；非 hex/未知报告/未知成员 404；全部零写入）、`TestServeDeleteReport`（204 无响应体；报告/指标项/修正对级联清除；images/ocr 留痕保留；删除后同图重传 201 非 duplicate、item_id 全换、识别重跑、OCR 历史追加；重复删除与非法形态 404）；fake 增 `DeleteReport` 镜像 FK 级联（修正对记录带 item_id 以断言级联）。`store_reports_test.go` 真库 gated 冒烟扩删除与重归档（一次性 PG 容器跑过）：修正造对 1 条 → 删除后 reports/items/pairs 0/0/0、images/ocr 1/1 → 再删 false,nil → 同图重写报告回读成功。`go test ./...`、`gofmt`、`go vet` 全绿；契约互证（fixtures/path set）未动、照常绿。mobile——typecheck、jest 11 套件 70 用例全绿（新增 `reports/manage.test.ts` 7 例：改归属合并只并归属、同成员点选不产生请求、无主必改派、确认行 from→to 文案两态）、codegen 零漂移、`expo export --platform web` 打包成功。
- **共享栈真栈 e2e**（快照 `data/e2e-snapshots/pre-10-report-self-heal-20261018-175308.dump`，98011B；仅重建 hdd，识别端未动免 refresh）：派生冒烟图（tft_03 + 6 字节白边）上传成员 5 → 201 success 6 项 → PATCH 改派成员 6 → 200、timeline(5) 无/timeline(6) 有 → 修正一项（造 1 条修正对）→ DELETE 204 → 详情 404、时间线消失、SQL reports/items/pairs 1/6/1→0/0/0、images/ocr 1/1 保留 → 同图重传 201 duplicate=false（对已核对报告删除后再传不落 409）、item_id 全换、pairs 0、OCR 历史 1→2 → 收尾 API 再删 + SQL 清冒烟 images/ocr 行，库回基线（members=6、images=reports=17、items=223、reviewed=0、pairs=0）。负例：未知成员改派 404、`member_id:null` 400、未知/非 hex 删除 404。`e2e.py verify` 全 ok（仅 RDW-CV 印刷串列与斜拍裕度两条已知 benign warn）。**评审收口后按同一序列在新镜像复跑：结果一致，库回基线、verify 全 ok。**

**Decision / 有意偏离**

1. **删除用单条 `DELETE FROM reports`（FK 级联）而非存储层显式多表删除**：级联语义已被迁移 FK 与 migration 冒烟锁定，三表逐个手删是双源真相；`DeleteReport` 只用 RowsAffected 判存在性（false, nil，404 口径与 SummaryByImage 缺失同惯例）。
2. **PATCH 校验次序成员先于报告**（与上传端点一致）：契约 404 语义含 unknown member；换来的是同文件内一致的错误面，不引入新机制。
3. **改归属就地合并只并 `member_id`**：改派不动报告内容（状态/计数/指标项），并入完整头部反而可能覆盖本地更新的服务端响应，不符合票 09 的单响应合并口径。
4. **删除后 `router.dismissTo('/timeline')`**：时间线在栈中则弹回（聚焦重载，卡片消失可见），深链直入不在栈中时替换为时间线——两种入口同一出口。
5. **管理卡放详情页底部**：低频/破坏性操作避开阅读主路径（头部 → 核对入口 → 指标项 → 原图）。两动作都走显式提交：改归属在点选后经「确认改派」提交（确认行给出 from→to）、删除经确认步骤（维护者 2026-10-18 增补，见下）。

**评审（两轴 sub-agent：Standards / Spec）后收口**

- 已修（Standards 全量；复审后按维护者指令逐项收口）：
  1. 重复文案与身份守卫单点化：`reportIDFromPath`（详情/核对/改归属/删除共用的 404 守卫）+ `reportNotFoundMessage`/`memberNotFoundMessage` 常量（serve.go）；`requireMember` 收口上传/改归属的成员校验；测试 helper `sameDate` 改名 `sameOptionalString`（它比较的是任意可空字符串）。
  2. 改归属竞态路径 404：`SetReportMember` 改签名 `(bool, error)`（与 `DeleteReport`/`SummaryByImage` 同一缺失口径）——摘要读出后报告被并发删除时回 404 而非 500；新增 `TestServeReassignConcurrentDelete` 锁住该路径。
  3. 移动端新增纯逻辑可直测：新模块 `reports/manage.ts`（`applyReassignResult`/`needsReassign`）+ `manage.test.ts` 5 例；数据挂载与管理卡改为消费该模块（UI 行为仍归验证门禁，spec 口径）。
  4. API 解包单点：`api/client.ts` 增 `unwrap`/`assertOk`，`reports.ts`/`members.ts` 全部改用（消 5 处内联解包）。
  5. 样式原语单点：新模块 `components/styles.ts` 收口 `horizontal/label/muted`（此前 5–6 份拷贝），9 个文件改用 `sharedStyles.*`；`manageCard` 改为复用 `card` + `{gap:8}`。**两处微视觉对齐**（消除近似变体）：members 页 muted 行高随共享原语（20）、review 页横向行间距 12→8——浏览器验收时顺带留意。
  6. Spec 轴：验收 #5 勾选加注「浏览器交互点检待维护者复验」，不以 API 冒烟冒充 UI 验收。
- 保留（判定为 judgement call）：其余逐 handler 错误文案（`lookup report failed` 等）未常量化的处理——分属不同失败面、无跨端点复用语义，本轮只收口被指出的复核面；决策 #2（PATCH 成员先于报告校验）、#3（只并 member_id）、#5（管理卡形态）不变。

**维护者指令增补（2026-10-18）：改归属两段式确认**

- 背景：原实现「点选目标成员即刻 PATCH」与全 app 成员 chips 的「选择」语义不一致（时间线过滤、上传归属都只改本地态）且误触为静默错分；评估后维护者拍板加确认。数据面零改动（仍是单次同步 PATCH，「即刻生效」语义不变），仅客户端多一状态阶段。
- 交互定稿：点选 = 选择态（高亮目标；点当前成员清除选择）；确认行「把报告从「A」改派给「B」?」（无主报告「把该报告改派给「B」?」）+「确认改派」/「取消」（取消关闭面板、零写入）；PATCH 失败停留确认态可重试。
- 纯逻辑 `reassignConfirmText` 进 `reports/manage.ts`（+2 例覆盖有主/无主两态）；UI 行为仍归验证门禁口径。

**披露 / 留给维护者**

- **浏览器交互验收未跑**（本机无浏览器操作通道）：共享栈 hdd 现为**本分支镜像**（供验收；合并后按 main 重建即可）。建议点检：详情底部「改归属」→ 点选另一成员 → 确认行 from→to → 「确认改派」→ 提示改派成功 → 返回时间线看移位；「取消」不产生任何写入；「删除」→ 确认面板 → 确认 → 回时间线看消失；同图可重新上传入档。Web 通道需临时开 CORS：`HDD_SERVE_CORS_ORIGIN='*' docker compose -p deploy -f deploy/docker-compose.yml up -d hdd`。
- 冒烟派生图已随 SQL 清理（worktree `data/` 下文件已删）；SeaweedFS 对象一枚为惰性孤儿（对象不随行删，同图重传幂等覆盖；与 06/07 冒烟惯例一致）。

