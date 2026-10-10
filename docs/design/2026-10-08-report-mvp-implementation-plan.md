# 检查报告管理 MVP · 实施计划

**版本：** 1.0
**日期：** 2026-10-08
**状态：** 已与维护者对齐共识，可启动 P0

> 本文自包含：MVP 的目标、约束、契约与决策全部收录于此。统一语言见 [CONTEXT.md](../../CONTEXT.md)，架构决策见 [docs/adr/](../adr/)——三者均为长存产物，不依赖任何工作稿。

---

## 1. 目标与非目标

### 1.1 目标（MVP）

1. 医院检查单拍照/导入，OCR 识别（RapidOCR ≥3.9.0 默认配置，onnxruntime CPU 推理）
2. 规则化后处理（不依赖 LLM）抽取结构化字段，优先支持**血常规**
3. 三层持久化并可追溯：原图（对象存储 SeaweedFS）、OCR 结果与结构化报告（PostgreSQL）
4. `hdd` CLI：全流程入库（ingest）、仅重跑后处理（reparse）、查询（list / show）
5. 识别服务 Docker 化 + HTTP 接口；全链路 `docker compose` 本地一键跑通

### 1.2 非目标（本期不做）

- 育儿问答 / 教程、实时体征监测与预警
- 多用户账号体系与云端同步（可预留）
- 以 LLM 为主的结构化抽取（P4 认知层增强）
- 原生 App 正式版（P4 后视产品需要）
- 血常规之外的报告类别（P3 起扩展尿常规、产检关键项）

---

## 2. 关键约束

| 编号 | 约束 |
|------|------|
| NF-01 隐私 | 默认本地处理与本地存储（全部本机容器，不外发原图）；样本库只收**脱敏**图像 |
| NF-02 可重复 | 模型与配置固定时，同一输入 OCR 结果可复现 |
| NF-03 可测试 | OCR 与后处理可分别用真实样本回归（golden 对比） |
| NF-04 性能 | Mac CPU 单张清晰报告 OCR 秒级可接受（<3s 量级，P1 实测记录） |
| NF-05 可演进 | OCR 模型与后处理规则可独立升级；保留 OCR 原始结果以支持重跑 |
| NF-06 契约稳定 | OCR 结果与报告输出有明确 JSON Schema（`schemas/`），Go / Python 双语共用 |

---

## 3. 架构与语言边界

> 决策全文见 [ADR-0002](../adr/0002-python-recognizer-go-data-service.md)。

```
┌──────────────────────────────┐          ┌────────────────────────────────┐
│ 识别服务（Python · Docker）    │   HTTP   │ 数据工程（Go · hdd CLI）        │
│ RapidOCR + 规则后处理          │◄─────────│ ingest 编排 / reparse / 查询    │
│ POST /report /reparse /ocr    │   JSON   │ （P4 起可常驻为 API 服务）      │
│ 镜像内调试 CLI（图片→OCR JSON）│          │ pgx + sqlc · minio-go          │
└──────────────────────────────┘          └───────┬────────────────────────┘
                                                  │
                                  ┌──────────────────────────────────┐
                                  │ docker compose: PostgreSQL + SeaweedFS │
                                  └──────────────────────────────────┘
```

**模块布局：**

```text
/                      # go.mod（module healthyduoduo）
  cmd/hdd/             # hdd 入口：ingest / reparse / list / show
  internal/…           # 识别服务客户端、PG / S3 存储（SeaweedFS）、编排逻辑
  recognizer/          # Python 识别服务（uv 包）：ocr.py、postprocess.py、api.py、__main__.py（调试 CLI）
  schemas/             # ocr_result.schema.json、report.schema.json（双语契约）
  samples/             # 脱敏样本 + expected/（golden 期望输出）
  deploy/              # recognizer Dockerfile、docker-compose.yml
  docs/                # 本计划、ADR
```

**技术栈：**

| 层 | 选型 |
|----|------|
| Python | uv + Python **3.12**（P3+ 表结构识别 TableStructureRec 要求 `<3.13`，ADR-0004；rapidocr 支持区间 3.8–3.13）；rapidocr ≥3.9.0 **默认配置**（Det/Rec: PP-OCRv6 small，Cls: PP-OCRv4 mobile）；onnxruntime（**显式依赖**，rapidocr 不自带）；wired-table-rec / lineless-table-rec（**表结构识别**，ONNX）；FastAPI + uvicorn；pydantic（契约源）；pytest + ruff |
| Go | Go **1.27**；pgx + sqlc；minio-go（S3 客户端库，服务端为 SeaweedFS）；goose（SQL 迁移）；标准库 net/http、httptest |
| 基础设施 | PostgreSQL、SeaweedFS（S3 API 网关）、Docker Compose（本机全部） |

> FastAPI / sqlc / goose 为计划推荐，P0 脚手架时若有更顺手替代可换，**契约与验收不变**。

---

## 4. 数据契约

### 4.1 OCR 结果（识别服务输出；`/ocr` 端点与 `/report` 内嵌）

```json
{
  "txts": ["血红蛋白", "128", "g/L"],
  "boxes": [[[x1,y1],[x2,y2],[x3,y3],[x4,y4]]],
  "scores": [0.99, 0.98, 0.97],
  "elapse": 1.23,
  "elapse_list": [0.4, 0.05, 0.78],
  "engine": "onnxruntime",
  "model_info": { "det": "PP-OCRv6_small", "cls": "PP-OCRv4_mobile", "rec": "PP-OCRv6_small" }
}
```

### 4.2 报告（识别服务输出；ID 由 Go 落库时赋予）

```json
{
  "report_type": "血常规",
  "report_date": "2026-10-01",
  "status": "success",
  "items": [
    {
      "name": "血红蛋白",
      "value": 128,
      "unit": "g/L",
      "ref_range": "115-150",
      "flag": "normal",
      "low_confidence": false,
      "raw_text": "血红蛋白 128 g/L 115-150"
    }
  ]
}
```

- `report_date` 可为 `null`（识别失败且未补录）
- `status ∈ {success, partial, failed}`：success = 类别 + 主要指标解析成功；partial = 有产出但有指标缺失 / 低置信 / 日期未解析；failed = 无法产出结构化结果（OCR 空、类别不识别），**仍留痕落库**
- `flag ∈ {normal, high, low, unknown}`：数值 vs 参考范围比较（支持 `115-150`、`<3.5` 格式）；无参考范围或非数值 → unknown
- `raw_text`：该指标项对应的原始 OCR 行文本，供人工核对
- 落库视图（PG）另含 `report_id` / `image_id` / `ocr_result_id` / `created_at` / `updated_at`

### 4.3 服务端点（P0 定稿于 `schemas/`）

| 端点 | 请求 | 响应 | 交付阶段 |
|------|------|------|----------|
| `GET /healthz` | — | `{"status":"ok"}` | P0（占位实现） |
| `POST /ocr` | multipart: image | OCR 结果 JSON | P1 |
| `POST /report` | multipart: image | `{ocr_result, report}` | P2 |
| `POST /reparse` | JSON: ocr_result | report | P2 |

### 4.4 存储模型

- **对象存储 bucket `raw`（SeaweedFS，S3 API）**：原图对象，键 = 图像内容 sha256
- **PostgreSQL 四表**：
  - `images`：id、sha256（**唯一**，去重依据）、object_key、original_filename、created_at
  - `ocr_results`：id、image_id（FK）、engine 输出全量（jsonb）、created_at——**每次识别追加一行**，`--force` 重跑时历史保留（NF-05 重跑对比）
  - `reports`：id、image_id（FK，每图一行）、ocr_result_id（指向本次生成所用的最新 OCR 结果）、report_type、report_date（nullable）、status、created_at、updated_at
  - `report_items`：id、report_id（FK）、name、value、unit、ref_range、flag、low_confidence、raw_text

---

## 5. 关键决策记录

| # | 决策 | 内容 | 理由 |
|---|------|------|------|
| 1 | 架构与语言 | ADR-0002：Python 识别服务（Docker+HTTP）+ Go 数据工程 | 认知试错闭环 vs 工程强度各就各位 |
| 2 | 分期 | P0–P4；P0–P2 任务级详案，P3–P4 目标级 | 骨架已收敛；近详远略 |
| 3 | 统一语言 | CONTEXT.md 六术语（检查单/图像/OCR 结果/报告/报告类别/指标项） | 消除「记录/报告」混用 |
| 4 | 枚举 | status / flag 见 4.2 | 失败可观测，不静默丢弃 |
| 5 | 去重 | 图像 sha256 命中 → 幂等返回既有报告 ID；`--force` 重跑并更新报告、OCR 历史追加 | CLI / 脚本重复执行天然安全 |
| 6 | 日期兜底 | `ingest --date` 补录；未解析且未补录 → `report_date=null` + `status=partial` | 文件时间不可靠 |
| 7 | 低置信 | 指标项任一组成行 score < 0.8 → 该项 low_confidence、报告 ≥ partial；阈值固定常量 | P2 样本实测后可微调一次 |
| 8 | 参考范围 | **一律从检查单抽取**；词典只做「项目名→规范名+已知单位集」（血常规约 25 项+别名/缩写），**不内置人群参考值** | 人群差异内置必错；单上印的才是核对依据 |
| 9 | CLI | `hdd`（Go 二进制）四命令；OCR-CLI 职责由镜像内 Python 调试 CLI 承担 | 编排/查询本在 Go；不起容器即可调试 OCR |
| 10 | 样本 | 脱敏样本提交 `samples/`；未脱敏原图永不入库；暂缺则先用仿真单 | NF-01/NF-03 |
| 11 | 验收形态 | 命令级（CLI + 期望输出）+ pytest golden + Go 契约 fixture | 「文档就绪」式弱验收不可执行 |
| 12 | CI | 双链（ruff+pytest；go vet+gofmt+go test）；Docker 构建/耗时验收本地手动 | 模型 wheel ~29MB，省 CI 额度 |
| 13 | 工具链 | uv + Python 3.12；Go 1.27 | P3+ 表结构识别（TableStructureRec）要求 `<3.13`（ADR-0004）；rapidocr 支持 3.8–3.13 |
| 14 | 存储 | MVP 即 compose 起 PostgreSQL + SeaweedFS（对象存储由 MinIO 改设，见 ADR-0003），**不经 SQLite 过渡** | 长线已明确，迁移成本 > 起步成本 |
| 15 | 流程 | main 只落文档；每阶段一个 worktree + 一次 commit（feat:/test: 前缀）；push 由维护者决定 | 仓库工作规范 |
| 16 | 依赖事实 | rapidocr ≥3.9.0 默认模型即 PP-OCRv6 small det/rec + PP-OCRv4 mobile cls（已核实）；onnxruntime 需显式安装；RapidOCR 无官方镜像，Dockerfile 自建 | 一级来源核实于 2026-10-08 |

---

## 6. 实施分期总览

| 阶段 | 目标一句话 | 主要交付物 | 验收一句话 |
|------|-----------|-----------|-----------|
| **P0 契约与样本** | 契约、样本、脚手架、空转全链路就绪 | schemas/、samples/、compose 骨架、CI 双链 | compose 起 pg+seaweedfs 健康；双链测试绿；样本 ≥5 张 |
| **P1 识别服务跑通** | 图 → OCR JSON 三通道一致 | run_ocr + 镜像 + /ocr + 调试 CLI + golden | 同图本地/容器/HTTP 输出一致且过 schema |
| **P2 全链路闭环** | 一图进、三层留存、一条报告出 | 后处理 v1（血常规）+ Go hdd + PG/SeaweedFS 落库 | hdd ingest→list/show 端到端演示通过 |
| **P3 增强** | 更多类别 + 统计 + 回归扩充 | 尿常规、产检关键项、趋势统计 | 样本集全量 golden 回归绿 |
| **P4 服务化扩展** | Go 常驻 API、LLM 抽取实验 | API 服务、批任务、LLM 增强 | 按产品需要启动时再定 |

---

## 7. P0 契约与样本

**目标：** JSON 契约、脱敏样本、双语脚手架、compose 空转全链路全部就绪，后续阶段在既定轨道上跑。

**为什么：** 契约（NF-06）是双语防漂移的地基——pydantic 与 Go struct 都从同一份 schema 出发；样本（NF-03）是回归测试的眼睛，没有它 P1/P2 的「识别质量」无从验收；空转全链路把基础设施风险（compose、PG、SeaweedFS）最早暴露。

**步骤：**

1. 脚手架：根 `go.mod` + `cmd/hdd`（四命令空实现，仅打印 not implemented）；`recognizer/` uv 包 + FastAPI `/healthz` 占位
2. `schemas/`：`ocr_result.schema.json`、`report.schema.json`（按 4.1/4.2 字段与枚举）；Python 侧示例校验测试，Go 侧 fixture 反序列化测试（用本文档 4.1/4.2 示例 JSON 作种子）
3. 样本：5–10 张**脱敏**血常规图像入 `samples/`（暂缺真实脱敏样本则先做 2–3 张仿真单，文件名前缀 `sim_`，P1 前补齐真实样本）；目录约定 `samples/<id>.jpg` + `samples/expected/ocr/<id>.json` + `samples/expected/report/<id>.json`
4. `deploy/`：`docker-compose.yml`（postgres + seaweedfs + recognizer 占位镜像）；`.gitignore`（.venv、__pycache__、dist 等）
5. CI：填入现有 `ci.yml` 占位——双链（uv run ruff+pytest；go vet+gofmt+go test）

**验收（全部可执行）：**

- [ ] `docker compose -f deploy/docker-compose.yml up -d postgres seaweedfs` 空转健康（在仓库根目录执行）：
  ```bash
  # PostgreSQL:容器内自检(宿主机无需安装 pg 客户端)
  docker compose -f deploy/docker-compose.yml exec -T postgres \
    pg_isready -U healthyduoduo -d healthyduoduo
  # 期望输出 accepting connections,退出码 0

  # SeaweedFS master(默认端口 9333)
  curl -sf http://localhost:9333/cluster/status      # 200;JSON 含 "IsLeader":true
  # SeaweedFS S3 API 网关(默认端口 8333)
  curl -sf http://localhost:8333/                    # 200;ListAllMyBucketsResult XML
  ```
  注：①宿主端口被占时用 compose 端口变量覆盖（`POSTGRES_PORT` / `SEAWEEDFS_MASTER_PORT` / `S3_API_PORT`，默认值见 `deploy/docker-compose.yml`）；②seaweedfs 单进程承载 master+volume+filer+S3，S3 网关需等 raft 收敛（冷启或重启约 15-20 秒），验证应轮询重试（如 `for i in 1 2 3 4 5; do curl -sf ... && break; sleep 2; done`），不要在容器刚起时单次 curl 判失败；③`telemetry` 上报日志与 `raft.Server: Not current leader` 为收敛期正常日志，以 curl 结果为准。
- [ ] `docker compose up -d recognizer` → `curl localhost:8000/healthz` 返回 `{"status":"ok"}`
- [ ] `uv run pytest`、`go test ./...` 全绿（含 schema 校验 / fixture 用例）
- [ ] `ls samples/*.jpg | wc -l` ≥ 5（或 ≥2 张 `sim_` 前缀，且计划中标注了补齐时间点）
- [ ] push / PR 后 GitHub Actions 双链绿（push 时机由维护者决定）

---

## 8. P1 识别服务跑通（OCR）

**目标：** 图 → OCR JSON 在三个通道（宿主机 uv、容器内调试 CLI、HTTP `/ocr`）结果一致，golden 回归建立。

**为什么：** OCR 是认知层地基；三通道一致性证明镜像内依赖与模型正确；golden 回归让此后任何模型 / 参数 / 依赖变更立刻可见（NF-02/NF-05）。

**步骤：**

1. `recognizer/ocr.py`：`run_ocr(image) -> OCR结果`（pydantic 建模 = 契约源）；rapidocr 默认配置 + onnxruntime 显式依赖；长边 > 2000px 先等比缩放（控制耗时与内存）
2. 调试 CLI：`python -m recognizer <image>` → stdout 输出 OCR JSON（容器内同款命令）
3. FastAPI：`POST /ocr`（multipart 上传）；uvicorn 起服务
4. Dockerfile（python:3.13-slim + rapidocr + onnxruntime；模型随 wheel 打包、离线可用）+ compose 接入 recognizer，挂载 `samples/`（只读）
5. golden：每张样本生成 `expected/ocr/<id>.json`；对比规则：**txts 行数相等、逐行忽略空白后编辑距离 ≤2**（跨 CPU 架构浮点求和顺序差异会翻转 rec 临界字符，阈值 2 为 arm64↔x86_64 实测容差——cbc 单字符、us_03 双字符如 `后6率`↔`6率`+全/半角逗号，模型/依赖退化时行内差异远超 2 不会被掩盖；同架构三通道输出仍逐字一致，由 CLI/HTTP 测试覆盖）、**scores 全行平均绝对误差 ≤0.03**（per-line 跨架构漂移实测 0.023~0.058 且不收敛——行级置信度是噪声主导维度，MAE 滤噪且统计级仍可检出真退化）、**boxes 容差 ±2px、elapse 只记录不比对**（耗时非确定量）

**验收(全部可执行;以下命令 2026-10-08 落地校准,逐字可粘贴,均在仓库根执行):**

- [ ] `cd recognizer && uv run pytest`:golden 全绿
- [ ] `docker compose -f deploy/docker-compose.yml up -d` → `curl -sF image=@samples/cbc_01.jpeg localhost:8000/ocr` 输出通过 schema 校验
- [ ] `docker compose -f deploy/docker-compose.yml exec -T recognizer python -m recognizer /samples/cbc_01.jpeg` 与宿主机 `uv run --project recognizer python -m recognizer samples/cbc_01.jpeg` 输出 txts **一致**(同图跨通道一致;样本挂载于容器 `/samples/`,扩展名以 samples/ 实际文件为准)
- [ ] 人工抽查 ≥2 张样本:txts 含清晰可读的指标名 / 数值行
- [ ] 记录单张耗时(NF-04 目标 <3s 量级),实测数字写入本文修订记录

---

## 9. P2 全链路闭环

**目标：** `hdd ingest` 一图进 → 原图入 SeaweedFS、OCR 结果与报告入 PG → `hdd list / show` 可查；`hdd reparse` 支持规则迭代后重跑历史。

**为什么：** MVP 的价值闭环（归档→回看→核对）从这一步成立；reparse 是规则迭代主路径——改词典 / 改正则后历史数据可批量刷新（NF-05），这是「不接 LLM 的规则抽取」策略可持续的前提。

**步骤：**

Python（认知层）：

1. `recognizer/postprocess.py`：文本清洗 → 行级解析（项目名+数值+单位+参考范围）→ 词典匹配 → 数值 / 单位校验 → 报告组装；血常规词典为**数据文件**（约 25 项 + 别名 / 英文缩写 → 规范名 + 已知单位集），非硬编码
2. 规则落地：低置信（任一行 score<0.8 → 项 low_confidence、报告 ≥partial）；日期识别失败 → null+partial；类别识别失败 → failed；参考范围只从检查单抽取（决策 #8）
3. `/report`、`/reparse` 端点（按 4.3）

Go（工程层）：

4. goose 迁移四表（按 4.4）；pgx + sqlc 访问层
5. minio-go（S3 客户端）：原图上传（键=sha256）；去重：sha256 命中 → 幂等返回既有报告 ID；`--force` → 重调 `/report` 并更新 reports、**追加** ocr_results
6. `cmd/hdd`：`ingest <image...> [--date YYYY-MM-DD] [--force]`、`reparse <report-id...>`、`list [--type] [--date]`、`show <report-id>`
7. 契约测试：Python golden 报告 JSON → Go fixture 反序列化 + schema 校验；httptest mock 识别服务单测编排 / 去重 / 补录逻辑

**验收（compose 全家桶端到端）：**

- [ ] `hdd ingest samples/cbc_01.jpg` → 输出报告 ID；`hdd show <id>` 含血红蛋白等指标项（value / unit / ref_range / flag 正确）
- [ ] `hdd list --type 血常规` 可查；`hdd list --date 2026-10-01` 可查
- [ ] 重复 `ingest` 同图 → **同一报告 ID，无新增行**（幂等）；`--force` → reports 更新、ocr_results 追加一行
- [ ] 无日期样本：`ingest --date 2026-10-01` 补录成功；不补录 → `report_date=null` 且 `status=partial`
- [ ] `hdd reparse <id>`：改一条词典规则后重跑，报告刷新（演示规则迭代闭环）
- [ ] psql 三层追溯断言：images → ocr_results → reports → report_items 外键贯通；report_items.raw_text 与原图肉眼可核对
- [ ] `go test ./...` + `uv run pytest` 全绿；golden 报告对比绿（含 flag / status 枚举、low_confidence、日期缺失场景）
- [ ] 演示口径：≥5 张样本连续 ingest + list（MVP 演示脚本）

---

## 10. P3 增强（目标级，启动时再细化）

**目标：** 报告类别扩展（尿常规、产检关键项）；简单趋势统计（同一指标跨报告对比）；样本集扩至 ≥20 张全量回归；坏例驱动的图像预处理开关（灰度 / 对比度 / 纠偏）。

**为什么：** 血常规验证了词典 / 规则框架的可扩展性，新类别是同一框架的复用；统计是「回看」场景的自然延伸；预处理按坏例再上，避免过早优化。

**候选演进（替代方案调研结论，2026-10-09）：** 新类别带来多样版式时,最近的免 LLM 路径是 PaddleOCR PP-Structure(V3) 表格识别（SLANet）——可在 P3 起**只替换 postprocess 的行带/配对这一层**（表格结构还原交给模型），词典数据文件与 report 契约不动;决定切换时出 ADR 并对照 golden 扩容样本验收。（开源直接可用的同题项目 MediParse / LabReport-Parser / MedClarify 等均依赖多模态 LLM/云 API,不符 NF-01。）

**演进结论（2026-10-10，ADR-0004 已落地）：** 原型对比后选定 **RapidAI TableStructureRec**（`wired_table_rec` + `lineless_table_rec`，ONNX，**两子模型择优**），**弃用 SLANet_plus**（须裁剪且精度落后）与 `table_cls` 单模型路由（误判率高）。表格类报告抽取前增 TSR 前端（退化回退启发式）；`mode=narrative`（超声）不变。OCR 结果契约增**可选** `table_structure`（`/report` 填充、随 `ocr_results` 落库、`/reparse` 复用）。为兼容两包的 `requires_python <3.13`，识别服务 Python 由 3.13 调整为 **3.12**（决策 #13 修订）。样本 golden 重基：lft_05 由 17 项恢复为 19 项；其余 23 张报告 golden 不变。

**P3 细化记录（2026-10-10 开工,按「无图像依赖 → 有图像依赖」分两波实施）：**

第一波（不依赖新类别样本,已落地）：

1. 多词典注册表：`recognizer/recognizer/` 下每个 `*_dict.yaml` 对应一个报告类别;分发为**内容优先(A1)**——命中项数 ≥ min_matched_items 者达标,达标者命中项数多者胜出、并列时标题(限标题区)命中者胜出、再并列取文件名序;无词典达标时才回退标题命中(避免正文偶发提及把报告抢到不相干类别);词典覆盖门禁 `test_dict_coverage` 参数化到注册表全部文件。
2. 趋势统计：`hdd trend <指标名> [--type <report-type>]`——同一指标项名（词典规范名）跨报告时序点,列 = date/type/value/unit/delta/flag/sha256（升序;delta 仅在相邻点单位一致时计算,unit 变了显示 "-"）。
3. 预处理开关（坏例驱动）：`run_ocr(image, preprocess=...)` 支持 `gray` / `autocontrast` / `deskew`,默认关闭——关闭时 OCR 输入与 P1 完全一致,golden 零影响;`/ocr` `/report` 可选 multipart 字段 `preprocess`,调试 CLI 与 `hdd ingest --preprocess` 同一开关透传;OCR 结果 / 报告契约 JSON 不变（NC-06 不动）。

第二波（2026-10-10 起实施,样本已到位）：
尿常规 UA / 血糖 GLU / 肝肾功能 LFT / 甲状腺功能 TFT / 超声 US 五类词典
（以真实 OCR 文本为证据,过门禁）→ golden 生成 → CLI `typeDisplay` 缩写 →
新类别端到端演示 + ≥20 张全量回归。

第二波进展（2026-10-10,维护者提供 19 张真实样本并确认脱敏合格、类别划分为
尿常规 UA / 血糖 GLU / 肝肾功能 LFT / 甲状腺功能 TFT / 超声 US 五类):

- 表格类落地(GLU/LFT/TFT):三个词典文件以 11 张样本 OCR 文本为证据落别名/单位
  (test_dict_coverage 门禁参数化通过);postprocess 抽取扩展——数据格按「上方最近表头带」
  分段(垂直堆叠小表互不串位)、本段表头带名称中心数 ≥2 才走左右半栏(整表全池配对);
  ROLE_HEADERS 增补 中文名称/No项/编号项目 印形。血常规 5 张 golden **零漂移**;
  glu_01 5/5、lft_02 17/17(success)、lft_04 13/13、lft_05 17 项恢复。
- 行 ua(定性为主)/us(叙述体):ua 词典 ~40 词条(定性形 value=None 留痕;镜检 /HP 与
  计数个/uL 版式拆分、机器/镜检双段 canonical 拆分);us 走 postprocess narrative 模式
  (别名 + 桥接符 + 后随数值行内配对,门禁按子串见证);词典覆盖门禁 narrative 分支。
- CLI `typeDisplay` 扩 5 缩写(域值不变);e2e(compose + hdd)验证:五类样本全量 ingest
  幂等、`list --type UA` 可查、`trend 血红蛋白/促甲状腺素` 时序+delta 正常;golden 样本
  合计 24 张(≥20) 全量回归绿。

验收口径：第一波/第二波以双链测试全绿为证（词典门禁全量参数化、注册表分发 / trend /
预处理 / 抽取分段化 / 叙述体均有单测）;粗验收已执行(e2e 见第二波进展行)——新类别各
≥3 张样本端到端可演示、`hdd list --type 尿常规` 可查、样本集 24 张全量 golden 回归绿。

**粗验收：** 新增类别各 ≥3 张样本端到端可演示；`hdd list --type 尿常规` 可查；样本集全量 golden 回归绿。

---

## 11. P4 展望（目标级）

**目标：** Go 常驻 API 服务（供未来 App 接入）、重跑批任务、多用户 / 云同步预留、LLM 抽取实验（认知层内替换规则抽取，不动工程层）。

**为什么：** 服务化与 LLM 均为长线方向，MVP 契约与边界已为其留好位置。

**粗验收：** 按产品需要启动时再定。

---

## 12. MVP 验收总表

- [ ] `docker compose up -d` 一键起全链路（pg + seaweedfs + recognizer）
- [ ] 图 → OCR JSON：`/ocr` 端点 + 容器内调试 CLI，输出符合 schema
- [ ] `hdd ingest`：原图 → 对象存储、OCR 结果 + 报告 → PG，三层可追溯
- [ ] 重复 ingest 幂等；`--force` 更新且 OCR 历史保留
- [ ] `hdd reparse` 仅重跑后处理（复用已存 OCR 结果）
- [ ] `hdd list / show` 按 ID / 类别 / 日期查询
- [ ] 血常规在 ≥5 张真实脱敏样本上可演示
- [ ] CI 双链绿（ruff + pytest；go vet + gofmt + go test）

---

## 13. 风险与对策

| 风险 | 对策 |
|------|------|
| 检查单版式多样，规则覆盖不足 | 锁定血常规起步；坏例入 samples 驱动规则 / 预处理迭代；LLM 留作 P4 认知层增强 |
| 拍照质量差致识别率低 | 长边 ≤2000px 控制；坏例增多后再加增强开关 |
| Mac CPU 耗时 / 内存压力 | P1 实测记录耗时；compose 限制服务 CPU / 内存 |
| 医疗数据敏感 | 全本机容器；samples 仅脱敏图像；未脱敏原图永不入库 |
| 双语契约漂移 | 同一份 schema 为源；Python golden JSON = Go fixture，CI 双向校验 |

---

## 14. 修订记录

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-10-08 | 与维护者三轮对齐后定稿（统一语言、架构、存储、CLI、枚举与阈值、验收形态） |
| 1.1 | 2026-10-08 | 对象存储由 MinIO 改为 SeaweedFS：官方 MinIO 镜像从 Docker Hub 撤下、社区镜像存维护风险，维护者决策（详见 ADR-0003）；bucket/键设计与 minio-go 客户端不变，契约与验收不变 |
| 1.2 | 2026-10-08 | golden 对比规则按跨平台噪声事实修正:txts 由「逐字相等」改为「行数相等 + 忽略空白的行内编辑距离 ≤1」;scores 由行级容差改为全行平均绝对误差 ≤0.03(CI 三轮实测 cbc_04 行级漂移 0.023→0.058 不收敛,置信度为噪声主导维度;MAE 滤除噪声、检出真退化)。同架构三通道逐字一致口径不变 |
| 1.3 | 2026-10-09 | /report 增可选 date 补录(决策 #6),/reparse 请求体 {ocr_result, date?};失败占位 report_type=unknown;pgx/v5+goose 库模式替代 sqlc(决策 #12);无日期样本为 cbc_02;P3 可换 PP-Structure(V3) 行带/配对(§10) |
| 1.4 | 2026-10-09 | 验收轮修订:① 对外身份统一为图像内容 sha256:images 主键=sha256(删除自增 id 与冗余 object_key 列,对象键=sha256),reports 每图一行、主键=图像 sha256(删除自增 id),ocr_history 仍以内部序号追加;hdd ingest 输出与 show/reparse/list 的入参均为图像 sha256(仅完整 64 位 hex;不支持前缀);② CLI 报告类别展示为英文缩写(血常规→CBC),--type 亦接受 CBC,域数据值不变;③ low_confidence 收敛至决策 #7 原义(仅 score<0.8;单位损耗保留原文),词典 RDW-SD 单位集修正为 fL |
| 1.5 | 2026-10-10 | P3 开工(§10 细化记录):第一波落地——① postprocess 多词典注册表(`*_dict.yaml` 按文件名序,分发=标题命中>命中项数>文件名序);② `hdd trend <指标名> [--type]` 趋势统计;③ OCR 预处理开关 gray/autocontrast/deskew 默认关闭(/ocr /report multipart `preprocess`、调试 CLI、hdd ingest `--preprocess` 透传;契约 JSON 不变),numpy/pillow 提升为显式依赖。第二波(尿常规/产检词典+真实样本+golden)待图像到位 |
| 1.6 | 2026-10-10 | P3 第二波落地(§10 细化记录):① 新类别 血糖 GLU / 肝肾功能 LFT / 甲状腺功能 TFT / 尿常规 UA / 超声 US 五词典(19 张真实脱敏样本,别名/单位以 OCR 文本为证据过门禁);② 抽取分段化:按「上方最近表头带」归属数据格,+单名称中心时整表全池配对;③ 尿常规定性值 value=None 留痕,机器/镜检双段 canonical 拆分;④ 超声 category.mode=narrative(别名+桥接符+数值行内配对),词典门禁补 narrative 子串见证;⑤ 粗验收已执行:五类 e2e ingest/list/trend 演示,24 张全量回归绿,血常规 golden 零漂移 |
| 1.7 | 2026-10-10 | golden txts 容差由 ≤1 放宽至 ≤2:CI(x86_64)实测 us_03 单行漂移 2(增字 + 全/半角标点),arm64↔x86_64 实测上限为 2;失败信息改为输出超容差行明细(§8-5 同步) |
| 1.8 | 2026-10-10 | review 修复①(分类解耦 A1):`_select_category` 由「标题命中者硬覆盖」改为**内容优先**——命中项数达标者胜出、标题只在达标者并列时(限标题区)裁决、无达标者才回退标题命中(含全页,保住 glu_03 这类「标题词实为项目名」样本);24 张 golden 零漂移,新增 3 条分发单测 |
| 1.9 | 2026-10-10 | review 修复②(表结构识别 TSR,ADR-0004):① 表格类报告抽取前增 TSR 前端(TableStructureRec `wired+lineless` 择优,ONNX;退化回退启发式),`mode=narrative` 不变;② OCR 结果契约增可选 `table_structure`(html/model/elapse),`/report` 填充并随 `ocr_results` 落库、`/reparse` 复用;schema/Go 契约同步;③ 识别服务 Python 3.13→**3.12**(两包 `requires_python <3.13`;决策 #13 修订);④ Dockerfile 构建期预下载模型(离线可用);⑤ `_match_name` 支持序号/星号任意组合前缀(多形态);⑥ golden 重基:lft_05 由 17 项恢复为 **19 项**,其余 23 张报告 golden 不变,新增 TSR 单测/网格抽取单测 |
