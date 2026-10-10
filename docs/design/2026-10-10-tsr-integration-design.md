# 检查报告后处理 · TSR 前端集成设计

**版本：** 0.1（草案，2026-10-10）
**状态：** 待维护者确认（决策见 [ADR-0004](../adr/0004-table-structure-recognition-frontend.md)）

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

- 新增 `wired-table-rec`、`lineless-table-rec`（ONNX，依赖 onnxruntime/opencv/scipy/scikit-image/shapely）。
- **Python 版本冲突**：两者声明 `requires_python <3.13`，本项目锁 3.13。实测 uv 可安装且 3.13 运行正常，但上游未声明支持。**待决策**：降 recognizer 至 3.12（官方支持）或将风险记录在案、接受 3.13。
- 模型为运行时从 ModelScope 下载 → 容器/离线需**自带模型并固定路径**（`WiredTableInput(model_path=...)` / `LinelessTableInput(model_path=...)`），纳入 `deploy/` 镜像构建。

### 3.2 契约与 reparse（NF-05/NF-06）

TSR 需要**原图**（裁格/复识别），而 `/reparse` 现仅持 OCR 结果。方案（二选一，建议 A）：

- **A（推荐）**：在识别结果契约中增**可选** `table_structure`（`pred_html` + `cell_boxes` + `logic_points`），随 OCR 结果一同落 `ocr_results` jsonb；`/reparse` 复用已存结构，不需图像。同步改 `schemas/ocr_result.schema.json` 与 Go `internal/contract`。
- B：`/reparse` 保留图像引用，重跑时重新执行 TSR——需改存储与 CLI，且违反「reparse 只重跑后处理」的轻量语义。

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

## 4. 怎么验收

- [ ] 24 张（或扩至 ≥20 的有效表类样本）走 TSR 路径，`clean`（名称独占格）≥ 0.9、**lft_05 恢复全部指标项**（对照人工核对清单）。
- [ ] TSR 退化样本自动回退启发式，产出不劣于现状（血常规 5 张既有 golden 人工核对不劣化）。
- [ ] `/reparse` 在仅有已存 `table_structure`（无图像）前提下产出与 `/report` 一致。
- [ ] 离线/容器：断网镜像内可跑（模型自带）；单张表类样本 OCR+TSR 端到端 < 3s 量级（NF-04）。
- [ ] 数值/单位/参考范围正确性：对每类 ≥3 张样本人工核对，`flag` 计算正确。
- [ ] CI 双链绿（ruff+pytest；go vet+gofmt+go test）。
