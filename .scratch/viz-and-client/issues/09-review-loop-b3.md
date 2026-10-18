# 09: 条件核对闭环（B3）纵切

**What to build:** 产品灵魂落地：只有需要核对的报告（部分产出/失败/低置信项）进入核对，干净报告零摩擦直落档案；需要核对的报告从"上传返回秒回"当场逐项对照确认/修正，从详情页高亮处顺手补做，队列同步完成时主动提醒直达。家长改四个纸面事实，颜色永远由后端按事实重算——屏幕上剩下的数，要么高置信、要么经家长亲手确信。

**Blocked by:** 06（上传 + 时间线纵切）, 07（报告详情纵切）, 08（离线队列纵切）。

**前置小票:** `.scratch/recognizer-quality/issues/03-ocr-lines-contract-v2.md`（ocr_lines 精确化：识别侧直吐行号 + 迁移回填；本票「缩放至 OCR 行对应区域」直接消费精确行号，不再依赖文本反推）。

**Status:** resolved

- [x] 触发条件正确：仅 partial / failed / 含低置信项触发；干净成功报告免核对直落档案（上传返回分岔即归档轻提示）
- [x] 双入口同一闭环：上传当场（首选，纸还在手边）+ 详情顺手（兜底）；队列同步完成提醒「N 项待核对」并直达核对（前半句本票落地；队列提醒属票 08 范围，落地时深链核对路由 `/review/{report_id}` 即可接通）
- [x] 核对界面为逐项列表 + 确认/修正按钮（横滑对照 POC 结论记录在案）；低置信项三方对照：图像缩放至 OCR 行对应区域 ↔ 原始行文 ↔ 解析值/参考范围
- [x] 修正收 value / unit / name / ref_range 四项纸面事实；`flag` 显示永远来自服务端按值×范围重算，客户端无手改 flag 入口
- [x] 决策可重复（每次修正追加一条修正对）、无"撤销回未核对"、报告状态派生单调；确认不产生修正对
- [x] 详情页任意指标项可发起修正（高置信亦然）；核对/修正后时间线徽标与"剩余 N 项"同步更新，partial → success 单向
- [x] failed 报告核对形态 = 展示图像与失败原因不计入待核对；重拍生成新报告，原报告留痕
- [x] 详情页头部显示报告短 ID（前 12 hex，muted 小字；问题定位时可直接引用）
- [x] API 集成测试：核对状态机全转移、修正对追加、状态派生单调、flag 重算、幂等重试安全、对已核对报告重跑识别 409
- [x] 验证环境六条门禁用例中核对相关三条（当场确认+修正、免核对直落、任意项修正同步）全通过（真栈 API+客户端逻辑直跑 23/23；浏览器点击通道未跑，见披露）

## Comments

**实现（分支 `review-loop-b3`，worktree `.worktree/review-loop-b3`，单 commit，push/PR 待用户）**

- **Go API**：`cmd/hdd/serve_review.go` 新增 `PATCH /api/v1/reports/{report_id}/items/{item_id}`（契约冻结于票 02，**零契约改动**）：confirm 只置 `review_status=confirmed`（不产生修正对）；correct 覆盖传入的 value/unit/name/ref_range 四项纸面事实、flag 由服务端按修正后 value × ref_range 重算（新增 `internal/pipeline/flag.go`，与识别端 `_flag` 逐字同口径：区间 `--`/`~`/性别前缀、单侧 `<≤>≥` 含边界值同判、比较式前缀匹配、不可数值化→unknown；双侧单测锚定）、`review_status=corrected` 并 append 一条修正对。响应 = ReviewResult（报告级 status + 未核对低置信计数 + 本次指标项含行链接，单响应刷新整屏）；`toAPIItem` 从详情投影抽出，详情与核对共用。请求体严格解码：confirm 不得携带修正字段、correct 至少一项且 name 非空、手改 `flag` 直接拒收（fixture `review.request.flag.invalid.json` 的端侧执行）。404/400 口径：report_id 非 hex → 404（内容身份同详情）、item_id 非整数 → 400（同成员端点）、报告/指标项缺失 → 404（存储一次定位，不预查报告）。
- **storage**：`ReviewItem` 单事务——报告行 `FOR UPDATE` 串行化同一报告的并发核对（状态翻转不丢）→ 取指标项 → 应用动作 → **partial → success 单调翻转**（当且仅当未核对低置信计数归零）→ 读出动作后状态与计数。修正对 append-only 落 `correction_pairs`（before→after 四项全集 + raw_text + ocr_line_numbers；NULL 行号落空数组）。无新迁移。
- **App**：`src/review/`（`items.ts` 纯逻辑：触发判断/待核对项/修正草稿→请求只带变化字段/单响应合并 + flag 徽标重算；`zoom.ts` 行框并集 → 视口摆放几何；`components.tsx` **ReviewItemCard**（确认/修正按钮 + 四项事实编辑器 + 已确认/已修正标签）+ **LineZoom**（图像缩放至 OCR 行区域））；核对页 `src/app/review/[report_id].tsx`（待核对项逐项列表、三方对照、完成态、failed 深链兜底形态）；详情页（票 09 兜底入口）：**短 ID（前 12 hex muted 小字）** + 每项可确认/修正（高置信亦然）+「逐项核对 N 项低置信」入口；上传页分岔：需核对（partial/含未核对低置信项）给「立即核对」直达核对页，干净 success 轻提示已归档；两屏共用 `reports/useReportData`（加载/聚焦重载/响应合并）。`MetricItem/MetricStrip` 由 reports/components 迁入 review/components（详情与核对页同一卡）。
- **测试证据**：Go——`flag_test.go` 读共享向量表 33 例（区间/边界/性别前缀/单侧比较/不可解析/退化输入/真实语料；Python 侧 `test_flag_vectors.py` 同表）；`review_test.go` 6 例（确认不动事实、修正 before→after、flag 重算、重复决策）；`serve_review_test.go`（确认/修正/全转移/单调/幂等重放/409/校验与 404 矩阵零写入）＋票 06 的测试桩 `setReviewStatus` 删除（409 用例改经真实核对端点建决策）；`store_review_test.go` gated 真库冒烟（一次性 PG：确认/翻转/修正对内容含 NULL 行号落空数组/重放 no-op 对/缺失 nil,nil）。`go test ./...`、gofmt、go vet 全绿；mobile——jest 10 套件 63 用例（新增 review 15：items 10 + zoom 5）、typecheck、codegen 同步（无 diff）、`expo export --platform web` 打包成功。
- **共享栈真栈 e2e**（快照 `data/e2e-snapshots/pre-09-review-loop-20261018-115300.dump`，90401B）：按本 worktree 重建 hdd（无新迁移、recognizer 未动）→ tsx 直跑三条门禁用例（真实 `/api/v1` + 客户端纯逻辑 + openapi-fetch）：**23/23 PASS**——① lft_01（partial + 低置信碱性磷酸酶）上传 → 确认（响应含行链接）→ success/0 → 任意项修正 value 999 → flag 服务端重算 high → 重放幂等 → 详情/时间线一致 → 同图重传 409 → override 重置；② lft_02 干净 success 直落、`needsReview=false`；③ cbc_03（success 含低置信）触发核对 → 确认后不再触发 → 高置信项修正 → 时间线 high 徽标 +1 且详情一致。修正对 DB 证据：value 4.59→999（ref/unit/name 不动、raw_text 与行号随项）、重放 no-op 对 before=after=999（ADR-0005 决策可重复）。`e2e.py verify` 全 ok（warn=RDW-CV 印刷串列与斜拍裕度，均已知良性）。冒烟行已 SQL 清理（库回 16/16/198/0/6 基线）。

**Decision / 有意偏离**

1. **partial → success 的翻转点**：读路径不派生（时间线/详情查询零改动），由核对动作在写事务里翻转并把最新状态/计数随响应返回（ADR-0005「items 装最终生效值、下游零 join」）；报告行锁保证两枚并发确认不会各看未提交状态而都不翻转。
2. **flag 重算双实现口径以识别端为准**：Go `ComputeFlag` 与 Python `_flag` 逐字对齐（同一输入两处同判）；识别端 `≤`/`≥` 的边界值与 `</>` 同判（`op in "<≤"` 分支），Go 照抄不「修正」——改识别端口径会动 golden，非本票范围。**复审后语义单点化**：共享向量表 `schemas/testdata/flag.vectors.json`（33 例）双侧读同一份测试，漂移必红；运行时回调识别服务（修正新增运行时耦合）与整体搬家（动识别端契约与 golden）两条路均判定为更差代价，不做。
3. **404 不区分报告/指标项**：两者对客户端是同一件事（同票 07「非法形态与不存在同答」立场），存储一次定位即答。
4. **确认不重算 flag、修正一律重算**：confirm 不改事实；correct 即使只改 name/unit 也全量重算，维持 `flag = f(value, ref_range)` 不变量。
5. **重放的修正 append 一条 no-op 对**：ADR-0005「每次修正 append、决策可重复」的字面执行；重放最终生效状态不变（离线补传安全），不引入去重机制（修正对本期只存不回流，无消费者）。
6. **核对页列表口径 = 未核对低置信项**（「剩余 N 项」同口径）：success 含未核对低置信项同样列出（含低置信项触发）且确信后不再触发（`needsReview` 只看未核对）；页面不显示自算的「N 项待核对」数，避免与服务端只统计 partial 的派生计数打架（徽标以 ReportCard 为准）。partial 但零低置信项（如仅缺日期/缺值，真实语料 ana_04 形态）进入核对页为空态 + 详情出口——冻结计数口径不含非低置信缺项，留给后续真实使用反馈。
7. **failed 的核对形态落在详情页**（票 07 已有：图像+原因+重拍）；核对路由对 failed 深链给同形兜底、不设入口。
8. **`needsReview` 只认未核对低置信项**：已确信（confirmed/corrected）的低置信项不再触发核对，与待核对列表同口径。

**披露 / 留给维护者**

- **队列同步完成提醒**（清单第 2 项后半句）：票 08（离线队列）未实现，用户明确本次只做 09；落地时深链核对路由 `/review/{report_id}` 即可接通（核对页与徽标已就绪）。
- **浏览器交互验收未跑**（本机无浏览器操作通道）：三条门禁用例已以真栈 API + 客户端逻辑直跑（23 项断言），但未见真实点击。建议经 Expo Web 点检：上传 lft_01 →「立即核对」→ 三方对照（缩放/原文/解析值）→ 确认与修正 → 徽标/剩余数更新；详情页短 ID 与「逐项核对」；上传 lft_02 直落轻提示；failed 详情形态。Web 通道需临时 CORS：`HDD_SERVE_CORS_ORIGIN='*' docker compose -p deploy -f deploy/docker-compose.yml up -d hdd`（共享栈 hdd 现为本分支最终镜像，供验收；合并后按 main 重建即可）。
- **POC 记录在案**：核对界面采用**逐项列表 + 确认/修正按钮**（trust-first 决策：横滑误滑=误确认、代价不对称，横滑仅作对照，不采用）；低置信项三方对照（图像缩放至 OCR 行区域 ↔ 原文 ↔ 解析值/参考范围）落在 ReviewItemCard。

**复审收口（2026-10-18 维护者二审三点）**

1. **已确认项确认按钮**：改为 `review_status=confirmed` 时禁用（再点是无变化空操作）；修正保持可用（误确认的出口是修正）；已修正项仍可再确认/再修正（ADR-0005 决策可重复，与服务端状态机一致）。
2. **flag 语义单点化**：共享向量表 `schemas/testdata/flag.vectors.json`（33 例，含真实语料与识别端 `≤`/`≥` 边界口径），Go `ComputeFlag` 与识别端 `_flag` 双侧读同一份——两套实现绑一套语义，改语义先改向量表、漂移必红。运行时复用与整体搬家两条路判定为更差代价（见 Decision #2）。
3. **「生化（未知日期）」根因**（维护者实测）：`091b5de9….jpeg`（生化）图上印有 `送检/检验/报告时间 2026/05/16`，OCR 亦完整识别，属**抽取漏**而非 OCR 未识别——`_find_date` 标签/值分格的几何兜底按小图校准（`Δcx ≤ 300px`），该照片宽 3316px 下 `检验时间：→2026/05/16` 的 Δcx=322.6 恰好越界；且独立日期行兜底只认破折号形态。**抽取修复另开识别质量小票 `.scratch/recognizer-quality/issues/05-date-extract-scale.md`（独立分支/PR）**；`glu_01.png` 截图无日期行属真缺失——「手动补录日期只面向日期不存在的图像」的规则记录在 05 票面与本文，待维护者定实现去留。
