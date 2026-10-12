# 2026-10-12 检查项 method 字段(检测方法/实验方法列)

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
