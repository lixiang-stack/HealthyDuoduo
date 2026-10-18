# 02: e2e verify 增 ocr_lines 链接覆盖率与歧义裕度持续度量

**What to build:** `tools/e2e.py verify` 增加 Ocr 行链接度量：每份报告的每个指标项「行链接覆盖 + 链接行 join 与 raw_text 一致 + 行框与 OCR box 一致」为硬门禁；「歧义度（重复碎片消歧被启用的项数）与歧义裕度（所选链接的 y 聚带比）」为持续度量并打印最差项。经 hdd serve `/api/v1/reports/{id}`（票 07 契约）与库内 OCR JSON 对账。

**Status:** resolved

- [x] 结构门禁：无链接 / join 不一致 / box 不一致即 FAIL（含 hdd serve 不可达的明确报错）
- [x] 度量输出：歧义度计数 + 最差 y 聚带比（>1 打印告警并点名，供抽查）
- [x] `docs/agents/e2e.md` 与脚本 docstring 同步（HDD_SERVE_PORT 口径）
- [x] 真栈验证：198/198 项链接一致；度量当场抓出 尿潜血 页脚散文格污染（ratio 12.02），随票 04 修复回落至 1.97（斜拍版式的良性上限）

## Comments

**实现（分支 `recognizer-quality` 内单 commit；与 03/04 同 PR）**

- **放置**：`verify()` 在单位不变量之后、trend 检查之前调 `verify_ocr_lines()`；数据源 = hdd serve（`GET /reports/{sha}`，端口沿 compose 的 `HDD_SERVE_PORT` 缺省 8080）+ `psql` 取 `ocr_results.engine_output`（单个报告 1 次 HTTP + 1 次 SQL，16 份报告开销可忽略）。
- **硬门禁口径**（结构正确性，应为 100%）：①每项 `ocr_lines` 非空；②`"  ".join(strip(链接行文本)) == raw_text`；③每行 `box` 逐点等于库中该行 OCR box。任一不满足即 FAIL 并点名 sha/item——识别端 raw_text join 格式漂移、服务端行映射错位都会在此当场失败。
- **度量口径（非门禁）**：①**歧义度** = 链接行文本在单内多次出现的项数（同文本行的绑定歧义面；v2 前由服务端几何消歧消化，v2 后由识别端直吐下标，保留为持续观测；>0 属常态，实测 141/198）；②**歧义裕度** = 链接行中心 y 展开 / 最大行高——同一视觉行内应远小于 1，跨行错配才 >1。**真·最优解-备选解差值只在服务端内部存在**，契约无法暴露，故以「所选解的几何聚拢度」为外部代理；纵向回退项为合法跨行，故只告警不硬判；>1 时打印 worst 项供人工抽查。`warn` 有常客（共振斜拍/竖排项 ratio ≈1.1–2.0）属预期，与 RDW-CV 印刷串列 warn 同类。
- **证据**：真栈 `verify` 输出 `ok: ocr-lines: linked 198/198 items; join/box 一致` + `margin: ambiguous-resolution 141/198, max y-spread ratio 1.97 (worst bd3d…/人绒毛膜促性腺激素及β亚单位)`；首轮运行即抓出 `22f7054338/尿潜血 ratio 12.02`（页脚地址被吸收，属真实数据缺陷 → 立票 04 修复后回落）。
- **披露**：度量依赖已构建的 hdd serve 容器（票 07 端点）；老栈未重建时 verify 会以明确报错提示「hdd serve 不可达/端点缺失」，不静默跳过。
