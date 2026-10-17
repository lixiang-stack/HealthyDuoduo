# 07: 报告详情纵切

**What to build:** 家长点开时间线卡片，看到一份完全可对照的报告详情：每个指标项的值画在参考范围条带上的位置、异常颜色一句话看懂、原始 OCR 行文就摆在旁边、原图随点随看；失败报告显示图像与原因。

**Blocked by:** 06（上传 + 时间线纵切）。

**Status:** resolved

- [x] 指标项列表展示：名称/数值/单位/参考范围/flag 染色语义（normal 绿 / high 红 / low 蓝·黄 / unknown 中性灰，色板实施期定）
- [x] 条带组件：参考范围条 + 当前值点位；标量 / 区间 / 带不等号 / null 四形态各有明确画法——M1 前 POC 定稿规则收进统一的解析模块
- [x] 参考范围解析客户端实现一份、集中放置；纯逻辑单测覆盖各形态与边界（解析失败不算崩，退化为纯文本展示）
- [x] 原始 OCR 行文面板；图像查看；低置信项高亮
- [x] failed 报告：图像 + 失败原因可见（无可核对项的氛围）
- [x] 详情 API 集成测试（items 外部句柄、ocr_lines、图像地址、报告级状态与计数）；fixtures 对账绿
- [x] `prev`（B2-lite 上一值）本票不实现（契约已冻结，M3 填充）

## Comments

**实现（分支 `report-detail`，worktree `.worktree/report-detail`，单 commit，push/PR 待用户）**

- **Go API**：`cmd/hdd/serve_reports.go` 新增 `GET /api/v1/reports/{report_id}`（详情 = 头部 + 指标项含 OCR 行链接）；新增 `cmd/hdd/serve_images.go` 的 `GET /api/v1/images/{sha256}`（对象存储直出，Content-Type 由 `http.DetectContentType` 内容嗅探——上传不落 content-type）。两端点对非 64 hex 路径参数一律 404（契约只声明 404；非法形态无法是任何已有报告/对象，与不存在同答）。
- **ocr_lines 读时推导（本票核心裁决）**：`internal/pipeline/ocrlines.go` 的 `LinkOCRLines` 把 `raw_text` 反推回组成它的 OCR 行（行下标 + 框坐标）。口径：识别端 `raw_text` = 各格 `text.strip()` 以两空格 join、格按 cx 升序；文本层面对纯 `%`/`↑ %` 等重复碎片有天然歧义，再以 cx 非降 + 同视觉行 y 聚带（取 y 展开最小者，平局取 y 偏差和）消解；重建失败返回 nil（退化为无行链接，不猜）。**无迁移、不动识别端**，CLI 存量报告同享；`reports.ocr_result_id` 本就在库（设计决策「OCR JSONB 含 Boxes 已持久化，无第二跳」）。
- **ReportStore 增 `OCRResultJSONByReport`**（reports.ocr_result_id → ocr_results.engine_output；缺失 nil,nil）；`ObjectStore` 增 `Get`（S3 实现把 minio 惰性读取的 NoSuchKey 映射 nil,nil；缺失即 404，对象存在性即图像存在性——内容寻址不查 DB，桶内惰性孤儿对象照常可回）。OCR 结果缺失/损坏按 500 fail-fast（ingest 原子写入 + FK，报告行在则 OCR 行必在；不静默降级为空链接）。**上传响应与详情共用同一投影**：票 06 的 `ocr_lines:[]` 占位自此兑现为真实链接（同一 ReportDetail 形状不留半填状态）。
- **App**：`src/app/report/[report_id].tsx` 详情页（时间线卡片 `Link asChild` 点入；头部复用 `ReportCard`）+ `src/reports/refRange.ts` 条带解析（唯一一份，POC 规则见下）+ `src/reports/components.tsx` 的 `MetricItem`/`MetricStrip`/`ReportBadges`（`ReportCard` 徽标抽共用）+ `src/api/reports.ts` 的 `getReport`/`reportImageUrl`。指标项卡：名称、值/单位(flag 色)+ flag 文案、参考范围原文、条带、`原文:` 面板；`low_confidence` 项描边高亮 + 「低置信」标签。图像查看：failed 默认展开、其余「查看检查单原图」点按展开/收起；failed 头下给无核对项提示与重新拍照入口（原报告留痕）。指标条带色用中性灰，只有值点用 flag 色。
- **测试证据**：Go——`ocrlines_test.go` 9 例（多格行重建、重复碎片按 y 聚带消歧、单格、纯 `%` 碎片、cx 非降拒绝、非 `  ` 边界拒绝、退化输入、不可重建、重复行封顶终止）；`serve_reports_test.go` 详情用例（完整形状：外部句柄/核对状态/行链接/prev null/聚合计数；图像地址字节逐位；未知与非法形态 404）+ failed 详情（`items:[]`、计数 0、fail_reason 透传）+ 上传/重复响应含链接；`serve_images_test.go` 真实样本图字节直通 + `image/jpeg` 嗅探 + 四类 404；`store_reports_test.go` gated 真库冒烟补 OCR 回读（一次性 PG 跑过）。`go test ./...`、`gofmt`、`go vet` 全绿；fixtures 对账（`internal/httpapi`）未动、照常绿。mobile——jest 8 套件 48 用例（新增 refRange 17 + flagLabel 1；既有 30 全绿）、typecheck、codegen 同步（契约零改动）、`expo export --platform web` 打包成功。
- **共享栈真栈 e2e**（快照 `data/e2e-snapshots/pre-07-report-detail-20261017-000608.dump`，33083B）：按本 worktree 重建 hdd（`docker compose -p deploy build hdd && up -d`；无新迁移，recognizer 代码未动故免 refresh）→ 脚本逐份核对 5 份真实报告：**89/89 指标项 ocr_lines 齐全**，每项链接行文本按原序 join 后与 `raw_text` 完全一致、每行框与库中 OCR box 逐点一致、**每份图像端点字节 sha256 == report_id**、bad=0；`e2e.py verify` 全 ok（warn=RDW-CV 印刷串列，已知合法）。

**Decision / 有意偏离**

1. **条带画法 POC 定稿（2026-10-17 与维护者确认）**：参考范围——区间画带（窗口 = 锚点外扩 8%）、单侧 `<b`/`≤b` 从左缘画到 b（`>`/`≥` 对称）、标量画细基准线、解析失败/null 整条不画（退化纯文本）；当前值——标量实心点、区间跨度条不画点（spec「区间无点」）、不等号空心点置于条端外（spec「点置于条端外」；单侧参考无外侧空间时夹边缘）、null/文本不画点。区间降序（如 80-70）解析期归一并序；性别前缀（男:0-15）剥离后解析、显示仍原文。
2. **详情/图像非 hex 路径 → 404 而非 400**（判为 judgement call）：契约该两端点只声明 200/404；report_id/sha256 是内容身份，非法形态与「不存在」对客户端是同一件事。`GET /reports` 非整数 member_id 的 400 系票 06 既有口径，不回改。
3. **ocr_lines 读时推导而非入库**：写时推导需迁移 + 存量回填，且识别端契约（冻结）本就不产出行号，写时同样只能文本匹配——读时零迁移、重跑/修正自动一致（票 09 核对响应可复用同一函数）。
4. **上传响应同步填充 ocr_lines**：同一 ReportDetail 契约形状不应一半填一半空；客户端「单响应刷新整屏」（票 09）也会依赖。
5. **`ReportCard` 在详情页复用**（评审收口）：详情头部与时间线卡片本就同字段同版式，直接复用组件而非复制一份标记（若 M1 详情头部要长出改归属/删除入口，属票 10 的改动点）。
6. **flag 色板落地**：low = `#1d4ed8`（独立深蓝，与交互色 accent 区分）；进 `components/theme.ts` 单处定义。`unknown` 中性灰不吓人。

**回归修复（推送后维护者实测发现）**

- **原生上传失败：`Unsupported FormDataPart implementation`**。根因：Expo SDK 57 原生全局 `fetch` 是 WinterCG 实现（`expo/src/winter`），FormData part 只接受 string | Blob | 带 `bytes()` 的对象；票 06 的原生装包 `{uri,name,type}` 是旧 RN fetch 的私有形态，在该 fetch 下必被拒（既有单测只覆盖取值形态，未覆盖 SDK 57 的转换契约）。已用 Expo 真实 `convertFormDataAsync` 复现：旧 part 抛同一文案，`bytes()` part 正常转换（带 filename/content-type）。修复：原生改 append `expo-file-system` 的 `File`（implements Blob + `bytes()`），Web 仍直传 DOM File；新增依赖 `expo-file-system@~57.0.7`（Expo Go 内置）。单测改为断言原生 part 保留 `uri` 且具备 `bytes()`（即 Expo fetch 的判据）、`appendImagePart` 装入 `image` 字段。

**披露 / 留给维护者**

- **浏览器交互验收未跑**（本机无浏览器操作通道）：建议点检 时间线卡片点按 → 详情（条带/染色/原文/低置信高亮）→ 图像展开 → failed 卡点入（图像默认展开 + 原因 + 重新拍照入口）。Web 通道需临时开 CORS：`HDD_SERVE_CORS_ORIGIN='*' docker compose -p deploy -f deploy/docker-compose.yml up -d hdd`；共享栈 hdd 现为**本分支镜像**（供验收；合并后按 main 重建即可）。
- 详情页**无核对按钮**（属票 09）；`ocr_lines` 的框坐标尚无客户端消费者（票 09 的缩放定位消费；本票 UI 只到行文面板与图像查看）。
- 核对入口文案「重新拍照上传」目前指向通用上传页（无预选成员/来源提示）；如要「带着原报告上下文重拍」属票 09/10 范畴。
