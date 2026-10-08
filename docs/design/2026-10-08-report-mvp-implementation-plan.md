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
3. 三层持久化并可追溯：原图（对象存储 MinIO）、OCR 结果与结构化报告（PostgreSQL）
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
                                  ┌───────────────▼───────────────┐
                                  │ docker compose: PostgreSQL + MinIO │
                                  └───────────────────────────────┘
```

**模块布局：**

```text
/                      # go.mod（module healthyduoduo）
  cmd/hdd/             # hdd 入口：ingest / reparse / list / show
  internal/…           # 识别服务客户端、PG / MinIO 存储、编排逻辑
  recognizer/          # Python 识别服务（uv 包）：ocr.py、postprocess.py、api.py、__main__.py（调试 CLI）
  schemas/             # ocr_result.schema.json、report.schema.json（双语契约）
  samples/             # 脱敏样本 + expected/（golden 期望输出）
  deploy/              # recognizer Dockerfile、docker-compose.yml
  docs/                # 本计划、ADR
```

**技术栈：**

| 层 | 选型 |
|----|------|
| Python | uv + Python **3.13**（rapidocr 支持区间 3.8–3.13，不用系统 3.14）；rapidocr ≥3.9.0 **默认配置**（Det/Rec: PP-OCRv6 small，Cls: PP-OCRv4 mobile）；onnxruntime（**显式依赖**，rapidocr 不自带）；FastAPI + uvicorn；pydantic（契约源）；pytest + ruff |
| Go | Go **1.27**；pgx + sqlc；minio-go；goose（SQL 迁移）；标准库 net/http、httptest |
| 基础设施 | PostgreSQL、MinIO、Docker Compose（本机全部） |

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

- **MinIO bucket `raw`**：原图对象，键 = 图像内容 sha256
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
| 13 | 工具链 | uv + Python 3.13；Go 1.27 | rapidocr 支持至 3.13；本机已装 |
| 14 | 存储 | MVP 即 compose 起 PostgreSQL + MinIO，**不经 SQLite 过渡** | 长线已明确，迁移成本 > 起步成本 |
| 15 | 流程 | main 只落文档；每阶段一个 worktree + 一次 commit（feat:/test: 前缀）；push 由维护者决定 | 仓库工作规范 |
| 16 | 依赖事实 | rapidocr ≥3.9.0 默认模型即 PP-OCRv6 small det/rec + PP-OCRv4 mobile cls（已核实）；onnxruntime 需显式安装；RapidOCR 无官方镜像，Dockerfile 自建 | 一级来源核实于 2026-10-08 |

---

## 6. 实施分期总览

| 阶段 | 目标一句话 | 主要交付物 | 验收一句话 |
|------|-----------|-----------|-----------|
| **P0 契约与样本** | 契约、样本、脚手架、空转全链路就绪 | schemas/、samples/、compose 骨架、CI 双链 | compose 起 pg+minio 健康；双链测试绿；样本 ≥5 张 |
| **P1 识别服务跑通** | 图 → OCR JSON 三通道一致 | run_ocr + 镜像 + /ocr + 调试 CLI + golden | 同图本地/容器/HTTP 输出一致且过 schema |
| **P2 全链路闭环** | 一图进、三层留存、一条报告出 | 后处理 v1（血常规）+ Go hdd + PG/MinIO 落库 | hdd ingest→list/show 端到端演示通过 |
| **P3 增强** | 更多类别 + 统计 + 回归扩充 | 尿常规、产检关键项、趋势统计 | 样本集全量 golden 回归绿 |
| **P4 服务化扩展** | Go 常驻 API、LLM 抽取实验 | API 服务、批任务、LLM 增强 | 按产品需要启动时再定 |

---

## 7. P0 契约与样本

**目标：** JSON 契约、脱敏样本、双语脚手架、compose 空转全链路全部就绪，后续阶段在既定轨道上跑。

**为什么：** 契约（NF-06）是双语防漂移的地基——pydantic 与 Go struct 都从同一份 schema 出发；样本（NF-03）是回归测试的眼睛，没有它 P1/P2 的「识别质量」无从验收；空转全链路把基础设施风险（compose、PG、MinIO）最早暴露。

**步骤：**

1. 脚手架：根 `go.mod` + `cmd/hdd`（四命令空实现，仅打印 not implemented）；`recognizer/` uv 包 + FastAPI `/healthz` 占位
2. `schemas/`：`ocr_result.schema.json`、`report.schema.json`（按 4.1/4.2 字段与枚举）；Python 侧示例校验测试，Go 侧 fixture 反序列化测试（用本文档 4.1/4.2 示例 JSON 作种子）
3. 样本：5–10 张**脱敏**血常规图像入 `samples/`（暂缺真实脱敏样本则先做 2–3 张仿真单，文件名前缀 `sim_`，P1 前补齐真实样本）；目录约定 `samples/<id>.jpg` + `samples/expected/ocr/<id>.json` + `samples/expected/report/<id>.json`
4. `deploy/`：`docker-compose.yml`（postgres + minio + recognizer 占位镜像）；`.gitignore`（.venv、__pycache__、dist 等）
5. CI：填入现有 `ci.yml` 占位——双链（uv run ruff+pytest；go vet+gofmt+go test）

**验收（全部可执行）：**

- [ ] `docker compose up -d postgres minio` → `pg_isready` 通过、MinIO 健康端点 200
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
5. golden：每张样本生成 `expected/ocr/<id>.json`；对比规则：**txts 逐字相等、scores 容差 ±0.02、boxes 容差 ±2px、elapse 只记录不比对**（耗时非确定量）

**验收（全部可执行）：**

- [ ] `uv run pytest`：golden 全绿
- [ ] `docker compose up -d` → `curl -F image=@samples/cbc_01.jpg localhost:8000/ocr` 输出通过 schema 校验
- [ ] `docker compose exec recognizer python -m recognizer samples/cbc_01.jpg` 与宿主机 `uv run python -m recognizer samples/cbc_01.jpg` 输出 txts **一致**（同图跨通道一致）
- [ ] 人工抽查 ≥2 张样本：txts 含清晰可读的指标名 / 数值行
- [ ] 记录单张耗时（NF-04 目标 <3s 量级），实测数字写入本文修订记录

---

## 9. P2 全链路闭环

**目标：** `hdd ingest` 一图进 → 原图入 MinIO、OCR 结果与报告入 PG → `hdd list / show` 可查；`hdd reparse` 支持规则迭代后重跑历史。

**为什么：** MVP 的价值闭环（归档→回看→核对）从这一步成立；reparse 是规则迭代主路径——改词典 / 改正则后历史数据可批量刷新（NF-05），这是「不接 LLM 的规则抽取」策略可持续的前提。

**步骤：**

Python（认知层）：

1. `recognizer/postprocess.py`：文本清洗 → 行级解析（项目名+数值+单位+参考范围）→ 词典匹配 → 数值 / 单位校验 → 报告组装；血常规词典为**数据文件**（约 25 项 + 别名 / 英文缩写 → 规范名 + 已知单位集），非硬编码
2. 规则落地：低置信（任一行 score<0.8 → 项 low_confidence、报告 ≥partial）；日期识别失败 → null+partial；类别识别失败 → failed；参考范围只从检查单抽取（决策 #8）
3. `/report`、`/reparse` 端点（按 4.3）

Go（工程层）：

4. goose 迁移四表（按 4.4）；pgx + sqlc 访问层
5. minio-go：原图上传（键=sha256）；去重：sha256 命中 → 幂等返回既有报告 ID；`--force` → 重调 `/report` 并更新 reports、**追加** ocr_results
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

**粗验收：** 新增类别各 ≥3 张样本端到端可演示；`hdd list --type 尿常规` 可查；样本集全量 golden 回归绿。

---

## 11. P4 展望（目标级）

**目标：** Go 常驻 API 服务（供未来 App 接入）、重跑批任务、多用户 / 云同步预留、LLM 抽取实验（认知层内替换规则抽取，不动工程层）。

**为什么：** 服务化与 LLM 均为长线方向，MVP 契约与边界已为其留好位置。

**粗验收：** 按产品需要启动时再定。

---

## 12. MVP 验收总表

- [ ] `docker compose up -d` 一键起全链路（pg + minio + recognizer）
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
