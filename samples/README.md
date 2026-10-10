# samples · 样本库

- 只收**脱敏**图像(NF-01);未脱敏原图永不入库、永不提交本目录。
- 目录约定(实施计划 §7-3):
  - `samples/<id>.jpg`
  - `samples/expected/ocr/<id>.json`(P1 golden)
  - `samples/expected/report/<id>.json`(P2 golden)
- 当前样本(P3 起按报告类别前缀分目录 …… 沿用 `samples/<id>.<ext>` 平铺):
  - cbc_01~05:真实脱敏血常规(P2 入库)。**cbc_02 全单无日期/时间行**,用于
    date-null / partial 场景回归(P2 golden 已验证;P0 时曾记 cbc_03 为无日期版式,与
    真实样本不符——cbc_03 含「检验时间：2023-09-08」,以本次修订为准)。
  - glu_01~03 / lft_01~05 / tft_01~03:真实脱敏 血糖 / 肝肾功能 / 甲状腺功能(P3 第二波,
    2026-10-10 维护者提供并口头确认脱敏合格;版式覆盖见各词典文件头注释)。
    ua_01~05(尿常规) / us_01~03(超声) 已到位待第二波后半段(定性与叙述体版式)。
- golden(`expected/ocr/<id>.json`)由调试 CLI 生成:
  `cd recognizer && uv run python -m recognizer ../samples/<id>.<ext> > ../samples/expected/ocr/<id>.json`
  (stderr 的日志重定向丢弃;对比规则见实施计划 §8-5,elapse 只记录不比对。)
- golden(`expected/report/<id>.json`,P2)由后处理对 golden OCR 生成:
  `cd recognizer && uv run python -m recognizer.golden`
  (规则或词典改动后重新生成;报告对 OCR golden 的产出是确定性的,跨架构可复现。
  示例:cbc_02 全单无日期/时间行 → report_date=null + status=partial,是补录场景载体;
  cbc_03 为单栏清单且缺 血红蛋白/MCH 行(备注行挤占),partial 属正常回放。)
- 收录新样本时的已知边界:在 `samples/expected/ocr/` 落 golden 后用同一命令生成报告
  golden 并人工抽查。后处理对**已入库样本**的版式/指标收敛良好;**新样本如出现
  新版式或新指标名,不会崩溃但会漏抓**(未命中行被跳过 → 指标缺失 → status=partial/failed,
  供人工核对 raw_text)。扩展顺序:①改对应类别词典 `*_dict.yaml` 数据文件(别名/单位,
  新类别 = 注册表加文件)→ ②重跑 golden → ③仍不命中的版式才动 postprocess 解析规则
  (以真实样本 golden 锚定后再改)。
