# 06: 上传 + 时间线纵切（在线路径）

**What to build:** 家长第一次把一张真实检查单变成档案里的卡片：验证环境选图、归属到成员、同步等待识别（约秒级）、时间线上出现带异常计数徽标的报告卡片。重复上传同一张图只归档一份；失败的图留痕但不骗人。

**Blocked by:** 05（成员纵切）, 03（空 items ⇒ failed 不变量）。

**Status:** resolved

- [x] 上传前选择/确认归属成员；multipart 直传、服务端原子接收（无分片）、同步等待识别，等待体验可感知（进度/loading）且有超时提示
- [x] 同步响应驱动页面：成功轻提示；识别失败展示后端错误信息
- [x] 图像内容哈希幂等：重复上传返回已有报告并标记 duplicate，不重复归档
- [x] 失败路径填充失败原因（流水线产出），时间线可见
- [x] 时间线按当前成员过滤：报告类别、日期、状态文案（识别完成/待核对/识别失败）、按 flag 聚合计数徽标与"待核对 N 项"；failed 不计入待核对计数
- [x] 日期缺失的报告不进时间线日期分组错位（null 日期展示不破坏排序）
- [x] 空列表引导上传；加载与错误可重试
- [x] API 集成测试：同步语义、幂等、失败留痕、成员过滤；fixtures 对账绿；真栈 e2e 冒烟扫通上传→时间线

## Comments

**实现（分支 `upload-timeline`，worktree `.worktree/upload-timeline`，单 commit，push/PR 待用户）**

- **Go API**：`cmd/hdd/serve_reports.go` 两端点——`GET /api/v1/reports?member_id=`（时间线卡片，缺省全部含存量无主；未知成员 200 空信封、非整数 400）与 `POST /api/v1/reports`（multipart：image + member_id + 可选 override）。上传为原子接收（一次读全图，无分片；25 MiB 上限防巨包），严格表单校验（未知字段/缺件/空图/非法 override 一律 400，未知成员 404）。sha256 幂等分岔：命中且无核对决策 → 200 duplicate:true（识别不重跑）；已有核对决策 → 409（文案锚定冻结 fixture `report.conflict.json`）；override=true → 重跑识别、items 全量重置（item_id 全换、review_status 回 unreviewed）。错误全部走契约 `Error` 形状。
- **pipeline**：`IngestImage`（从 `Ingest` 抽出字节入口，CLI 行为零变化——`Ingest` 读文件后委托）；身份计算收口 `ImageSha256Hex`（handler 预查重与 ingest 写库同源同一函数，防两处编码漂移）。新窄端口 `ReportStore`（ListTimeline/SummaryByImage/ItemsWithReviewStatus/SetReportMember——ISP 惯例同票 05，serve 经 `deps.reports` 注入同一 `*storage.Store`）；`MemberStore` 增 `MemberByID`（上传归属校验）。类型：`ReportSummary`/`ReportItemRow`/`FlagCounts`。
- **storage**：`sqlTimeline`/`sqlReportSummaryByImage`（LATERAL 聚合：flag 计数 + unreviewed_low 以 `r.status='partial'` 为门——success 免核对、failed 无项均恒 0，ADR-0005 响应派生不落列）；`sqlItemsWithReviewStatus`（含外部句柄与核对状态）；`sqlReportSetMember`（只写归属，0 行更新报错不静默，override 重跑经 upsert 不触碰 member_id）。排序日期降序 NULLS LAST + sha 升序。
- **App**：`src/app/timeline.tsx` 时间线（成员 chips 过滤 = 当前成员、卡片含类别/日期/状态文案/flag 徽标/「待核对 N 项」、failed 展示 fail_reason、空态引导上传、加载/错误可重试）；`src/app/upload.tsx` 上传（归属默认当前成员可换、拍照/相册选图+预览、上传中 spinner + 计时，超 10s 提示确认后端可达、结果轻提示/失败原因/重复提示/硬错误可重试）；`src/reports/status.ts`（状态文案、徽标、上传结果文案——纯逻辑）+ `order.ts`（日期降序 dateless 沉底、同日期 sha 升序）+ `components.tsx`（ReportCard）；`src/api/reports.ts`（openapi-fetch，multipart 经 bodySerializer + platform 装包）；`src/platform/imageUpload.ts`（与选图分离的装包抽象：Web File 直传 / 原生 {uri,name,type}）；`MemberChips` 复用组件；首页入口 + 路由注册。
- **测试证据**：Go——`serve_reports_test.go` 内存 fake（同源实现 Store+ReportStore，镜像 storage 语义）覆盖 201 全形状（含 ocr_lines 序列化为 `[]` 非 null、prev null）、幂等重传（item_id 不变/识别不重跑/归属不被夺走）、无主存量自愈、failed 留痕 + 重复返回同一失败报告、409/override 双路径、校验矩阵（7 例 + 非 multipart，拒绝零写入）、识别硬故障 500、时间线过滤/计数（partial 待核对、success 恒 0、failed 计数 0 + fail_reason、无主含于全量、未知成员空信封）；`store_reports_test.go` 真库 gated 冒烟（一次性 PG：排序/过滤/聚合/回读/归属写入/重跑保归属）。`go test ./...` 全绿（fixtures 对账未动、照常绿）。mobile——jest 7 套件 30 用例（新增 imagePartValue 3 + status 文案 11 + sortReports 3）、typecheck、codegen 同步、`expo export --platform web` 打包成功。

**共享栈真栈 e2e**（snapshot `data/e2e-snapshots/pre-06-upload-timeline-20261016-145424.dump`，82412B）

- 按本 worktree 重建 hdd（`docker compose -p deploy build hdd && up -d`；goose 4 无新迁移）→ curl 冒烟：真实样本派生图（tft_03 + 6px 白边）上传成员 boy → **201** 完整报告（激素/2018-12-27/success/high=1，1.8s < NF-04 3s）；同图重传 → **200 duplicate**（item_id 不变、识别不重跑）；空白图 → **201 failed** + `fail_reason=no metric items recognized`（计数全 0、dateless）；时间线 `?member_id=7` → 2 卡片（dated 沉底正确）；全量 27 卡片；存量影像报告（bd39e0b2…）卡片计数与 SQL 聚合逐项一致。
- 409/override：SQL 置一项 confirmed → 重传 **409**（文案与 fixture 逐字一致）→ `override=true` → **201**（item_id 1631-35→1636-40 全换、review 全回 unreviewed、归属保持 7——请求带 member 9 也不夺走）；无主存量图（cbc_01）重传 → 200 + 补归属 7（自愈），随后 SQL 还原 member_id=NULL。
- 移动端封装以 `npx tsx` 直跑真栈：列成员 5 → boy 空时间线 → Web File 通道 multipart 上传 → 201 failed + fail_reason → 时间线 1 卡片 → 未知成员错误体 `member not found`。冒烟行（报告/OCR/图像）已 SQL 清理，库回 snapshot 计数 25/25/25/315、owned=0、members=5；`e2e.py verify` 全 ok（warn=RDW-CV 印刷串列，已知合法）。
- 栈收尾：hdd = 本分支镜像、CORS 缺省关（浏览器验收需临时 `HDD_SERVE_CORS_ORIGIN='*' docker compose -p deploy up -d hdd`）；两枚冒烟图像的 SeaweedFS 对象成孤儿（行已删、对象未删，惰性字节，重传幂等覆盖），快照可回滚。

**Decision / 有意偏离**

1. **归属写入语义（对 06 未明说的分岔的裁决）**：新建归档（201）归属本次上传成员；同图命中（200 duplicate）与 override 重跑**不改已有非空归属**（改归属是 PATCH 端点的职责，防重传静默夺走）；仅当报告**无主**（NULL）时补归属——覆盖「ingest 成功但归属写入失败被重试」的自愈窗口与存量 CLI 时代报告重新入档两个真实场景，且永不覆盖已有信任判定（核对决策 409 分支更早拦截）。
2. **上传响应 items 的 `ocr_lines: []` / `prev: null`**：契约形状先满足（数组非 null）；行链接实现属票 07 明确范围，prev 属 M3（07 票面自述「本票不实现」），不在本票冒充填充。
3. **超时提示口径**：选图等待超 10s 显示「识别用时较长，请确认后端与识别服务可达」，不主动中止请求（幂等重试安全，中止反而丢结果）。
4. **GET /reports 未知成员回 200 空信封**（而非 404）：该端点契约只声明 200（与 05 成员端点同惯例：未声明的错误码不新增语义）；非整数 member_id 400 与成员端点口径一致。
5. **「唯一新 seam = HTTP 边界」的落点**：新增 `pipeline.ReportStore` 窄端口与 HTTP 层内存 fake 沿用票 05 已被评审接受的双窄端口 + serve fake 模式（Q&A #1），不在 storage/pipeline 低层新增测试 seam。
6. **状态映射三处（文案表/颜色表/结果 switch）不合并**：输出物不同（标签、色值、含计数的结果句），合并会引入无收益的间接。

**评审（两轴 sub-agent：Standards / Spec）后收口**

- 已修：`'photo.jpg'` 重复字面量收敛常量；两处 sha hex 编码收口 `pipeline.ImageSha256Hex`；上传页加载错误补「重试」；`gap: 9` 笔误对齐 8；新增 `imagePartValue` 单测（3 例，Web/原生/缺省形态）。
- 保留（判定为 judgement call）：SQL 中枚举字面量（adapter 层惯例，与既有 'high'/'low'/'failed' 同法）；页面 loading/error 脚手架三页各自持有（05 既有页面模式，抽组件属未请求的重构）；报告/成员/状态模块边界与端口拆分（对齐 05 已评审结论）。409 英文原文透传：文案锚定冻结 fixture 且客户端「展示后端错误信息」为本票口径，友好 409 交互（override 确认）留给核对闭环票。

**复审收口（2026-10-16 维护者复审：parseUploadRequest 简化 / handleListReports 参数解析两问）**

- 已修（语义漏洞）：`parseUploadRequest` 取值 `r.FormValue → r.PostFormValue`（两处）。实测复核 Go 行为：multipart 请求下 `ParseMultipartForm` 把 body 字段**追加在 query 值之后**，`FormValue` 取 `[0]` 即 query 优先——`?member_id=999` 可覆盖 body 归属、query 可单独满足必填、`?member_id=` 空值可遮蔽合法 body 值、`?override=true` 可绕过 body 显式 `override=false`。契约字段面 = 表单 body，修后 query 完全不参与取值。新增回归 `TestServeUploadIgnoresURLQuery`（4 子例），红测验证：旧实现下四例全挂（404/201/400/201），修后全绿。
- 保留（判定为 judgement call）：`parseUploadRequest` 48 行结构不重构——9 个错误分支 ≈57% 行数是「严格校验 + 独立文案」的不可压缩成本，替代方案（helper 合并/流式 multipart.Reader/解码框架）净行数不降或引入新依赖；`handleListReports` 9 行解析保留——`Query().Get + ParseInt + 400` 即 stdlib 惯用，不抽 `optionalInt64Query`（单消费点、总行数净增；后续票 07/09/10 的 int 参数均为 path 形态）；未知查询参数不拒绝（`additionalProperties:false` 只挂 multipart body；GET 惯例忽略未知参数，全端点统一策略另议）；`GET /reports` 的 400 不写进冻结 yaml（与 `GET /members` 只声明 200 而实际有 500 同类，错误码声明应全端点统一修订）。对比证据与实测输出见会话记录。

**披露 / 留给维护者**

- **浏览器交互验收未跑**（本机无浏览器操作通道）：建议点检 时间线空态→上传（选图/换成员/等待提示/结果轻提示）→卡片徽标与「待核对 N 项」→failed 卡片原因→重复上传提示。Web 通道需临时开 CORS（命令见上）。
- 冒烟用的派生图/空白图在 worktree `data/`（gitignored）：`smoke-tft-bordered.jpeg`、`smoke-blank.jpeg`，可直接复用做浏览器验收。
