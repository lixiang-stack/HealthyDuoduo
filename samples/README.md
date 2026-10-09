# samples · 样本库

- 只收**脱敏**图像(NF-01);未脱敏原图永不入库、永不提交本目录。
- 目录约定(实施计划 §7-3):
  - `samples/<id>.jpg`
  - `samples/expected/ocr/<id>.json`(P1 golden)
  - `samples/expected/report/<id>.json`(P2 golden)
- 当前 5 张真实脱敏血常规样本 cbc_01~05(2026-10 入库,原 sim_cbc_01/02/03 占位仿真单已删除)。
  其中真实样本以维护者确认的脱敏程度为准(NF-01);**cbc_03 有意保留为「无检验日期」版式**,
  用于 date-null / partial 场景回归。
- golden(`expected/ocr/<id>.json`)由调试 CLI 生成:
  `cd recognizer && uv run python -m recognizer ../samples/<id>.<ext> > ../samples/expected/ocr/<id>.json`
  (stderr 的日志重定向丢弃;对比规则见实施计划 §8-5,elapse 只记录不比对。)
