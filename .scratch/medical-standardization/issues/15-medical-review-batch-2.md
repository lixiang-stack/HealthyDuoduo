# 15: 医学复核 batch 2

**What to build:** 医学复核 batch 2（86 项：尿常规、免疫、专项、血型）。逐项确认标准名/码或 local_code、`result_kind`、单位/替代单位、定性/定序参考范围；重点裁决 `S/CO`、`AU`、滴度、`/HP`、`个/uL` 的建模。复核通过后翻 `review_status=approved` 并重生成 golden。

**Blocked by:** 09–11（batch 2 各 panel 标准输出）

**Status:** ready-for-human

- [ ] 86 项逐项有复核记录与来源；无来源项不得 approved。
- [ ] `S/CO`、`AU`、滴度、`/HP`、`个/uL` 均有明确 result_kind/单位表达，不出现伪单位。
- [ ] 定性/定序项的 `ref_kind` 与 flag 语义确认。
- [ ] approved 项重生成 golden；`unit_status=verified` 只在 approved 后出现。
