# samples · 样本库

- 只收**脱敏**图像(NF-01);未脱敏原图永不入库、永不提交本目录。
- 目录约定(实施计划 §7-3):
  - `samples/<id>.jpg`
  - `samples/expected/ocr/<id>.json`(P1 golden)
  - `samples/expected/report/<id>.json`(P2 golden)
- `sim_` 前缀 = 系统生成的仿真单。当前 3 张(sim_cbc_01/02/03)为占位;
  **P1 起需补齐 ≥5 张真实脱敏血常规样本**(P0 验收豁免条款,计划文件修订记录有注)。
  其中 sim_cbc_03 被有意做成**不含检验日期**,用于 date-null / partial 场景回归。
