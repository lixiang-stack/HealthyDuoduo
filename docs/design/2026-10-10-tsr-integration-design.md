# 检查报告后处理 · TSR 前端集成设计

**版本：** 0.1（草案，2026-10-10）
**状态：** 已实施（2026-10-10；决策见 [ADR-0004](../adr/0004-table-structure-recognition-frontend.md)）

## 1. 为什么做

P3 review 暴露三处后处理脆弱性，根因都在**几何层**而非词典层：

1. 分类分发对「标题命中」过度信任，正文偶发提及可把报告抢到不相干类别（已由 review 修复①`p3-content-first-classify` 处理，本设计不重复）。
2. `_extract_items`/`_role_centers` 用 OCR 行框 + 绝对像素容差启发式重建行列：lft_05 把元数据标签「检验项目：生化类」误判为列中心、误切左右半栏，**丢失 ALT/AST 两行**；每来一批新版式都要改解析代码，自适应差。
3. 依赖固定表头关键词集（`ROLE_HEADERS`），新增印形持续打补丁。

词典型数据文件（规范名/别名/单位、真实语料门禁）是稳定且可测试的，应保留；要替换的是**表格结构还原**这一层。原型对比（`recognizer/tools/tsr_compare.py`）证明 TableStructureRec 能把几何还原的 clean 均值从 0.35/0.63（SLANet_plus）提升到 **0.97、0 退化**，并修好 lft_05。

## 2. 做什么（范围 / 非目标）

**做：**

- 仅 `category.mode=table` 的报告类别，抽取前增加 TSR 前端：模型还原单元格网格 → 语义列映射（名称/结果/单位/参考）→ 复用现有词典 `_match_name` 与 `_decompose`。
- 采用 TableStructureRec（`wired_table_rec` + `lineless_table_rec`，ONNX），**同时运行两子模型并择优**（择优规则见 §3.4）。
- TSR 输出退化时**回退**现有启发式抽取（保证不劣化）。
- 契约与存储扩展以支持 `/reparse` 复用（§3.2）。

**非目标：**

- 叙述体（超声 `mode=narrative`）：无表头/行列，**不适用**，保持现有行内配对。
- 更换分类算法（已在 review 修复①）。
- 引入 PaddlePaddle / 多模态 LLM（NF-01 本机、免 LLM 路径）。

## 3. 怎么实施

### 3.1 依赖与离线（NF-01）

- 新增 `wired-table-rec`、`lineless-table-rec`（ONNX，依赖 onnxruntime/opencv/scipy/scikit-image/shapely），已入 `pyproject`。
- **Python 版本**：两包声明 `requires_python <3.13` → 已决策**降 recognizer 至 3.12**（`.python-version` / `pyproject` / Dockerfile 同步）。
- 模型为运行时从 ModelScope 下载 → 已在 `deploy/recognizer.Dockerfile` **构建期预下载**（运行时离线可用，NF-01）。

### 3.2 契约与 reparse（NF-05/NF-06）

TSR 需要**原图**（裁格/复识别），而 `/reparse` 原仅持 OCR 结果。**采用方案 A（已实施）**：识别结果契约增**可选** `table_structure`（`html` + `model` + `elapse`），随 OCR 结果一同落 `ocr_results` jsonb；`/reparse` 复用已存结构，不需图像。已同步 `schemas/ocr_result.schema.json` 与 Go `internal/contract`。（相对初稿省去 `cell_boxes`/`logic_points`：postprocess 只需 HTML 网格。）

### 3.3 抽取管线（新增 TSR 路径）

```
OCR results ──┬─(有 table_structure)─► grid → 语义列映射 → _match_name + _decompose → items
              └─(无 / TSR 退化)───────► 现有 _extract_items 启发式 → items
```

- grid 解析：`<tr>/<td>` → 行×格；处理 `rowspan/colspan`（合并格按覆盖范围展开）。
- 语义列映射：在网格中找表头行（复用 `ROLE_HEADERS`），据列标题确定 name/value/unit/ref 列；单元格内仍用 `_decompose` 拆「值/范围/单位熔断」。
- 名称匹配保持 `_match_name`（词典）与 `_low_score` 逻辑不变；`raw_text` 改为该行网格文本拼接。

### 3.4 子模型择优与回退

- 同时跑 `wired_table_rec` 与 `lineless_table_rec`，取「非退化优先、格数多者」为结果。
- 两者皆退化（行列<2）→ 回退现有启发式抽取（记录日志，供坏例库归档）。
- `table_cls` 单模型路由弃用（原型显示误判率高、且是 lft_05/tft_02 失败的根因）。

### 3.5 测试与 golden

- 单测：grid 解析（含 rowspan）、语义列映射、择优规则、回退触发。
- golden：既有 24 张样本**重基**（TSR 路径产出新结构，人工核对 `raw_text` 后再冻结）；lft_05 期望恢复为完整 19 项。
- 契约：schema 校验 + Go fixture 双向。

## 4. 怎么验收（状态 2026-10-10）

- [x] 表格类报告走 TSR 路径（`/report` 填充 `table_structure`，退化回退启发式）；**lft_05 恢复全部 19 项**（原 17），其余 23 张报告 golden 不变。
- [x] TSR 退化自动回退且不劣化：`_extract_pairs` 取「网格 / 启发式」命中项数多者；`tests/test_table_structure.py`（退化/异常/择优）与 `test_postprocess.py`（网格抽取、双栏、熔断、无表头回退）覆盖。
- [x] `/reparse` 仅凭已存 `table_structure`（`expected/ocr` 已含）产出与 `/report` 一致（golden 回归）。
- [x] 离线/容器：`docker build` 成镜像（模型构建期预下载）；`docker run --network none` 实测 cbc_01/lft_05 端到端出报告，无运行时下载。
- [x] 契约双语一致：`schemas/ocr_result.schema.json` + Go `internal/contract` + pydantic 同步。
- [ ] 数值/单位/参考**逐项人工核对**（本轮以 golden 机器回归为主；`flag` 计算由既有单测覆盖）。
- [x] CI 双链绿：ruff + pytest（140 passed）；go vet + gofmt + go test。
