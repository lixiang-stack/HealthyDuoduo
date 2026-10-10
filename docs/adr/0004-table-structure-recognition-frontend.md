# 0004 · 表格类报告引入表格结构识别（TSR）前端：选 TableStructureRec

**状态：** 已采用（2026-10-10，维护者决策：D1 接受 TableStructureRec；D2 识别服务降至 Python 3.12；D3 契约方案 A）。已实施。

表格类报告（血常规/尿常规/血糖/肝肾功能/甲状腺功能）的后处理，从「用 OCR 行框启发式重建表格几何」改为「表格结构识别（TSR）模型还原网格 + 语义列映射」。选定 **RapidAI TableStructureRec**（`wired_table_rec` + `lineless_table_rec`，ONNX 推理；**同时运行两个子模型并择优**，不依赖 `table_cls` 单模型路由）。叙述体（超声 `mode=narrative`）**不适用**，保留现有行内配对。词典数据文件与报告契约的字段语义不变。

## 背景

P3 review 发现：`postprocess._extract_items`/`_role_centers` 的几何重建对版式自适应差、依赖绝对像素容差，坏例（lft_05）会把元数据标签误判为列中心而丢行；每来一批新版式就要改解析代码。词典（数据文件、真实语料门禁）本身是稳的，脆弱点在**几何层**。设计计划 §10 已预告可换 PP-Structure/SLANet。

## 原型对比（2026-10-10，24 张真实样本）

方法：复用已存 OCR 框（不重复 OCR），跑三个方案，指标 `clean` = 该类别词典命中项里「独占一个单元格」的比例（左右双栏兼容；同格合并 ≥2 项记失败）。脚本：`recognizer/tools/tsr_compare.py`。叙述体样本不适用。

| 方案 | 输入 | n | 退化(行列<2) | clean 均值 | clean 最低 |
|------|------|---|-------------|-----------|-----------|
| SLANet_plus（rapid_table） | 整页 | 21 | 4 | 0.35 | 0.00 |
| SLANet_plus（rapid_table） | 裁剪到表格 | 21 | 0 | 0.63 | 0.00 |
| TableStructureRec（`table_cls` 路由） | 整页 | 21 | 2 | 0.84 | 0.00 |
| **TableStructureRec（wired+lineless 择优）** | 整页 | 21 | **0** | **0.97** | **0.80** |

- SLANet_plus **必须裁剪表格**才不退化，即便裁剪后仍显著落后。
- `table_cls` 会把有线表误判为无线（如 cbc_01），导致少数样本走错子模型而退化（lft_05/tft_02/cbc_04）；**同时跑 wired+lineless 择优即全部恢复**（lft_05 从 7×1 恢复到 26 行 49 格）。
- 结论：TableStructureRec 胜出；且不采用 `table_cls` 单模型路由。

## Considered Options

- **PP-StructureV3（PaddleOCR）**：能力最全（含版面/表格检测），但依赖 PaddlePaddle；macOS Apple Silicon 无官方 wheel（需源码编译），与现有 onnxruntime 栈冲突，本地开发不可用。否。
- **SLANet_plus（rapid_table，ONNX）**：同栈、轻，但必须裁剪且精度落后（0.63 vs 0.97）。否。
- **Table Transformer（TATR）**：DETR 系，公开评测精度更低、依赖 transformers/torch 更重。否。
- **维持启发式，只做加固（相对容差、表头元数据排除）**：成本最低，但不解决版式自适应的天花板。作为 TSR 退化时的**回退**保留，不作为主路径。

## Consequences

- 仅 `mode=table` 走 TSR；`mode=narrative`（超声）不变。
- TSR 需要**原图**（裁格/复识别），而 `/reparse` 原仅持 OCR 结果 → 采用**契约方案 A**：OCR 结果契约增**可选** `table_structure`（`html` + `model` + `elapse`），`/report` 填充并随 `ocr_results` 落库，`/reparse` 复用它（不再读原图），NF-05 闭环保持。已同步 `schemas/ocr_result.schema.json` 与 Go `internal/contract`（NF-06）。（相对原型设计省去 `cell_boxes`/`logic_points`：postprocess 只需 HTML 网格。）
- `wired-table-rec`/`lineless-table-rec` 声明 `requires_python <3.13`；本项目原锁 3.13 → **决策：识别服务降至 Python 3.12**（官方支持区间）。
- 模型为运行时从 ModelScope 下载 → 已纳入 `deploy/recognizer.Dockerfile` 构建期预下载，运行时离线可用（NF-01）。
- golden 已重基：OCR golden 增 `table_structure`；报告 golden 仅 lft_05 变化（17 → **19 项**），其余 23 张不变。
- 单张新增耗时：TSR 子模型 ~0.7s（CPU）+ 可选检测；仍在 NF-04 <3s 量级内。
- 指标 `clean` 只衡量「名称独占格」的结构正确性，**不含**数值/单位/参考范围的正确配对；数值解析仍由词典 + `_decompose` 承担，须在集成验收中另行验证。
