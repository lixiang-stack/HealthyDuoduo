# HealthyDuoduo 医学检查项目解读 RAG 需求文档

**状态**：v1 需求基线（语料清单待用户提供）
**日期**：2026-11-06
**关联**：
- 架构决策：`docs/adr/0018-rag-explanation-layer.md`
- 现有分析设计：`.scratch/llm-analysis/spec.md`
- UI 原型：`.scratch/rag-liver/prototype/prototype.html`、`shots/00-contact-sheet.png`
- 领域语言：`CONTEXT.md`
- 部署边界：`docs/adr/0006-lan-first-no-auth.md`

---

## 0. 决策速览

| 主题 | 决策 |
| --- | --- |
| 主目标 | 覆盖（A）+ 降低新增条目的边际成本（C）；深度（B）作为后续收益；不做开放问答（D） |
| 长期方向 | 删除本地 YAML，解释层由 RAG 知识库承担 |
| 模型角色 | LLM 基于检索内容生成四段式；服务端校验；不再要求四段正文逐字来自 YAML |
| 检索 | 本地 BGE-M3 独立 Python 服务 + pgvector；dense + sparse/BM25 混合 |
| 引用 | 每条 claim 带 `quote + chunk_id`；服务端做逐字子串校验 |
| 运行时校验 | 规则 only；entailment 只在评估期做 |
| 内容边界 | 允许病因、进一步检查、生活建议、跟踪、红黄绿灯、一般诊断标准、一般复查间隔；禁止个体诊断、治疗/药物/剂量、预后、个体化复查医嘱、v1 儿科输出 |
| v1 范围 | 肝功 Panel + 单指标入口；成人 18+；5 类核心模式 |
| 语料 | 10–20 份，用户提供清单；人工下载；全文本地私有；manifest 入 git |
| 语料切块 | 整段保留，不按内容类型拆分、不丢弃治疗段（C0）；仅按标题/段落切块 |
| 评估 | 40 个成人肝功 case × 3 次；硬 gate 见 §11 |
| 上线 | RAG-only + kill switch；不做生产灰度；不兼容旧数据，可删除存量分析 |
| 配额 | 12k input / 2.5k output / 每天 20 次 |
| UI | 方案 B：内联引用编号 + 来源列表 |

---

## 1. 背景与问题

当前解释层依赖 `internal/analysis/knowledge/*.yaml` 硬编码条目：

- 只有 2 条肝功条目（`liver-hepatocellular-mild`、`liver-low-protein-albumin`），覆盖面窄。
- 条目内容单薄，LLM 只写 `bridge_text`，四段医学正文逐字取自条目；输出缺乏深度与系统性。
- 新增一个模式需要手写 YAML + 人工 review，边际成本高。

真正瓶颈不是「检索不到」，而是**审核过的医学内容少**。RAG 不会自动产生可信内容；它把工作单元从「按模式手写条目」变成「按原文切知识块 + 检索组合」，从而降低新增覆盖的边际成本，但把瓶颈转移到**语料质量、检索质量和生成校验**。

产品边界原本是「不做诊断与治疗建议，解释必须可溯源、可降级、可审计」。本需求把边界调整为：

> 展示指南知识与来源；在证据完整时判断本次数据是否符合某指南标准；不做个体疾病诊断、治疗、用药、剂量或预后判断。

这是一次实质性的产品定位变化，必须同步修改 README、`CONTEXT.md`、首次使用确认和结果页声明。

---

## 2. 目标与非目标

### 2.1 目标

1. **覆盖**：v1 让肝功 5 类核心模式在成人分析中得到有依据的解释，不再依赖逐条 YAML。
2. **深度**：四段式由 LLM 基于多段权威原文组织，允许跨段综合，不再只是单条条目的格式化。
3. **降低边际成本**：新增一个来源只需入语料、标 metadata、重建索引，不新增 YAML 条目。
4. **可溯源**：每条医学结论必须绑定 `chunk_id + 逐字 quote`；引用由服务端渲染，模型不能自行编造来源。
5. **可验证**：接入产品前，用固定评估集独立验证检索、忠实度、边界和有用性。
6. **可降级**：RAG 失败、索引缺失或被关闭时，事实层与框架层始终可用，解释层整层省略。

### 2.2 非目标

- 不做个体疾病诊断结论（「你有 X 病」）；只允许「符合/不符合某指南标准」，且必须证据完整。
- 不做治疗、药物、剂量、预后判断。
- 不做 v1 儿科解释输出；儿科语料入库但不参与检索。
- 不做肝功以外的 Panel（后续迭代）。
- 不做运行时在线抓取、自动爬虫。
- 不做知识图谱、微调、本地 LLM。
- 不做语料管理后台；语料与 manifest 走 git + 离线构建。
- 不做生产灰度；项目未上线，直接切换。
- 不兼容旧分析数据；可删除存量分析、旧缓存和旧 YAML 代码路径。

---

## 3. 产品边界

### 3.1 允许 / 禁止

| 内容 | 允许 | 条件 |
| --- | --- | --- |
| 指南层病因/机制 | ✅ | 引用 chunk + quote |
| 个体可能原因 | ✅ | 写成「可能与…有关」，不得写成「你有…」；引用 chunk |
| 进一步检查 | ✅ | 引用 chunk；不得给具体时间/剂量 |
| 生活建议 | ✅ | 引用 chunk；不得含药物、剂量、治疗方案 |
| 长期跟踪 | ✅ | 引用 chunk |
| 红黄绿灯/就医提示 | ✅ | 仅当 chunk 原文明确写出 |
| 一般诊断标准 | ✅ | 写成「指南中，X 的诊断需要…」；引用 chunk |
| 一般复查间隔 | ✅ | 写成「指南建议 3–6 个月复查」；引用 chunk |
| 判断本次是否符合某标准 | ⚠️ | 仅当该标准要求的全部条件都能在本次证据中找到并逐条引用；缺一项只能写「缺少 X，无法判断」 |
| 个体疾病诊断 | ❌ | 如「你有脂肪肝」「这些结果说明肝脏有脂肪堆积」 |
| 治疗 / 药物 / 剂量 | ❌ | 如「可以吃水飞蓟素，每天 2 次」 |
| 预后 | ❌ | 如「会发展成肝硬化」 |
| 个体化复查医嘱 | ❌ | 如「你应该 3 个月后复查」；只能说「指南建议 3–6 个月」 |
| v1 儿科解释 | ❌ | 儿童成员只输出事实层 + 框架层 + 固定提示 |

### 3.2 措辞规则

- 允许「可能」「提示」「考虑」「建议」「应该」等词，但必须绑定引用；不再沿用当前 `containsBannedInference` 的一刀切禁词。
- 一般知识用「指南中 / 指南建议 / 通常 / 常见于」。
- 个体判断用「本次结果…」；不得用「你就是 / 说明你有 / 可以诊断为」。
- 诊断标准必须带 `standard_check`：
  - `complete=true` 才能写「符合」；`missing` 必须为空。
  - `complete=false` 必须写「缺少…，无法判断」，不得写「符合」。
- 服务端不知道「医学上是否真的完整」；它只校验结构一致性。医学正确性靠评估集和人工复核。

### 3.3 必须同步修改的用户文案

- README：「不提供诊断、治疗或复查建议」→「展示指南知识与来源；证据完整时判断是否符合标准；不做个体诊断、治疗、用药或预后判断」。
- `CONTEXT.md`：新增「知识块」「检索」「claim」「引文」「语料版本」；更新「分析」定义中对解释层的描述。
- 首次使用确认：说明会把检验数值、年龄段、诊断/处理原文和公开指南片段发送到云端模型；不包含姓名、生日、机构、原图。
- 结果页固定声明：「内容来自指南原文，不是诊断。」

---

## 4. v1 范围

### 4.1 Panel 与入口

- Panel：`liver`。
- 入口：Panel 详情 + 单指标；`clinical_note` 不做解释层。
- 仅在有至少 1 个异常指标（`high` / `low` / `abnormal`）时触发解释层；全正常不调用 LLM。
- 单指标入口：查询构造只用该 analyte + 框架标签；不扩展到整个 Panel 的解释。

### 4.2 人群

- v1 只支持成人：成员年龄 ≥ 18 岁。
- 儿童成员（< 18 岁）：
  - 事实层、框架层照常输出；
  - 解释层不调用 RAG/LLM；
  - 显示固定提示：「儿童肝功解读需要结合年龄和儿科医生意见，本版本暂不提供。」
- 儿科语料可以入库，但 `age_group=pediatric`，v1 检索强制过滤 `age_group=adult`；不得被成人 case 命中。

### 4.3 目标模式

v1 目标覆盖 5 类：

1. 肝细胞型轻度/中度（ALT/AST 升高，ALP 不明显升高，R ≥ 5）
2. 胆汁淤积型（ALP 升高为主，R ≤ 2）
3. 混合型（ALT/AST 与 ALP 都升高，2 < R < 5）
4. 单纯胆红素升高（ALT/AST/ALP 基本正常）
5. 低白蛋白/总蛋白

补充规则：

- AST/ALT 比值、>15×ULN 重度：有对应语料才出解释；没有则只输出框架层 + 固定「请尽快让医生评估」提示。
- 最终模式清单以用户提供的语料能覆盖的范围为准；语料不足的模式允许只输出框架层。

### 4.4 解释层输出

- 四段式：可能原因、进一步检查、生活建议、长期跟踪。
- 另设「指南标准」卡片：用于一般诊断标准 + `standard_check`（是否符合 / 缺少什么）。
- 空段省略；`causes` 和 `checks` 至少一段有内容，否则视为解释失败。
- 每条 claim 是列表项，不是整段散文；服务端按 section 聚合渲染。
- 输出上限：12k input / 2.5k output；每天 20 次。

---

## 5. 总体架构

```text
POST /api/v1/analyses
  ↓
事实层 + 框架层（不变）
  ↓
解释层开关
  ├─ HDD_RAG_ENABLED=false → 省略解释层（reason=rag_disabled）
  └─ 开启
       ↓
     查询构造：panel + age_group=adult + analytes + flags + framework labels + required sections
       ↓
     RAG 检索（pgvector）
       - dense：BGE-M3 1024 维 cosine
       - sparse/BM25：BGE-M3 sparse 或 Go BM25
       - metadata 过滤：panel / age_group / sections
       - RRF 融合 → top 8–12 chunk
       ↓
     LLM 生成（flash 档）
       - 输入：事实层 + 框架层 + 命中 chunk 全文 + metadata
       - 输出：claims JSON（text / quote / chunk_id / source_id / evidence_ids / standard_check）
       ↓
     服务端校验（规则 only）
       - 结构、chunk 存在、quote 子串、数字/方向、边界、standard_check 一致性
       - 失败 claim 丢弃；全失败则省略解释层
       ↓
     服务端组装四段 + 来源列表
       ↓
     快照（rag 元数据）+ sources + 缓存
       ↓
     响应
```

### 5.1 分层职责

| 层 | 职责 | 不做什么 |
| --- | --- | --- |
| 事实层 | 数据、病历原文、未纳入原因 | 不做医学判断 |
| 框架层 | R 值、分级、AST/ALT、BMI 等确定性计算 | 不解释病因 |
| 检索层 | 按 metadata + 向量/关键词召回 chunk | 不生成文字 |
| 生成层 | 基于 chunk 写 claims | 不自行写来源、不改写数字 |
| 校验层 | 结构、引用、数字、边界 | 不保证语义正确 |
| 组装层 | 按 section 聚合 claims、渲染来源 | 不改写 claim 文本 |

### 5.2 代码落点

- 领域：`internal/analysis`
  - `Claim` / `Explanation` / `KnowledgeChunk` / `RetrievedChunk` / `StandardCheck`
  - 查询构造、prompt 组装、claim 校验、四段聚合
- 基础设施：`internal/rag`
  - pgvector 检索适配器
  - BGE-M3 HTTP 客户端
  - 离线 ingestion 的 Go/Python 接口
- 离线工具：`rag/ingest.py`（Python，读 manifest、抽文本、切块、调用 embed 服务、写库）
- 服务：`rag-embed`（独立 Python FastAPI 容器，BGE-M3）
- HTTP：`internal/httpapi` + `schemas/openapi.yaml`
- App：`apps/mobile/src/analysis/` + `api/analyses.ts`

---

## 6. 语料与离线构建

### 6.1 语料来源

- 10–20 份，肝功优先，中英混合；由用户提供清单（文档名/链接/年份/语言/是否含儿科/是否含治疗段）。
- 人工下载 PDF/HTML；不写爬虫。
- 权威等级：`high` / `medium`；记录 `authority`、`year`、`org`、`url`。
- 全文存本地私有目录（`corpus/private/`，gitignored）；git 只存 manifest + chunk 元数据 + 短引文。
- 索引产物不入 git；按 `index_version` 归档在私有存储。

### 6.2 收录规则

- 接受全文入库，包括诊断标准、治疗、药物、剂量、预后、具体复查间隔、儿科内容。
- **不按内容类型拆分，不整段丢弃治疗段**（C0）。
- 仅按标题层级 + 段落边界切块；一个 chunk 尽量保持医学逻辑完整。
- 每个 chunk 必须标：
  - `source_id`
  - `sections`（多选：`causes` / `checks` / `advice` / `tracking` / `standard`）
  - `panels`（如 `liver`）
  - `analytes`（如 `alanine_aminotransferase`）
  - `patterns`（如 `hepatocellular` / `cholestatic` / `mixed` / `isolated_bilirubin` / `low_albumin`）
  - `age_group`（`adult` / `pediatric` / `all`）
  - `language`（`zh` / `en`）
  - `authority`（`high` / `medium`）
  - `year`
- 不标 `content_type`，不标 `usable_for_output`；治疗段与允许段可能在同一 chunk。这是已接受的风险（见 §12）。

### 6.3 切块

- 目标 200–500 token；超长段落按句号/分号切，保留标题路径。
- 不重叠；同一标题下的连续段落可合并到一个 chunk，但不超过上限。
- `chunk_id` 稳定：`{source_id}-{heading_slug}-{index}`；重建时保持稳定，内容变化用 `content_hash` 识别。
- 每个 chunk 记录 `content_hash`、`corpus_version`、`chunk_index`。

### 6.4 离线构建流程

1. 读取 `corpus/manifest.yaml`。
2. 从 `corpus/private/` 读取 PDF/HTML，抽取正文与标题结构。
3. 切块、补 metadata。
4. 调用 `rag-embed` `/embed`，得到 dense + sparse 向量。
5. 写入 Postgres：
   - `rag_documents`
   - `rag_chunks`
   - `rag_index_versions`
6. 生成新 `index_version`；激活由 `HDD_RAG_INDEX_VERSION` 指定。
7. 构建失败不阻塞 `hdd serve`；索引缺失时解释层不可用（reason=`rag_index_unavailable`）。

### 6.5 Manifest 模板

```yaml
version: "2026-11-06.1"
documents:
  - id: aasld-elevated-liver-enzymes
    title: "How to approach elevated liver enzymes"
    org: "American Association for the Study of Liver Diseases"
    year: 2024
    authority: high
    language: en
    url: "https://www.aasld.org/..."
    file: "corpus/private/aasld-elevated-liver-enzymes.pdf"
    age_group: adult
    license_note: "个人家庭使用；不提交全文到 git"
  - id: acg-2016-abnormal-liver-chemistries
    title: "Evaluation of Abnormal Liver Chemistries"
    org: "American College of Gastroenterology"
    year: 2016
    authority: high
    language: en
    url: "https://doi.org/10.1038/ajg.2016.517"
    file: "corpus/private/acg-2016.pdf"
    age_group: adult
    license_note: "个人家庭使用；不提交全文到 git"
```

**语料清单待用户提供；在提供前，§6 的文档 ID、模式覆盖和评估 case 只能先用模板占位。**

---

## 7. 检索

### 7.1 查询构造

输入：`Request` + `Facts` + `Framework`。

```json
{
  "panel": "liver",
  "age_group": "adult",
  "sex": "male",
  "analytes": ["alanine_aminotransferase", "aspartate_aminotransferase"],
  "flags": {"alanine_aminotransferase": "high", "aspartate_aminotransferase": "high"},
  "framework_labels": ["liver_r_value:肝细胞型", "liver_transaminase_grade:轻度"],
  "required_sections": ["causes", "checks"],
  "optional_sections": ["advice", "tracking", "standard"]
}
```

查询文本： analyte 中英文名 + 模式标签 + section 关键词。不要拼接姓名、生日、报告 ID。

### 7.2 过滤

- `age_group = adult` 强制；`pediatric` 永不进入 v1 检索。
- `panels` 包含 `liver`。
- `sections` 与 `required_sections` 有交集。
- `analytes` / `patterns` 作为加分项，不作为硬过滤（避免语料标注不全导致零召回）。

### 7.3 排序

- dense：BGE-M3 1024 维 cosine。
- sparse：BGE-M3 sparse 权重；若 embed 服务不返回 sparse，则用 Go 侧 BM25。
- RRF 融合 dense/sparse；取 top 8–12。
- v1 不引入额外 rerank 模型；LLM 在生成时隐式做最终选择。
- 父章节扩展默认关闭：C0 下扩大上下文会放大治疗段泄漏风险。后续如开启，必须重新评估。

### 7.4 存储

- Postgres 镜像改为 `pgvector/pgvector:pg17`（或安装 pgvector 扩展的同版本镜像）。
- 迁移 `0021_rag.sql`：

```sql
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE rag_documents (
  id            TEXT PRIMARY KEY,
  title         TEXT NOT NULL,
  org           TEXT NOT NULL,
  year          INTEGER,
  authority     TEXT NOT NULL CHECK (authority IN ('high','medium')),
  language      TEXT NOT NULL CHECK (language IN ('zh','en')),
  url           TEXT,
  license_note  TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE rag_chunks (
  id             TEXT PRIMARY KEY,
  document_id    TEXT NOT NULL REFERENCES rag_documents(id) ON DELETE CASCADE,
  chunk_index    INTEGER NOT NULL,
  heading_path   TEXT,
  text           TEXT NOT NULL,
  token_count    INTEGER NOT NULL,
  sections       TEXT[] NOT NULL,
  panels         TEXT[] NOT NULL,
  analytes       TEXT[] NOT NULL DEFAULT '{}',
  patterns       TEXT[] NOT NULL DEFAULT '{}',
  age_group      TEXT NOT NULL CHECK (age_group IN ('adult','pediatric','all')),
  language       TEXT NOT NULL,
  authority      TEXT NOT NULL,
  year           INTEGER,
  content_hash   TEXT NOT NULL,
  corpus_version TEXT NOT NULL,
  embedding      vector(1024),
  sparse        sparsevec,
  UNIQUE (document_id, chunk_index, corpus_version)
);

CREATE TABLE rag_index_versions (
  version          TEXT PRIMARY KEY,
  corpus_hash      TEXT NOT NULL,
  embedding_model  TEXT NOT NULL,
  chunker_version  TEXT NOT NULL,
  status           TEXT NOT NULL CHECK (status IN ('building','ready','failed')),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX rag_chunks_embedding_idx ON rag_chunks USING hnsw (embedding vector_cosine_ops);
CREATE INDEX rag_chunks_panels_idx ON rag_chunks USING gin (panels);
CREATE INDEX rag_chunks_sections_idx ON rag_chunks USING gin (sections);
```

- <10k chunk 时 exact scan 也可；HNSW 为后续规模预留。
- `sparsevec` 需要 pgvector ≥ 0.7；若部署镜像不支持，改为 `sparse JSONB`，在 Go 侧计算稀疏分数。

### 7.5 Embedding 服务

- 独立 Python 容器 `rag-embed`（FastAPI），模型 `BAAI/bge-m3`。
- 端点：
  - `POST /embed`：输入 `{texts: []}`，返回 `{dense: [[1024]], sparse: [{token_id: weight}]}`。
  - `GET /health`：模型已加载。
- 模型缓存挂载 volume；CPU 推理，无 GPU。
- 配置：`HDD_RAG_EMBED_URL`；超时 10s。
- 失败：检索不可用 → 解释层省略（reason=`rag_unavailable`），事实层/框架层不受影响。

---

## 8. 生成

### 8.1 模型与配置

| 用途 | 模型档位 | 配置 |
| --- | --- | --- |
| 生成 | DeepSeek flash | `HDD_LLM_MODEL`（当前默认 `deepseek-chat`，部署时按可用 flash 档填写） |
| 评估 judge | DeepSeek pro/reasoner | 新增 `HDD_LLM_JUDGE_MODEL`（如 `deepseek-reasoner`） |
| Embedding | BGE-M3 | `rag-embed` 服务 |

- 两者都保持可配置；不在代码里写死模型 ID。
- `temperature=0`，`response_format=json_object`。
- 生成超时 30s；失败重试 1 次。
- 同一供应商的 judge 有自评偏差；评估报告必须记录这一点。

### 8.2 输出契约

```json
{
  "sections": [
    {
      "kind": "causes",
      "claims": [
        {
          "text": "指南中，ALT/AST 升高而 ALP 不明显升高时，通常先归为肝细胞型损伤。",
          "quote": "ALT and AST elevation with normal or mildly elevated ALP suggests a hepatocellular pattern.",
          "chunk_id": "aasld-elevated-liver-enzymes-approach-3",
          "source_id": "aasld-elevated-liver-enzymes",
          "evidence_ids": ["e1", "e2"]
        }
      ]
    },
    {
      "kind": "standard",
      "claims": [
        {
          "text": "指南中，MASLD 的诊断需要影像学脂肪肝证据 + 至少一项代谢异常，并排除其他原因。本次资料缺少影像学证据，无法判断是否符合。",
          "quote": "MASLD requires hepatic steatosis plus at least one cardiometabolic criterion.",
          "chunk_id": "aasld-elevated-liver-enzymes-standard-2",
          "source_id": "aasld-elevated-liver-enzymes",
          "evidence_ids": ["e1"],
          "standard_check": {
            "name": "MASLD",
            "complete": false,
            "missing": ["影像学脂肪肝证据"]
          }
        }
      ]
    }
  ]
}
```

字段约束：

- `kind`：`causes` / `checks` / `advice` / `tracking` / `standard`。
- 每段 1–4 条 claim；总数 ≤ 12。
- `text` ≤ 200 中文字；`quote` ≤ 500 字符。
- `chunk_id` 必须来自本次检索集合。
- `source_id` 必须来自本次检索集合。
- `evidence_ids` 必须来自事实层证据映射。
- `standard_check` 仅用于 `standard` section；`complete=true` 时 `missing` 必须为空。

### 8.3 Prompt 规则

1. 只能使用提供的 chunk；不得使用模型自身医学知识补充事实。
2. 每条 claim 必须带一条逐字 `quote`；quote 必须来自对应 chunk。
3. 不得写诊断结论（「你有 X」）；只允许「指南中，X 的诊断标准是…」和「本次符合/不符合…（证据完整时）」。
4. 不得写治疗、药物、剂量、预后。
5. 不得写个体化复查医嘱；只能写「指南建议…」。
6. 不得写具体复查间隔，除非 chunk 原文明确写出，并标注「指南建议」。
7. 缺少证据时写「缺少…，无法判断」，不得写「符合」。
8. 只输出 JSON；不要输出 Markdown。
9. 不引用儿科内容；输入已过滤。
10. 若某段没有足够证据，省略该段；不要编造。

### 8.4 运行时校验（规则 only）

校验顺序：

1. **结构**：JSON 合法；`sections` 非空；`kind` 合法；claim 数量/长度合规。
2. **引用存在**：`chunk_id` 在本次检索集合；`source_id` 匹配；`evidence_ids` 在映射内。
3. **引文真实**：`quote` 归一化空白后是 chunk 文本的子串。
4. **数字/方向**：claim 中的数字必须来自事实层证据或 quote；方向词（升高/降低/正常）必须与 evidence flag 一致。
5. **边界**：
   - 禁止药物名/剂量模式（正则 + 词典）。
   - 禁止治疗动词（如「服用」「注射」「用药」）在非引用语境中出现。
   - 禁止预后词（如「会发展成」「预后」）。
   - 禁止个体诊断句式（「你有」「说明你有」「可以诊断为」）。
   - 禁止个体化复查句式（「你应该 … 复查」）。
6. **standard_check 一致性**：
   - `complete=false` → `missing` 非空，且 text 含「无法判断」/「缺少」；
   - `complete=true` → `missing` 为空，且 text 不含「无法判断」。
7. **来源渲染**：服务端按 `source_id` 去重，生成来源列表；模型不能写来源。

校验失败：

- 单条 claim 丢弃；
- 某 section 为空则省略；
- 全部为空 → 解释层省略，记录 `rag_validation_rejected`；
- 仍返回事实层 + 框架层。

**规则校验不能保证语义正确。** C0 下治疗段可能与允许段同 chunk；validator 只能抓显式模式。残余风险见 §12。

### 8.5 组装

- 服务端按 `kind` 聚合 claims，渲染为四段 + 「指南标准」卡片。
- claim 文本按列表项输出，不在服务端改写。
- 来源列表：
  - `source_id`、`title`、`org`、`year`、`authority`、`url`、`language`、`note`。
  - 每条来源附上被引用的 quote（可多条）。
- 解释层 `truncated` 继续保留，用于输出达到 `max_tokens` 的提示。

---

## 9. 数据模型与契约

### 9.1 领域类型

新增/替换：

```go
type KnowledgeChunk struct {
    ID           string
    DocumentID   string
    Text         string
    HeadingPath  string
    Sections     []string
    Panels       []string
    Analytes     []string
    Patterns     []string
    AgeGroup     string
    Language     string
    Authority    string
    Year         int
    ContentHash  string
    CorpusVersion string
}

type RetrievedChunk struct {
    KnowledgeChunk
    Rank  int
    Score float64
}

type Claim struct {
    Text           string
    Quote          string
    ChunkID        string
    SourceID       string
    EvidenceIDs    []string
    StandardCheck  *StandardCheck
}

type StandardCheck struct {
    Name     string
    Complete bool
    Missing  []string
}
```

删除：

- `KnowledgeEntry`
- `EntryMatcher`
- `EntryVersions`
- `Snapshot.EntryVersions`
- `ExplanationSection.EntryID` / `BridgeText` / `CheckDirections` / `GeneralAdvice` / `TrackItems`
- `knowledge/*.yaml` 的生产加载

保留：

- `internal/analysis/testdata/legacy_yaml/`：只供离线评估 baseline，不参与生产。
- 现有 `Source` 类型扩展，不删除 `standard` / `note`（框架层继续使用）。

### 9.2 HTTP / OpenAPI

`AnalysisExplanation` 改为：

```json
{
  "sections": [
    {
      "kind": "causes",
      "claims": [
        {
          "text": "...",
          "quote": "...",
          "chunk_id": "...",
          "source_id": "...",
          "evidence_ids": ["e1"],
          "standard_check": {
            "name": "MASLD",
            "complete": false,
            "missing": ["影像学脂肪肝证据"]
          }
        }
      ]
    }
  ],
  "sources": [
    {
      "source_id": "aasld-elevated-liver-enzymes",
      "standard": "AASLD",
      "title": "How to approach elevated liver enzymes",
      "org": "American Association for the Study of Liver Diseases",
      "note": "...",
      "year": 2024,
      "authority": "high",
      "language": "en",
      "url": "https://..."
    }
  ],
  "truncated": false
}
```

`AnalysisSnapshot` 新增：

```json
{
  "rag": {
    "corpus_version": "2026-11-06.1",
    "index_version": "2026-11-06.1",
    "embedding_model": "BAAI/bge-m3",
    "retriever_version": "rag-v1",
    "prompt_version": "rag-v1",
    "model": "deepseek-flash",
    "retrieved_chunks": [
      {"chunk_id": "...", "rank": 1, "score": 0.82}
    ]
  }
}
```

`AnalysisQuota.explanation_reason` 新增：

- `rag_disabled`
- `rag_index_unavailable`
- `rag_unavailable`
- `rag_no_results`
- `rag_validation_rejected`

`analysis_events.kind` 新增（三处镜像：Go 常量、迁移 CHECK、`AnalysisMetrics.degradation_kinds`）：

- `rag_disabled`
- `rag_index_unavailable`
- `rag_unavailable`
- `rag_no_results`
- `rag_validation_rejected`

### 9.3 快照与缓存

- 快照永久保留，存 `rag` 元数据 + 检索 chunk id/rank/score；不存全文。
- 缓存 key 加入：
  - `corpus_version`
  - `index_version`
  - `embedding_model`
  - `retriever_version`
  - `prompt_version`
  - `model`
  - `output_token_limit`
- 缓存命中不消耗 LLM 配额，也不重新 embedding。
- v1 不做 `source_updated` / `entry_updated` 提示；旧数据可删除。
- 旧 `entry_versions` 不再读；新分析不写。

### 9.4 迁移与重置

- 新迁移 `0021_rag.sql`：`CREATE EXTENSION vector` + `rag_*` 表。
- Postgres 镜像改为 `pgvector/pgvector:pg17`。
- 项目未上线：
  - 删除 `analyses`、`analysis_events`、`analysis_llm_calls`、`analysis_explanation_cache` 的存量行；
  - 不写回填、不写兼容转换；
  - 旧 App 版本不需要兼容。
- 删除生产 YAML 加载路径；`knowledge/*.yaml` 移到 `internal/analysis/testdata/legacy_yaml/`。
- 更新 `docs/agents/contracts.md` 的 Analysis 镜像表。

---

## 10. App / UI

### 10.1 选型

采用原型 **方案 B：内联引用编号 + 来源列表**。

- 保留四段式卡片。
- 每条 claim 末尾显示 `[1]` `[2]` 引用编号。
- 点击编号：
  - 若来源列表在同一卡片内，滚动到对应来源并高亮；
  - 来源较多时，打开现有 `SourceSheet`。
- 来源项显示：编号、标题、机构、年份、权威等级、被引用 quote、「打开原文」。
- 「指南标准」卡片单独展示，含「本次资料缺少…，无法判断」提示。
- 不做逐条内联 quote 展开（那是方案 A）；不把四段压成纯聚合文字（方案 C 已否决）。

### 10.2 组件改动

- `apps/mobile/src/analysis/components.tsx`
  - `ExplanationCard`：改为 `sections -> claims` 渲染。
  - 新增 `ClaimRow`、`CitationButton`、`InlineSourceList`、`StandardCard`。
  - `SourceSheet`：支持新来源字段与 quote 列表。
- `apps/mobile/src/analysis/facts.ts`
  - 删除 `SOURCE_COPY` 硬编码；`sourceDisplay` 改为优先使用 API 的 `title/org/year/authority`，缺失时回退 `standard/note`。
- `apps/mobile/src/app/analysis/[analysis_id].tsx`
  - `visibleAnalysisSources` 改为按 `claims[].source_id` 过滤；确保 RAG 来源不被隐藏。
- `apps/mobile/src/api/analyses.ts` + `schema.d.ts`
  - 重新 codegen；更新类型。
- RNTL 测试：
  - 四段 claims 渲染；
  - 引用编号点击滚动/打开 sheet；
  - 「无法判断」标准卡片；
  - 来源缺失/未知字段回退；
  - 解释层 unavailable（rag_disabled / rag_no_results）；
  - 儿科固定提示；
  - 旧数据不兼容（不测）。
- 原型：`.scratch/rag-liver/prototype/prototype.html` + `shots/02-b-inline-citation.png`、`04-c-source-sheet.png`（B 复用来源弹层）。

### 10.3 UI 文案

- 结果页固定声明：「内容来自指南原文，不是诊断。」
- 标准卡片：「指南标准」+「本次资料缺少 X，无法判断是否符合；可结合医生意见补充检查。」
- 儿科：「儿童肝功解读需要结合年龄和儿科医生意见，本版本暂不提供。」
- 解释不可用：
  - `rag_disabled`：「解读功能未开启。」
  - `rag_no_results`：「本次没有找到可引用的指南段落，事实与分级仍可用。」
  - `rag_unavailable`：「解读服务暂时不可用，事实与分级仍可用。」
  - `rag_validation_rejected`：「本次生成的解读未通过校验，已省略；事实与分级仍可用。」

---

## 11. 评估与验收

### 11.1 评估集

- 40 个成人肝功 case：5 类模式 × 8。
- 来源：`samples/expected/report/*.json` 的 golden + 手工边界 case；合成/脱敏数据。
- 每个 case 包含：
  - 成员年龄/性别（成人）；
  - 指标点、flag、框架标签；
  - 期望命中的 chunk ID（每个 required section 至少 1 个）；
  - 3–5 个「必须覆盖的要点」；
  - 「禁止出现的结论」清单（治疗/药物/剂量/预后/个体诊断/个体化复查/儿科）；
  - 证据完整性标注（标准判断用）。
- 额外的对抗 case：
  - 治疗段与允许段同 chunk，且治疗段高度相关；
  - 儿科 chunk 高度相关，验证年龄过滤；
  - 证据不完整的标准判断，验证「无法判断」；
  - 伪造 quote / 数字反向 / 个体诊断句式。

### 11.2 指标与 gate

| 指标 | 门槛 | 说明 |
| --- | --- | --- |
| 检索 Recall@10 | ≥ 0.90 | 每个 required section 至少命中 1 个相关 chunk |
| Quote 子串校验 | 100% | quote 必须是 chunk 子串 |
| 数字/方向校验 | 100% | 规则校验 |
| Claim entailment | ≥ 90% | 评估期 LLM judge + 20% 人工抽检 |
| 政策违规 | 评估集 120 次运行中「观察到 0」 | C0 下不能保证 0；残余风险接受 |
| 覆盖率 | ≥ 80% | causes + checks 至少两段有内容 |
| 标准判断 | 100% 一致 | `complete=false` 必须「无法判断」；`complete=true` 才可「符合」 |
| 儿科泄漏 | 0 | 成人 case 不得引用 pediatric chunk |
| 有用性 | ≥ 3.5/5，且无 case < 2 | 盲评 |
| 延迟 | 检索 ≤ 1s；端到端 p95 ≤ 30s | 本地 embed + LLM |

- 每个 case 跑 3 次（LLM 非确定性）。
- 安全 gate 看所有 run；质量 gate 看中位数。
- 评估报告按 `corpus_version / index_version / prompt_version / model / commit` 归档。
- 评估脚本输出 Markdown + JSON；典型 case 前后对比附在报告里。

### 11.3 Judge 与人工复核

- Judge：DeepSeek pro/reasoner；生成用 flash。
- 同供应商 judge 有自评偏差；报告必须记录。
- 人工抽检 20%；无第二评审人，单人审核风险明确记录。
- 盲评：至少隔一个月回看一次真实输出，避免「当时觉得对」。

### 11.4 上线 gate

| Gate | 内容 | 通过条件 |
| --- | --- | --- |
| G1 离线评估 | 40 case × 3 | §11.2 全部通过 |
| G2 离线影子 | 20 个真实脱敏 case，不面向用户 | 观察到 0 政策违规；quote 100%；有用性平均 ≥ 3.5/5，且无 case < 2 |
| G3 直接切换 | 生产 RAG-only + kill switch | 不做生产灰度；切换后监控 2 周反馈率/降级率 |
| G4 清退 YAML | 删除生产 YAML 路径 | G3 稳定后执行 |

- G2 不是生产灰度，是离线人工复核；不向家长展示。
- 生产切换前删除存量分析数据；不做兼容。

---

## 12. 风险与已接受的取舍

### 12.1 已接受的风险

| 风险 | 决策 | 缓解 |
| --- | --- | --- |
| **C0：治疗段与允许段同 chunk，仅靠 prompt + validator** | 接受 | 强 prompt；运行时规则校验；对抗 case；评估集观察 0 违规；文档明确「不是保证」 |
| 单人审核，无第二评审人 | 接受 | 20% 人工抽检；隔月盲评；风险写入文档 |
| 允许「符合某指南标准」，诊断边界前移 | 接受 | 证据完整才允许；`standard_check` 结构校验；缺证据必须「无法判断」；评估集覆盖完整/不完整两类 |
| 不做生产灰度 | 接受 | 离线 G2 + kill switch；切换后 2 周监控 |
| 不兼容旧数据 | 接受 | 删除存量分析；不做回填；旧 App 不兼容 |
| 本地 BGE-M3 占 NAS 资源 | 接受 | 独立容器、CPU、模型缓存；超时/失败降级 |
| 同供应商 judge 自评偏差 | 接受 | 用 pro 档；20% 人工抽检；报告记录 |

### 12.2 主要风险

- **语义错误**：quote 真实不等于理解正确。规则 validator 抓不住语义反转/过度推断。评估期 entailment + 人工抽检是唯一防线。
- **治疗内容泄漏**：C0 下治疗段可能被检索、被引用。规则只能抓显式药物/剂量；隐含治疗建议可能漏过。
- **诊断标准误用**：模型可能在没有完整证据时写「符合」。`standard_check` 只能校验结构，医学完整性靠评估。
- **语料质量**：语料决定上限；用户清单未提供前，覆盖模式无法锁定。
- **模型漂移**：DeepSeek 模型版本变化会影响输出；模型 ID 必须固定并随评估记录。
- **版权**：全文只存本地，不入 git；短引文入 manifest；仍需用户确认来源许可。
- **儿科语料误检索**：必须用 `age_group=adult` 硬过滤，并有对抗 case。
- **pgvector/镜像变更**：Postgres 镜像切换需要备份与迁移；本地栈可用 `deploy/reset-data.sh` 重建。

### 12.3 开放问题

1. **语料清单**：用户提供文档名/链接/年份/语言/是否含儿科/是否含治疗段。未提供前，模式覆盖和 eval case 只能占位。
2. **模型 ID**：flash / pro 的具体模型名与 base URL。
3. **NAS 规格**：BGE-M3 的内存/CPU 是否足够；是否需要限制并发。
4. **「指南标准」呈现**：作为独立卡片还是并入「可能原因」；原型按独立卡片设计。
5. **来源列表长度**：来源超过 3 条时是否默认折叠为「查看全部来源」。
6. **`source_updated`**：v1 不做；后续语料更新后是否需要「依据已更新」提示。
7. **语料版本归档**：私有存储位置与保留策略。

---

## 13. 实施计划

| 阶段 | 内容 | 产出 | 估算 |
| --- | --- | --- | --- |
| 0 | 用户提供语料清单 + PDF/HTML | `corpus/manifest.yaml`、`corpus/private/` | 阻塞项 |
| 1 | RAG 基础设施 | `0021_rag.sql`、`rag-embed` 容器、compose 更新 | 3–5 天 |
| 2 | 离线 ingestion | `rag/ingest.py`、manifest 校验、索引版本 | 3–5 天 |
| 3 | 检索 | 查询构造、pgvector 适配、RRF、单测 | 3–5 天 |
| 4 | 生成与校验 | prompt、claim schema、validator、单测 | 5–7 天 |
| 5 | 契约与 App | OpenAPI、Go 类型、codegen、UI B、RNTL | 5–7 天 |
| 6 | 评估 | 40 case、judge、报告、G1 | 5–7 天 |
| 7 | G2 + 切换 | 20 真实 case、kill switch、监控 | 3–5 天 |

总计约 **4–5 周**（语料就绪后，单人、熟悉代码库）。人工 curation 约 10–20 小时；评估标注约 20–30 小时。

### 13.1 实施顺序（依赖）

1. 先做 **检索-only**：manifest → chunk → embed → pgvector → 查询，先证明 Recall@10。
2. 再接 **生成 + 校验**：先用固定 chunk 做 prompt 迭代，再接真实检索。
3. 再做 **UI**：契约定稿后按原型 B 实现。
4. 最后做 **评估 gate + 切换**；YAML 在 G3 前只作离线 baseline。

### 13.2 文档与决策

- RAG 架构决策见 `docs/adr/0018-rag-explanation-layer.md`（2026-11-06 采纳），记录：RAG 取代 YAML、本地 BGE-M3 + pgvector、C0 残余风险、无生产灰度。
- 更新 `CONTEXT.md`：知识块、检索、claim、引文、语料版本；更新「分析」解释层定义。
- 更新 `README.md` 的范围与安全边界。
- 更新 `docs/agents/contracts.md` 的 Analysis 镜像表。
- 评估报告归档到 `.scratch/rag-liver/eval/`（或私有存储），按版本命名。

### 13.3 Definition of Done

- [ ] 语料 manifest + 全文私有目录就绪，索引可重复构建。
- [ ] `rag-embed` 容器健康；pgvector 迁移可重复执行。
- [ ] 检索 Recall@10 ≥ 0.90（G1）。
- [ ] Quote/数字/边界 gate 通过；观察到 0 政策违规。
- [ ] 标准判断完整/不完整两类 case 100% 正确。
- [ ] 儿科 chunk 0 泄漏。
- [ ] App 按方案 B 渲染 claims + 引用 + 来源；RNTL 通过。
- [ ] kill switch 生效：关闭后只输出事实层 + 框架层。
- [ ] 生产 YAML 路径删除；legacy YAML 只在评估目录。
- [ ] `go test ./...`、`go vet ./...`、`npm run typecheck`、`npm test`、`npm run lint -- --max-warnings=0`、`bash scripts/test-db.sh`、`python3 scripts/check-docs.py` 全绿。
- [ ] README / CONTEXT / OpenAPI / fixtures / `docs/agents/contracts.md` 同步。
- [ ] 评估报告归档（G1/G2），含残余风险声明。

---

## 14. 决策记录

| # | 决策 | 被否决的替代 |
| --- | --- | --- |
| 1 | 主目标 A+C，后续 B；长期删除 YAML | 继续扩 YAML；开放问答 |
| 2 | LLM 写四段，服务端校验（C） | 抽取式；半生成；逐句引文 |
| 3 | 语料 10–20 份，人工下载，全文本地私有 | 自动爬虫；只存 embedding |
| 4 | git 版本化 manifest + 离线构建 | 数据库管理后台；自动更新 |
| 5 | 允许病因/检查/建议/跟踪/红黄绿灯；禁止治疗/药物/剂量/预后 | 只做病因；放开治疗 |
| 6 | 本地 BGE-M3 + pgvector + dense/sparse | 云端 embedding；文件索引；Qdrant |
| 7 | claim + chunk_id + 逐字 quote | 只引 chunk；原子 claim |
| 8 | 运行时规则 only；评估期 entailment | 运行时第二 LLM；全量 entailment |
| 9 | v1 成人-only；儿科语料入库不检索 | 支持儿童；不入儿科语料 |
| 10 | 生产 RAG-only + kill switch；YAML 只作离线 baseline | YAML 兜底；生产灰度 |
| 11 | **允许「符合某指南标准」，但证据完整才允许** | 完全禁止诊断标准判断；无条件允许 |
| 12 | 语料整段保留，不拆内容类型（C0） | 按内容类型拆；句子级标签 |
| 13 | UI 方案 B：内联编号 + 来源列表 | A 逐条展开；C 现状聚合 |
| 14 | 12k input / 2.5k output / 20 次/天 | 8k/1.5k；降配额 |
| 15 | 无第二评审人；DeepSeek flash 生成 / pro judge | 引入外部 judge；人工全量 |

---

## 15. 附录：API 响应示例

```json
{
  "analysis_id": 42,
  "status": "success",
  "layers": {
    "facts": { "...": "不变" },
    "framework": { "...": "不变" },
    "explanation": {
      "sections": [
        {
          "kind": "causes",
          "claims": [
            {
              "text": "指南中，ALT/AST 升高而 ALP 不明显升高时，通常先归为肝细胞型损伤。",
              "quote": "ALT and AST elevation with normal or mildly elevated ALP suggests a hepatocellular pattern.",
              "chunk_id": "aasld-elevated-liver-enzymes-approach-3",
              "source_id": "aasld-elevated-liver-enzymes",
              "evidence_ids": ["e1", "e2"]
            }
          ]
        },
        {
          "kind": "standard",
          "claims": [
            {
              "text": "指南中，MASLD 的诊断需要影像学脂肪肝证据 + 至少一项代谢异常，并排除其他原因。本次资料缺少影像学证据，无法判断是否符合。",
              "quote": "MASLD requires hepatic steatosis plus at least one cardiometabolic criterion.",
              "chunk_id": "aasld-elevated-liver-enzymes-standard-2",
              "source_id": "aasld-elevated-liver-enzymes",
              "evidence_ids": ["e1"],
              "standard_check": {
                "name": "MASLD",
                "complete": false,
                "missing": ["影像学脂肪肝证据"]
              }
            }
          ]
        }
      ],
      "sources": [
        {
          "source_id": "aasld-elevated-liver-enzymes",
          "standard": "AASLD",
          "title": "How to approach elevated liver enzymes",
          "org": "American Association for the Study of Liver Diseases",
          "note": "肝细胞型定义与评估路径",
          "year": 2024,
          "authority": "high",
          "language": "en",
          "url": "https://www.aasld.org/..."
        }
      ],
      "truncated": false
    }
  },
  "snapshot": {
    "...": "不变",
    "rag": {
      "corpus_version": "2026-11-06.1",
      "index_version": "2026-11-06.1",
      "embedding_model": "BAAI/bge-m3",
      "retriever_version": "rag-v1",
      "prompt_version": "rag-v1",
      "model": "deepseek-flash",
      "retrieved_chunks": [
        {"chunk_id": "aasld-elevated-liver-enzymes-approach-3", "rank": 1, "score": 0.82}
      ]
    }
  },
  "sources": ["...合并框架层 + 解释层来源..."],
  "quota": {
    "used_today": 1,
    "limit_today": 20,
    "explanation_available": true
  },
  "stale": false
}
```

---

## 16. 附录：原型与截图

- 可交互原型：`.scratch/rag-liver/prototype/prototype.html`
- 三方案对照：`.scratch/rag-liver/prototype/shots/00-contact-sheet.png`
- 选定方案 B：`.scratch/rag-liver/prototype/shots/02-b-inline-citation.png`
- 来源弹层：`.scratch/rag-liver/prototype/shots/04-c-source-sheet.png`

原型只用于信息架构选型；生产实现必须复用现有设计系统与组件，不直接搬 HTML。
