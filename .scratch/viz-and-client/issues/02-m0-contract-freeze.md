# 02: M0 契约冻结

**What to build:** 需求文档 v1.3 定稿的 HTTP API 面（12 端点）与其全部数据形状一次冻结：OpenAPI 契约、数据库迁移、Go 契约镜像、HTTP fixtures 四者一致——此后所有实现票的 API 测试都有锚，识别端契约与存量识别 golden 零触碰。

**Blocked by:** 01（`hdd serve` 骨架落真栈，已 resolved/merge #11）。

**Status:** resolved

- [x] OpenAPI 契约含全部 12 端点与形状：成员（显示名 + 是否孩子）；报告（归属成员可空、fail_reason）；指标项（核对状态 + 稳定外部句柄 + ocr_lines 行号与框坐标）；修正对表形状（before→after 全集 + 原始行文 + OCR 行号）；trend 单位门控参数
- [x] 修正字段集契约化：核对修正只收 value / unit / name / ref_range，`flag` 由服务端按值×参考范围重算，契约不提供手改入口
- [x] 新迁移落库：成员表；报告表加成员归属（可空，存量 CLI 时代报告无主）与失败原因列；指标项表加核对状态；append-only 修正对表
- [x] 状态派生语义进契约：未核对低置信计数为响应派生值（不落列）且只统计 partial 报告；报告状态派生单调（partial→success 单向，success/failed 恒定）
- [x] 对已含核对决策的报告重跑识别 → 409（显式覆盖才放行）写入契约与 fixtures
- [x] Go 契约镜像与 OpenAPI 一致性测试绿（沿用识别端双向互证模式）；HTTP fixtures golden 建立并对账测试绿
- [x] 识别端契约文件与存量识别 golden 逐字节不动（本票不改识别行为）

## Comments

**实现（分支 `m0-contract-freeze`，worktree `.worktree/m0-contract-freeze`，single commit，push/PR 待用户）**

- `schemas/openapi.yaml`：12 端点全形状冻结（health / members GET·POST / members{id} PATCH / reports GET·POST / reports{id} GET·PATCH·DELETE / reports{id}/items{item_id} PATCH / images{sha256} GET / members{id}/indicators/{name}/trend GET）。要点：
  - **报告身份**沿全仓约定 = 图像内容 sha256（`report_id` 64 hex 正则冻结）；**指标项身份** = BIGSERIAL `item_id` 不透明句柄（重跑全换，契约描述明言"不可当持久主键"）。
  - **状态派生语义**进契约文本：`unreviewed_low_confidence_count` = 响应派生值（不落列），只统计 partial 报告中 `review_status=unreviewed 且 low_confidence` 的项，failed/success 恒 0；`ReportStatus` 描述 partial→success 单向、success/failed 恒定，唯一例外 = 显式 override 重跑（重置归档）。
  - **修金字段集**：`ReviewRequest` 以 `oneOf` 冻结——confirm 分支仅 `action`；correct 分支必须至少携带 value/unit/name/ref_range 之一；无 flag 字段，`additionalProperties: false` 直接把手改 flag 的请求打为非法（invalid fixture 锚定）。
  - **同图重传矩阵**（含离线队列重试安全约束，见下 Decision）：新图 201 `{duplicate:false}`；命中且无核对决策 200 `{duplicate:true}`；含核对决策 409；`override:true` 显式放行重跑 → 201，item_id 全换、review_status 回 unreviewed。
  - **指标项共享八字段**（name/value/unit/ref_range/method/flag/low_confidence/raw_text）与识别端 `report.schema.json#/$defs/reportItem` **镜像而非 $ref**：识别端 schema `additionalProperties:false` 已冻结，JSON Schema 2020-12 的 allOf 扩展会与之互斥校验——镜像 + 互证测试锁死逐字段子树相等（见下）。重算语义与 HTTP 侧说明只写在 PATCH 核对端点与 ReviewRequest 描述中，不加进共享字段描述。
  - **trend 单位门控**:`?unit=` 指定归一单位 → 只返回同单位点;缺省返回全部单位族点,跨单位族不连线不提示;`reviewed` 派生信任标记;dateless 点沉底。`prev`（B2-lite）形状 = `{report_date, value, unit}`,基线定义(严格早于/宁缺毋错)随描述冻结。
- `internal/httpapi/`(新包):HTTP 契约的 Go 镜像（Member / MemberCreate / MemberUpdate / MembersList / FlagCounts / ReportHeader(嵌入于 Summary·Detail) / ReportSummary / ReportDetail / OcrLineLink / PrevValue / ReportItem / UploadResult / ReviewRequest / ReviewResult / CorrectionValues / CorrectionPair / TrendPoint / TrendResult / Error)。仅形状,零业务逻辑;后续实现票的 handler 直接用本包类型做响应/请求绑定。
- `internal/storage/migrations/0004_m0_member_review_corrections.sql`:members 表(name+is_child 最小模型);reports 加 `member_id BIGINT REFERENCES members(id)`(可空)与 `fail_reason TEXT`;report_items 加 `review_status`(默认 unreviewed,CHECK 三枚举);correction_pairs append-only 表(item_id 级联删除、before/after 四字段全集快照、raw_text、`ocr_line_numbers INTEGER[]` 0-based、created_at)。
- HTTP fixtures(`schemas/testdata/http/`,11 枚,值全部取自真实识别金标 `samples/expected/report/*.json` 与既有契约 fixture `schemas/testdata/*.json`):member、report.summary(partial + 5/2/1 徽标口径)、report.summary.failed(fail_reason 形状锚点)、report.detail(cbc_02 真实 partial:单核细胞比率 10.8 % 3-10 high low_confidence=true,item_id=7,prev = cbc_01 真实基线 9.10 % @2009-03-21)、upload.duplicate(200 幂等形态)、report.conflict(409 错误体)、review.request.confirm/correct、**review.request.flag.invalid(必须被 schema 拒绝的负例——手改 flag)**、review.result(修正 5.6 × 3-10 → flag 服务端重算 normal + 报告派生 partial→success + 计数归零,数值取 cbc_05 真实单核细胞比率)、correction.pair(10.8→5.6 before/after 全集)、trend(血红蛋白 4 点真序列:cbc_01@2009-03-21 151、cbc_05@2019-08-07 139.0、cbc_04@2024-08-19 118、cbc_02 dateless 135 沉底;report_id 全部为 samples 真图 sha256)。
- 互证测试(`internal/httpapi/contract_test.go` + `schema_test.go`):
  1. `TestOpenAPIFrozenPathSet`:12 端点集合双向相等(实现票多/少/改名即失败);
  2. `TestOpenAPIMirrorsRecognitionItem`:HTTP 指标项八个共享字段 schema **子树级语义全等**(json 数值/键序归一后逐字段等价)于 report.schema.json 的 reportItem(两份定义的漂移在 CI 就死);
  3. `TestFixturesReconcileOpenAPI`:10 fixtures 各自对目标 component 校验 + flag.invalid **反例拒绝**;
  4. `TestFixturesRoundTripGoMirror`:fixtures ↔ Go 镜像双向(不丢字段不变形);FlagCounts 键序单测;
  5. `TestOpenAPIRefIntegrity`:全文档 $ref 无悬挂。
  - validator 为测试内迷你 2020-12 实现(合约只用到的 keyword 白名单),新增组合能力必须先扩展该表——防止契约面无意识扩大。
- `internal/storage/migrate_test.go`:`HDD_TEST_DSN` 门控迁移冒烟(用一次性 PG 容器跑);未设时 Skip,共享栈验证走 e2e 外环。

**Decision(与维护者对齐后定稿)**
1. **409/override 矩阵**：命中已有且无核对决策的报告 = 200 duplicate（离线队列重试安全——若命中即 409，票 08 重试被毁）；命中且含核对决策 = 409（已确信的数字不被同图重传悄悄覆盖/重跑）；`override=true` 为显式覆盖放行重跑（201，核对状态按新识别重置）。选择性符合 spec「显式覆盖才放行」字面，且正常离线重试不踩 409。
2. **时间线徽标计数**:`flag_counts = {high, low, unknown}` 三档必填(最小口径满足「3 项偏高」徽标;normal 不计,详情页逐项染色)。

**验证证据**
- `gofmt -l` 空、`go vet ./...` 零输出、`go test ./...` 全绿(6 包)。
- 迁移冒烟:一次性 `postgres:16-alpine` 容器 + `HDD_TEST_DSN` → goose 0001→0004 全应用、幂等重跑无害;断言:members 仅 4 列(最小模型)、reports.member_id/fail_reason 可空、review_status 默认 'unreviewed' 且 CHECK 拒非法值、修正对随报告删除级联清除而 images 保留(2/0/0→删后 items=0 pairs=0 images=2)。
- 识别端零触碰:`git diff origin/main -- recognizer/ schemas/*.schema.json schemas/testdata/*.json` 为空(本票仅在 schemas/testdata 下**新增** `http/` 目录)。
- Python 侧未跑(未改任何 .py);lefthook 与 CI 双链在 commit 时把关。

**披露 / 留给下票的口子**
- **共享栈 version 4 的走**:e2e.md 外环(schemas/迁移 → refresh → goose 自动升版)刻意**未在本票执行**——票 01 会话定下「快照守护 + 版本推进采纳成本」纪律,本票选择一次性容器完成迁移证明,共享栈升 4 将采纳到下一张动共享库的票(05/06)首跑 refresh 时生效;PR 合入前无需再动栈。若维护者希望本票内完成栈上刷新,票 05 开工首步即做,快照锚点先于刷新(e2e.md 序列不变)。
- 迁移冒烟测试属"门控 Skip"型:CI 默认跳过,接环境变量即跑——接线到 CI 属票 05+ 的常规门禁扩展,本票仅本地真实运行(容器已销毁)。
- `serve` 健康端点的 `version` 仍 0.1.0(与 openapi info.version 同源)——契约冻结不隐含版本号跳变;票 05+ 起随响应变更另行讨论。
- go.mod 新增直接依赖 `gopkg.in/yaml.v3`(openapi.yaml 是契约唯一源,Go 测试必须能读 YAML;间接拉入 `github.com/kr/text`)。
- 细节校验(minProperties/minLength/pattern 等)由 Go 迷你 validator 提供且识别端侧 pytest 未动;TS codegen(票 04)将直接以本 openapi.yaml 生成。

**验收对照(ticket 项)**:12 端点+形状冻结 ✅;修正字段集契约化(flag 无入口 + invalid 反例)✅;迁移四件落库(一次性容器冒烟)✅;派生语义进契约✅;409+override 契约与 fixtures✅;Go 镜像互证 + HTTP fixtures 对账绿✅;识别端零触碰(diff 为空)✅。

**code-review 修正(两轴并行评审后并入本 commit)**
- 🔴 修复:迁移冒烟的 CHECK 断言之前会因 FK/NOT NULL 等旁支报错「碰巧失败」而假绿——契约加名 `review_status_check` 并按约束名精确断言,且种子数据前置到子测试共享(保证触发点就是 CHECK 本身)。
- 修复:`report.conflict.json`(409 错误体 fixture)起初只写不打进对账表——已并入单表对的 schema 校验 + Go 镜像回转(11 枚全对账)。
- 修复:两份 fixture 清单(校验表/回转表)合并为单一 `reconcileCases` 表,消漂移;`jsonString`/`jsonCompact` 重复 helper 合一;validator 头注释乱字修正;`CREATE TEMP TABLE junk` 死码删除。
- 三处有意保留(对抗性审查后):(1) 测试内手写迷你 validator 而非引重型库——关键字白名单本身就是「契约面无意识扩大」的防线,识别端互证的另一文件经子树全等测试单独锁死,不需要完整库;RFC 评审后确认。(2) 迁移外环(refresh→goose 升版)按 e2e 表属本票改动类型,有意延后到下一张动共享库的票(05/06)首跑执行,一次性容器先行证明——共享栈升 4 的时机与快照锚点仍守 e2e.md 护栏。(3) `correct` 分支 value/unit/ref_range 为非空 string(修正=照纸面书写新值;修正清单空值 via 不传而非 null)。
- 评审提示的未来项:若契约将来新增组合关键字,须先扩 `schema_test.go` 白名单;若 oneOf 分支放宽到可重叠,需把 matchesOneBranch 升级为「恰好一个」。
