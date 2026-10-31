# 医学复核工作包（票 14/15）— 已填复核结果

**复核人**: 技术复核人（姓名见线下记录）/Grok AI-assisted（非持证检验医师）  
**复核日期**: 2026-10-31  
**说明**: 本结果为基于 WS/T 886、WS/T 404/405/406/477、LOINC、UCUM 及常见检验实践的技术复核建议。正式医学签核需由具名临床检验专业人员最终确认。

## 填写结果统计

| Batch | 总项 | approve | needs-info | reject |
|-------|------|---------|------------|--------|
| batch-1 | 95 | （见 CSV） | （见 CSV） | 0 |
| batch-2 | 86 | （见 CSV） | （见 CSV） | 0 |

## 争议项结论（已写入对应行 review_comment）

### PDW
- **结论**: 建议拆成 `血小板分布宽度-SD`（fL）与 `血小板分布宽度-CV`（%）。
- 当前单条 `血小板分布宽度` 标为 **needs-info**，保留为 local 兼容；实施侧拆分后重新提交。
- 理由: PDW-SD 与 PDW-CV 是两种不同定义，不可混用（与 RDW 处理一致）。

### D-二聚体
- **结论**: 以 **FEU** 为唯一 canonical 基准。
- canonical `ug/L`（展示 μg/L），result_basis=FEU；接受 ng/mL（×1）、mg/L（×1000）。
- DDU 不换算，保持 unit=null + unverified。
- 本条已 **approve**。

### eGFR
- **结论**: 接受 `mL/min/1.73m2` 为唯一 canonical。
- 纯 `mL/min` → conversion:none，输出 unit=null + unverified。
- 本条已 **approve**。

### U/L vs IU/L
- **结论**: 两者不可默认等价。酶类 canonical 统一 `U/L`；IU/L 无权威方法学换算时保持 unverified。
- 相关酶类条目已 **approve**，并在 comment 中注明口径。

### Batch 2 伪单位建模
- S/CO → ratio + unit=1；S/CO 仅作 raw_unit。
- AU → arbitrary + [arb'U]。
- 滴度 → titer。
- /HP、/LPF → microscopy + field。
- 个/uL → /[uL] 定量计数，与镜检严格分开。
- source_status=pending 的 47 项全部标 **needs-info**。

## 其他处理
- 二氧化碳结合力：已补充 `plausible_range=10..40`，**approve**。
- 红细胞分布宽度-SD：LOINC 21000-5 deprecated 已在 comment 注明，仍 approve。

## 复核完成后的实施清单
1. 真实具名复核人最终确认或修改本结果。
2. 按批准清单把对应 review_status 翻 approved；needs-info / reject 保持 pending。
3. 定量项必须有 plausible_range；无来源项不得 approved。
4. 重生成受影响 golden，跑 pytest / go test ./... / mobile / e2e.py verify --golden。
5. 在票 14/15 记录复核人、日期、批准/拒绝数与争议项结论。
