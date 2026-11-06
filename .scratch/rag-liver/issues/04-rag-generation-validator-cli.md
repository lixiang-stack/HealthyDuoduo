# 04: RAG 生成 + claim 校验 CLI

**What to build:** 一个 CLI 对成人肝功 case 检索、调用 flash 模型、输出 claims（text / quote / chunk_id / source_id / evidence_ids / standard_check）；服务端校验结构、引用、quote 逐字子串、数字/方向、内容边界、standard_check 一致性。伪造 quote、数字反向、药物/剂量、预后、个体化复查、证据不完整的标准判断都会被拒绝并给出原因。验证正常 case 出四段 claims、对抗 case 逐条拒绝。

**Blocked by:** 03 (语料 manifest + 离线 ingestion + 检索 CLI).

**Status:** ready-for-agent

- [ ] prompt 明确：只使用检索 chunk；每条 claim 带逐字 quote；禁止治疗/药物/剂量/预后/个体诊断/个体化复查；缺证据写「无法判断」。
- [ ] 输出 JSON 符合 spec §8.2；非法 JSON、空 claims、超长/超量有明确降级原因。
- [ ] validator 覆盖：chunk 存在、quote 子串、数字/方向、禁词/禁句式、standard_check 一致性。
- [ ] 单条 claim 失败只丢该条；全失败返回明确原因；不抛出未处理错误。
- [ ] CLI 打印 claims、来源和每条校验结果；对抗 case 有测试。
- [ ] 不改变现有 HTTP / App 行为。
