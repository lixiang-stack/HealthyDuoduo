# samples · 样本库

- **报告大类分组**(2026-10 词典重组):`report_type` 输出为大类 **临检/生化/免疫/激素/影像/其他专项**
  (临检=cbc/ua/coag/bloodtype 走 logical merge 未并文件;生化=chemistry(lft/glu/vitanemia)、
  免疫=immunology(ana/infectious/fibrosis)、激素=endocrine(tft/proghcg)、
  影像=us(logical merge;当前仅产检超声测量词表,叙述体)、
  其他专项=special(产筛,narrative mode;无创DNA/基因检测/微生物按需续录)。
  样本 id 前缀仍按细类语义命名,与 report_type 解耦;掩码语料无原图、编号接续该细类图像样本。)

- 只收**脱敏**图像(NF-01);未脱敏原图永不入库、永不提交本目录。
- 目录约定(实施计划 §7-3):
  - `samples/<id>.jpg`
  - `samples/expected/ocr/<id>.json`(P1 golden)
  - `samples/expected/report/<id>.json`(P2 golden)
  - `samples/expected/ocr/<id>.json` 亦可承载 **脱敏 OCR 语料条目**:本地未脱敏真实报告
    经 `recognizer/tools/local_golden.py mask` 脱敏(表格主体外的姓名/证件号/机构/地址/
    时间一律置 `◆`),**不附原图**(未脱敏原图永不提交)。该类条目 `test_ocr_golden`
    自动跳过,`expected/report/<id>.json` 照常由 `python -m recognizer.golden` 生成并回归。
- 当前样本(P3 起按报告类别前缀分目录 …… 沿用 `samples/<id>.<ext>` 平铺):
  - cbc_01~05:真实脱敏血常规(P2 入库)。**cbc_02 全单无日期/时间行**,用于
    date-null / partial 场景回归(P2 golden 已验证;P0 时曾记 cbc_03 为无日期版式,与
    真实样本不符——cbc_03 含「检验时间：2023-09-08」,以本次修订为准)。
  - glu_01~03 / lft_01~05 / tft_01~03:真实脱敏 血糖 / 肝肾功能 / 甲状腺功能(P3 第二波,
    2026-10-10 维护者提供并口头确认脱敏合格;版式覆盖见各词典文件头注释)。
  - ua_01/ua_04/ua_05 / us_01~03:真实脱敏 尿常规 / 超声(P3 第二波后半段;尿常规定性形态,
    超声叙述体走 postprocess 的 narrative 模式)。**ua_02/ua_03 为异常图像,已移出样本集**
    (其别名证据一并从 `ua_dict.yaml` 清理)。
  - 另有 cbc 无前缀 1.jpeg~5.jpg 维护者留底(未入 golden,勿提交)。
  - **掩码 OCR 语料**(`mask` 产物,无原图):ana_01~05 / bloodtype_01 / cbc_06~09 / coag_01 /
    fibrosis_01 / glu_04 / infectious_01~02 / lft_06~09 / proghcg_01 / tft_04~05 / ua_06~07 /
    unknown_01 / vitanemia_01~02(共 26 条,细类编号接在该类别图像样本之后;id 只作样本编号,
    与 report_type 解耦)。
    ana_04/05(2026-10-18 从共享栈真实报告冻结后脱敏):**左对齐长名称列**版式载体——
    名称格中心随文本长度漂移,整行不再被列归属固定容差丢弃(ana_04 抗核抗体谱 16 项,
    含 OCR 漏值行;ana_05 自免肝谱 10 项,兼作 `抗Sc1-70抗体`/`ANA(IIF) (ANA(IIF))`
    别名证据)。
    ua_07(2026-10-18 同上):竖排回退误把页脚「实验室地址:…155号」抽成 value 的真实载体
    (尿潜血 value 155→null;候选门的「数值 token 居格首」约束由它锁定)。
    unknown_01(产前筛查)已由 其他专项 词典收录,narrative 模式逐格收录项目名、
    value=None 留痕 raw_text(status=partial 属诚实落痕,漏值待行内配对规则扩展)。
- **真实数据基准 `expected/vl/<id>.json`**:由视觉模型 + LLM 结构化得到的**逐项真值**
  (`name/abbr/value/unit/ref_range`,印刷原样),**禁改**;抽取/规则迭代以它为对照基准,
  评测脚本 `recognizer/tools/gt_eval.py`(启发式 vs 网格等方案)。
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
