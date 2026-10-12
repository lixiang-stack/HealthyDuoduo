# 2026-10-12 ReportItem 契约扩展:value string 化与 method 字段

## 为什么做

`ReportItem` 两条契约缺口对真实样本实测暴露(review 发现):

1. **value `float | null` 丢结果**:抗dsDNA抗体结果印刷为 `<2.00`(低于检出限),float 无法
   承载,曾导致该值误塞进 `ref_range` 位并挤掉参考列真值 `0-30`(ana_02 / webwxgetmsgimg_1);
   尿镜检 `70-80`、`1-2` 范围型结果同样无处落位。而真值基准 `samples/expected/vl/<id>.json`
   (LLM 结构化)的 value 本就是**印刷原样字符串**,float 契约与之口径不一致;
2. **method 列无处落位**:免疫/生化类检查单带「检测方法/实验方法」列(如抗核抗体谱
   「流式荧光发光法」),方法不同参考范围与可比性都不同;该列的格要么被跳过、要么被
   当作定性值误吸入 value。本机 239 个指标项实测 120 个印有 method 列,属真实数据
   驱动的高价值字段。

## 做什么

- `ReportItem.value`:`float | null` → **`string | null`**(印刷原样,如 `'128'`、`'0.43'`、
  `'<2.00'`、`'70-80'`;无结果/解析失败仍为 null)。flag 语义不变:数值比较(需
  `_num`/`ParseFloat`);不可数值化(比较式/范围串)→ unknown,不猜;
- `ReportItem` 新增 **`method: string | null`**:检测方法/实验方法列印刷原样
  (如 `'化学发光法'` `'流式荧光发光法'`);无该列或解析失败为 null;
- 状态策略(同周收敛):low_confidence **不再压整单 partial**;partial 仅由 value 缺失、
  日期缺失,或 **低置信占比 ≥ 50%**(过半指标不可靠,降级人工甄别)触发;
- 涉及:schema/pydantic 双语契约、postprocess 抽取、Go contract/pipeline/storage 迁移
  (0002: value 列 DOUBLE PRECISION → TEXT;0003: 新增 method TEXT)、trend delta
  (ParseFloat 失败不计差值)、CLI 输出、gt_eval 数值/字符串双口径,以及 samples/
  本机 data freeze 的 report golden 全量再生成。

## 怎么实施

见本分支 commit。要点:

1. **py `_decompose`** 返回 `(str|None, str|None, str|None)`:值列纯比较式(如 `<2.00`)
   拆不出数值时**进 value 位、不进 ref 位**(修复 ref 污染);熔断式(`38.201<8`)首部
   数值仍作 value、比较式部分仍作 ref;复合表头列(如「结果判定参考范围」)豁免;
2. **method 列角色**:`ROLE_HEADERS` 注册 `method` 角色(表头词「检测方法/实验方法/方法」,
   长词在前);`_RowPiece` 加 `method` 槽,`absorb` 先到先得;role 循环顺序
   `"value" → "method" → "unit" → "ref"`;方法格由 `_method_value` 守卫(≤30 字、含尾注
   括号、非列角色关键词、非日期形),**复用既有 (role, blk) 分格与碎片配对流水线,
   只插 role、不加新层**;
3. **Go 存储**:迁移 `0002_item_value_to_text.sql`(Down 仅回转纯数值行)与
   `0003_item_method.sql`(可 Down DROP);`store.go` 的 `sqlItemsByReport` 投影 /
   `ItemsByImage` 扫描 / `sqlItemInsert` 三处同步加列;
4. **CLI**:`hdd show` 每项打印 `value/unit/ref_range/method` 印刷原样(null → `-` 占位);
   `trend` delta 用 `ParseFloat`,解析失败 → `-`。

## 怎么验收

- **py**:全量 `pytest` 全绿(golden 掩码条目跳过为预期;golden 工具与测试共享
  `date_hint_for` 口径);状态策略单测:低置信占比 <50% → success、恰 ≥50% → partial、
  value 缺失 → partial;
- **golden**:`samples/expected/report/*.json` 与本机 `data/expected/report` 按新契约
  再生成,`check --ocr` 24/24 unchanged;
- **效果指标**:`local_golden.py score` matched 230/236、value 206、unit 189、ref 194
  ——value 字段与 method 均为**纯新增/等价迁移,不挤占既有格**(ref 190→194 修复保留);
  vok 口径说明:GT 真值表把检出限记 `''`、pred 如实给出 `'<2.00'` 的 4 个对子不再计
  value 一致分——属真值口径差异,非抽取劣化,不改禁改的 vl 真值;
  method 实拍覆盖 120/239 项;
- **Go**:`go vet` + `go test ./...` 全绿;contract testdata(`report.example.json` 带
  `value: "<2.00"`、`method`)与 py/schema 三方互证通过(双语对齐同一 schema)。


## 为什么做

免疫/生化类检查单带「检测方法/实验方法」列(如 抗核抗体谱的「流式荧光发光法」、
化学发光项目说明配套试剂方法),方法不同参考范围与可比性都不同,但旧契约没有落位——
该列的格要么被跳过、要么被当作定性值误吸入 value(用「 method 列角色关键词」守卫前,
`化学发光法` 一度能进 value 位)。本机 239 个指标项实测 120 个印有 method 列,
属真实数据驱动的高价值字段。

## 做什么

- `ReportItem` 新增 `method: string | null`:检测方法/实验方法列**印刷原样**
  (如 `'化学发光法'` `'流式荧光发光法'`);无该列或解析失败为 null。
- 涉及:py contract(schema/pydantic)、postprocess 抽取、report.schema.json、testdata;
  Go contract / 迁移 0003(report_items 加 `method TEXT`)、store SQL、CLI `show` 输出;
  samples 与本机 data freeze 的 report golden 全量再生成。

## 怎么实施

见本分支 commit(单需求单 commit)。要点:

1. **py postprocess**:`ROLE_HEADERS` 注册 `method` 角色(表头词「检测方法/实验方法/方法」,
   注意「方法」是后两者的公共子串,排序按最长前缀优先);`_RowPiece` 加 `method` 槽,
   `absorb` 扩为 5 参、先到先得不覆盖;方法格由独立守卫 `_method_value` 判定
   (去箭头后 ≤30 字、含尾注括号、非列角色关键词、非日期形),**复用既有
   `(role, blk)` 分格与碎片配对流水线,只插 role、不加新层**:
   - role 循环顺序 `("value", "method", "unit", "ref")`——method 在 unit 之前,
     使方法格碎片(如「流式荧光发光法」)优先归 method 位,不去污染 unit/ref;
   - 碎片熔断对 method 列不适用(v/r/u 判 `qual is None` 才进),方法格在
     `v/r/u` 全 None 时走 `qual` 直接落 method 位;
2. **双语契约**:`schemas/report.schema.json` 与 pydantic `contract.py` / Go
   `internal/contract` 同步新增字段(印刷原样 string,null 缺省,`additionalProperties: false`;
   schema 为源,py/Go structs 与之互证防漂移);
3. **Go 存储**:迁移 `0003_item_method.sql`(`ALTER TABLE report_items ADD COLUMN method TEXT`,
   可 Down 反向 DROP);`store.go` 的 `sqlItemsByReport` 投影 / `ItemsByImage` 扫描 /
   `sqlItemInsert` 插入三处同步加列;
4. **CLI**:`hdd show` 每项追加 `method=<印刷原样|null → ->`,未填一律 `-` 占位。

## 怎么验收

- **py**:全量 `pytest` 152 passed / 25 skipped(golden 掩码条目跳过为预期;
  收敛前一轮需先重生成 golden——`python -m recognizer.golden` 与
  `python tools/local_golden.py freeze` 的 date_hint 口径已在 `test_report_matches_golden`
  / `test_reparse_golden_matches_postprocess` 与 golden 工具共享同一 `date_hint_for`);
- **golden**:`samples/expected/report/*.json` 全量按新契约(method 字段)再生成;
  本机 data freeze 后 `check --ocr` **24/24 unchanged**(含 OCR 无漂移,报告重算一致);
- **效果指标**:`local_golden.py score` matched 230/236、value 206、unit 189、ref 194
  ——与 method 交付前基线**持平**(method 为纯新增位,不挤占既有格);
  method 实拍覆盖 120/239 项(免疫/生化可采条目为主);
- **Go**:`go build`、`go vet`、`go test ./...` 全绿;contract testdata
  `report.example.json` 带 `method` 与 py/schema 三方互证通过。
