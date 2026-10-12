# 2026-10-12 检查项 value 字段 string 契约

## 为什么做

原契约 `ReportItem.value` 为 `float | null`,带来两类真实样本实测问题(review 会话发现):

1. **检出限比较式丢失**:抗dsDNA抗体结果印刷为 `<2.00`(低于检出限),float 无法承载,
   曾导致该值被误塞进 `ref_range` 位并挤掉参考列真值 `0-30`(ana_02 / webwxgetmsgimg_1);
2. **范围型/定性结果无法表达**:尿镜检结果 `70-80`、`1-2` 这类范围值,float 只能整行丢弃 value。

而真值基准 `samples/expected/vl/<id>.json`(LLM 结构化)的 value 本就是**印刷原样字符串**
(如 `"0.43"`、检出限记 `""`),float 契约与之口径不一致。

## 做什么

- `ReportItem.value`:`float | null` → **`string | null`**(印刷原样,如 `'128'`、`'0.43'`、
  `'<2.00'`、`'70-80'`;无结果/解析失败仍为 null)。
- flag 语义不变:数值比较(需 `_num`/`ParseFloat`);不可数值化(比较式/范围串)→ unknown,不猜。
- 涉及:py contract(schema/pydantic)、postprocess 抽取(_decompose/_match_name/值比较式落 value 位)、
  report.schema.json、testdata;Go contract/pipeline/storage 迁移(0002: DOUBLE PRECISION → TEXT,
  Down 仅回转纯数值行)、trend delta(ParseFloat 失败不计差值)与 CLI 输出、gt_eval 数值/字符串双口径。

## 怎么实施

见本分支 commit(单需求单 commit)。要点:

1. py `_decompose` 返回 `(str|None, str|None, str|None)`;值列纯比较式(如 `<2.00`)拆不出数值时
   **进 value 位、不进 ref 位**(修复 ref 污染);熔断式(`38.201<8`)首部数值仍作 value、比较式部分仍作 ref;
2. 复合表头列(如「结果判定参考范围」)豁免:表头文本含 ref 关键词的 value 列允许 r 落 ref 位;
3. Go `report_items.value` 列 `DOUBLE PRECISION → TEXT`(migration 0002);
   `trend` delta 用 `ParseFloat`,解析失败 → `-`。

## 怎么验收

- 全部单元/集成测试通过(py 152+ / go vet+test);
- `samples/expected/report/*.json` 与本机 `data/expected/report` golden 已按新契约再生成,
  `check --ocr` 24/24 unchanged;
- `local_golden.py score`:matched 230/236、uok/rok 不变(ref 190→194 修复保留);
  vok 口径变化:GT 真值表把检出限记 `''`、pred 现在如实给出 `'<2.00'` 的 4 个对子不再计 value 一致分
  (ana_02/not·webwxgetmsgimg_1、_4、_9、_10 各 1 项)——属**真值口径差异**,非抽取劣化;
  该口径差异已在本文件说明,不改禁改的 vl 真值。
- Go `contract_test`/`client_test`/`main_test` 与 testdata 契约互证通过(双语对齐 schema)。
