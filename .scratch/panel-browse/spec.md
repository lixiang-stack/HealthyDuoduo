# Spec：项目组分类与按项目浏览（panel-browse）

Status: ready-for-agent（2026-10-27 需求分析与原型选型定稿）

**范围：** 识别服务指标项级项目组（`panel`）+ 报告级项目组聚合（`panels`）；Go 持久化 / API / 存量回填；App 家庭首页、双 Tab、档案「按时间 / 按项目」、项目详情、报告详情按项目分组、异常指标页改按项目分组。不含诊断、建议、复查提醒。

**依据：** `CONTEXT.md`（报告 / 报告类别 / 指标项 / 成员 / 家长 / 核对）· `docs/design/2026-10-13-viz-and-client-spec.md`（v1.3，成员主轴与 M3）· `.scratch/member-overview/spec.md` · `.scratch/abnormal-trends/spec.md` · `docs/adr/0005-review-final-value-and-correction-pairs.md` · `docs/adr/0011-report-item-not-tested-state.md` · 现有实现 `recognizer/recognizer/*`、`internal/{pipeline,storage,httpapi}/*`、`apps/mobile/src/app/*`。医学依据与参考文档见 §5.5。

**替代：** `docs/design/2026-10-26-app-ia-and-pathology-tags.md`（v1）与 `docs/design/2026-10-26-app-ia-and-pathology-tags-v2.md`（v2.2）。本 spec 定稿后删除这两篇；下文「与现有文档的关系」保留其结论的取舍轨迹。

## 目录

**主路径（先读）：** §0 决策摘要 · §2 目标与成功标准 · §8 App 信息架构 · §9 怎么实施 · §14 决策记录

**Reference（按需查）：** §1 为什么做 · §3 原则 · §4 领域模型 · §5 项目组分类（§5.5 医学依据）· §6 识别服务 · §7 数据与契约 · §10 怎么验收 · §11 非目标 · §12 风险与假设 · §13 与现有文档的关系

- 0. 决策摘要
- 1. 为什么做
- 2. 目标与成功标准
- 3. 原则
- 4. 领域模型与统一语言
- 5. 项目组分类
- 6. 识别服务
- 7. 数据与契约
- 8. App 信息架构与页面
- 9. 怎么实施（三单）
- 10. 怎么验收
- 11. 非目标（本期不做）
- 12. 风险与假设
- 13. 与现有文档的关系
- 14. 决策记录（2026-10-27）

---

## 0. 决策摘要

| # | 决策 | 结论 |
|---|------|------|
| 1 | 分类粒度 | **指标项级项目组 `panel`**；报告级 `panels` 是项目组的排序聚合，`panels[0]` 只用于卡片主标签。不是报告级单标签，也不是报告级多标签。 |
| 2 | 分类位置 | **识别服务**决定每个指标项的 `panel` 与报告级 `panels`；Go 只持久化 / 聚合 / 返回，不维护「指标名 → 项目组」映射。 |
| 3 | 浏览模型 | 档案页「按时间 / 按项目」分段；按项目 → 项目列表 → 项目详情（异常项内联趋势，正常项最新值 + 点开趋势）。 |
| 4 | 导航 | **双 Tab：家庭 / 档案**；上传用档案页 FAB；设置进 header；异常指标保留为 Stack 页；报告详情 / 核对 / 项目详情 / 成员管理 / 设置都是 Stack。 |
| 5 | 分类集合 | 12 + `sex_hormone` / `vitanemia` / `coag` + `other`，共 15 个具名项目组 + `other`；UI 只展示当前成员实际出现过的项目组。 |
| 6 | 术语 | **项目组（panel）**；UI 用「按项目」；不再用「病理标签 / 系统标签 / 检查大项」。 |
| 7 | 家庭卡 | **项目口径**：待核对 + 最近报告主项目 + 异常项目（按项目最近值汇总）+ 最近报告偏高/偏低 + 报告总数。 |
| 8 | 报告卡 | 主项目 + 次项目（最多 2 个）+ `+N`；失败回退 `report_type`。 |
| 9 | 项目详情 | 异常项内联迷你趋势；正常 / 未知 / 未检测项给最新值与入口；底部「包含该项目的报告（N 份）」→ `/timeline?panel=code`。 |
| 10 | 异常指标页 | 保留为 Stack 页，分组从 `report_type` 改为项目；从档案「按项目」的「全部异常 N 项」进入。 |
| 11 | 交付 | 三单：① 识别 + 数据 + API + 回填；② App 项目视图；③ IA 壳（家庭首页 + 双 Tab）。 |
| 12 | 存量回填 | 新增识别服务 `POST /classify`（report_type + 当前指标名 → 项目组），`hdd backfill-panels` 只更新 panel 列，不重写指标项 / 核对状态。不用 `/reparse` 回填。 |
| 13 | ADR | 新增 `docs/adr/0012-item-level-panels.md`。 |

**原型（一次性，未入库到产品代码）：** `.scratch/panel-browse/prototype/prototype.html` + `prototype/shots/`。联系表见 `shots/00-contact-sheet.png`。

---

## 1. 为什么做

### 1.1 两篇旧文档的诊断与冲突

v1 与 v2.2 都指向两个问题：信息架构偏离成员主轴、分类维度不符合家长心智。两篇的结论有重叠，也有互相撤销：

| 决策 | v1 | v2.2 | 本 spec |
|------|----|------|---------|
| 一份报告几个分类 | 多标签 | 单标签 | 指标项级项目组；报告级 `panels` 聚合 |
| 分类在哪做 | 偏展示层，未定 | 识别服务 | 识别服务（指标项级） |
| 导航 | 4 Tab | 2 Tab | 2 Tab + Stack |
| 时间线分类 | chips 替代 `report_type` | 标签列表 | 按时间 / 按项目分段 |
| 项目详情 | 未展开 | 最近一份报告 + 异常项趋势 | 跨报告指标视图 + 异常项内联趋势 |
| 儿童场景 | 未特化 | 12 + 其他（成人基础体检） | 补性激素 / 维生素/贫血 / 凝血 |

**保留 v2.2 的：** 成员主轴、双 Tab、设置进 header、上传 FAB、家庭卡片首页、分类在识别服务、稳定 code + 中文 label、文案中性。
**撤销 v2.2 的：** 报告级单标签、标签详情只看最近一份报告、`/trends` 退役、12 + 其他固定集合。
**撤销 v1 的：** 报告级多标签、4 Tab、时间线 chips。

### 1.2 事实核查

1. **当前时间线并没有按 `report_type` 分类。** `/timeline` 按日期倒序平铺；真正按 `report_type` 分组的是首页「各类最近一次」和 `/trends`。要修的不是「时间线分类」，而是「缺少按项目浏览 / 跨报告聚合」。
2. **当前 IA 的问题比 v1 说的轻。** `/` 已是当前成员概览，设置只是底部链接，上传只是链接；没有 Tab，但没有「所有功能挤在同一页」。真正缺的是家庭首页、设置/上传归位、项目视图。
3. **`GET /reports` 摘要不含指标名。** 客户端无法从摘要派生项目组；项目组必须由识别服务随报告产出，数据服务持久化 / 返回。
4. **单标签在真实样本上会漏掉一半分类。** 49 份 golden 按「命中指标项数最多」模拟单标签：

| 标签 | 单标签结果 | 实际含该项目的报告 |
|------|------------|--------------------|
| 肾功 `kidney` | **0 份** | 3 份（lft_02 / lft_08 / lft_09） |
| 血脂 `lipid` | **0 份** | 1 份（lft_06） |
| 尿酸 `uric_acid` | **0 份** | 2 份（lft_02 / lft_08） |
| 电解质 `electrolyte` | **0 份** | 1 份（lft_08） |
| 肝功 `liver` | 9 份 | 9 份（生化 15 份里 60% 归肝功） |

也就是说，家长点「肾功」会看不到任何报告，但孩子的肌酐、尿素就在 lft_02 里。这是报告级单标签的必然结果，不是边缘情况。

5. **单标签的项目详情只看最近一份报告，回答不了「有没有反复」。** 正常指标没有跨报告历史；异常项只有最近一份报告的异常项有条形趋势。家长要回答「甲状腺最近怎么样」，仍然要逐份点开。
6. **影像 / 失败 / 混合报告需要不同的兜底。** 影像单没有指标项；失败报告没有可归类项；混合生化报告同时包含肝功 / 肾功 / 尿酸 / 电解质。报告级单标签无法同时表达这些。

### 1.3 方案选型

| 方案 | 做法 | 结论 |
|------|------|------|
| A 报告级单标签（v2.2） | 识别服务给每份报告一个 `pathology_tag` | 否：上面四个项目在样本里挂零；项目详情无跨报告历史 |
| B 指标项级项目组 + 项目视图 | 每个指标项一个 `panel`；报告级 `panels` 聚合；App 按项目 browse | **采纳** |
| C 报告级多标签 | `pathology_tags[]`，混合报告进多个标签 | 否：不解决项目内趋势；若按标签过滤指标，仍要指标级标签 |
| D 不做分类，只修 IA | 保留 `report_type`，只做家庭首页 / 双 Tab | 否：没有回答「按肝功 / 甲状腺找」 |

**一句话：** 家长要的不是「这份报告属于哪个标签」，而是「这个项目最近的指标怎么样、有没有反复」。所以分类必须落到指标项，报告级只做聚合。

---

## 2. 目标与成功标准

### 2.1 目标

1. 家长能按自己熟悉的项目（肝功、甲状腺、血常规、肾功、尿常规…）看到某个成员该项目全部指标的最新值与跨报告趋势。
2. 混合报告在它包含的每个项目下都能被找到；一份 lft_02 同时出现在肝功、肾功、尿酸。
3. 家庭首页第一屏回答「家里谁现在有什么要处理」；进入某成员后，第一屏明确「当前在看谁的档案」。
4. 设置离开内容区；上传不占一级导航；异常指标仍有一个横切入口。
5. 分类只在识别服务；Go 不做「指标名 → 项目组」映射；存量报告可在不破坏核对状态的前提下回填。

### 2.2 成功标准（可验收）

- 4/5 家长在 5 份真实报告上，无需帮助能在 4 次点按内从家庭首页找到肾功 / 肝功 / 甲状腺结果（Phase 2 后测；Phase 1 可在档案页测）。
- 识别服务对每条报告输出 `panels`，对每个指标项输出 `panel`；`panels` 与指标项面板集合一致，`panels[0]` 为命中项数最多的项目。
- 每个指标项的 `panel` 属于其 `report_type` 的允许集合（`other` 例外）；尿常规的「葡萄糖 / 胆红素 / 蛋白质」不会让报告进入血糖 / 肝功；影像成功报告为 `imaging`。
- 项目列表只出现当前成员实际有报告的项目；没有空分类；混合报告在多个项目下出现。
- 项目详情展示该项目全部指标的最新值与跨报告趋势；异常项内联趋势，正常项可点开趋势；不隐藏同报告的其他项目指标。
- 时间线卡片主展示为日期、项目名、状态、异常摘要；`report_type` 降为次要信息或失败 / 未分类回退。
- 家庭首页卡片展示待核对、最近项目、异常项目、最近报告偏高/偏低、报告总数；3 秒内能说出当前卡片属于谁。
- 现有核对 / 信任 / 趋势 / 上传 E2E 全绿；家庭首页在 LAN 上 2–5 成员、20–50 份/年/成员时首屏 <2s。

---

## 3. 原则

| 原则 | 说明 |
|------|------|
| 分类落到指标项 | 识别服务给每个指标项一个 `panel`；报告级 `panels` 是聚合，不是另一套分类 |
| 报告级只做排序 | `panels[0]` 是卡片主标签；报告可以属于多个项目，不强行「一报告一标签」 |
| 识别服务定分类 | Go 不维护「指标名 → 项目组」映射；回填也走识别服务 `/classify` |
| 项目是浏览入口，不是过滤器 | 项目详情展示该项目全部指标；报告详情始终展示报告全部指标，不按项目隐藏 |
| 家长心智优先 | UI 用肝功 / 肾功 / 甲状腺 / 血常规等报告封面词；code 用稳定英文 |
| 数据驱动 | 项目列表只出现当前成员实际出现过的项目；不显示空分类 |
| 单位门控不变 | 趋势连接键 = 指标名 + 归一单位；跨单位不连线、不换算；项目详情按 name+unit 分行 |
| 信任优先 | 项目详情 / 卡片保留低置信空心点、待核对徽标；不隐藏失败报告；失败不进项目列表 |
| 文案中性 | 不出现诊断、建议、复查提醒、风险类表述；「项目组」不代表疾病 |
| 最小实现 | 三个可独立验证 / 回滚的交付单；不新增前端依赖；不做搜索 / 年份分组 / 多页合并 |

---

## 4. 领域模型与统一语言

**项目组（panel）：**
一组围绕同一检查目的、通常在同一张检查单上一起出具的指标项集合（如肝功、肾功、甲状腺、血常规）。报告中每个指标项恰好属于一个项目组；一份报告可含多个项目组。用于家长按熟悉项目浏览与跨报告趋势；不是报告类别（`report_type`），不是单个指标项，也不代表疾病诊断。代码 / 契约用 `panel`，UI 用「按项目」。
_Avoid_: 病理标签、系统标签、检查大项、分类标签

**报告：**
沿用既有定义：一个图像对应一条报告。新增：报告含一组指标项时，其项目组集合为 `panels`（按命中项数降序，同数按固定优先级）；`panels[0]` 是卡片主标签。报告可以同时属于肝功 / 肾功 / 尿酸；`panels` 不限制报告的浏览入口。

**报告类别：**
沿用 `CONTEXT.md` 的既有定义，对应 `report_type`（临检 / 生化 / 激素 / 免疫 / 影像 / 其他专项）；继续服务识别管道、契约与测试。UI 中降为日期行小字或失败 / 未分类回退，不再作为家长主分类。

**指标项：**
沿用既有定义；新增：每个指标项恰好属于一个 `panel`。指标项仍是核对、趋势、异常计数的最小单位；不存在「项目组级核对」或「项目组级修正」。

**项目组与 `report_type` 的关系：**
项目组是 `report_type` 下的细分（临检 → 血常规 / 尿常规 / 凝血；生化 → 肝功 / 肾功 / 血脂 / 血糖 / 尿酸 / 电解质 / 维生素/贫血；激素 → 甲状腺 / 性激素；免疫 → 感染筛查 / 自身抗体；影像 → 影像）。约束是**逐指标项**的：一个指标项的 `panel` 必须属于其词典类别的允许集合；`other` 是唯一允许跨类别的兜底。报告本身不要求只有一个父类，因为混合生化报告可以同时包含肝功、肾功、尿酸。

**与既有词的关系：**
- `CONTEXT.md` 的「报告类别 / `report_type`」不变；新增「项目组」词条。
- `CONTEXT.md` 的「指标项」补一句「属于一个项目组」；「报告」补一句「可含多个项目组」。
- 不使用 v1 / v2.2 的「病理标签」；「检查大项」继续是 Avoid（易与单个指标项混淆）。

---

## 5. 项目组分类

### 5.1 父子约束（逐指标项）

| `report_type`（词典类别） | 允许的 `panel` | UI label |
|---------------------------|----------------|----------|
| 临检 | `cbc` / `urinalysis` / `coag` / `other` | 血常规 / 尿常规 / 凝血 / 其他 |
| 生化 | `liver` / `kidney` / `lipid` / `glucose` / `uric_acid` / `electrolyte` / `vitanemia` / `other` | 肝功 / 肾功 / 血脂 / 血糖 / 尿酸 / 电解质 / 维生素/贫血 / 其他 |
| 激素 | `thyroid` / `sex_hormone` / `other` | 甲状腺 / 性激素 / 其他 |
| 免疫 | `infection` / `autoimmune` / `other` | 感染筛查 / 自身抗体 / 其他 |
| 影像 | `imaging` / `other` | 影像 / 其他 |
| 其他专项 | `other` | 其他 |
| `unknown` / failed | `other` | 其他 |

- 每个指标项的 `panel` 必须在所属 `report_type` 的允许集合内；`other` 是唯一跨类别兜底。
- 不允许「`report_type=生化` + `panel=thyroid`」这类越界；识别端契约测试与 Go 契约测试双重校验。
- 影像成功报告没有指标项，报告级 `panels = [imaging]`；失败报告 `panels = [other]`。
- 混合报告按指标项自然分布：`lft_02` 的 `panels = [liver, kidney, uric_acid]`，不是「只能选肝功」。

### 5.2 项目组与覆盖

| code | UI | 父类 | 覆盖 | 备注 |
|------|----|------|------|------|
| `cbc` | 血常规 | 临检 | 白细胞、红细胞、血红蛋白、血小板、分类计数等 | 基础必查 |
| `urinalysis` | 尿常规 | 临检 | 尿蛋白、潜血、尿糖、尿胆红素、比重、镜检等 | 与血糖 / 肝功的歧义靠父类区分 |
| `liver` | 肝功 | 生化 | ALT、AST、GGT、ALP、胆红素、总蛋白 / 白蛋白、胆汁酸、胆碱酯酶、前白蛋白、AFP 等 | AFP 在 WS/T 645.2—2026 中与肿瘤标志物同组；本 spec 首版不设肿瘤标志物项目组，AFP 随报告归肝功，是产品偏离（见 §5.5.4） |
| `kidney` | 肾功 | 生化 | 尿素、肌酐、eGFR、血清胱抑素C | 尿酸独立 |
| `lipid` | 血脂 | 生化 | TC、TG、HDL-C、LDL-C、载脂蛋白等 | 基础 / 慢病筛查高频 |
| `glucose` | 血糖 | 生化 | 空腹 / 餐后血糖、糖化血红蛋白、C肽、胰岛素 | 不单独拆「糖化」 |
| `thyroid` | 甲状腺 | 激素 | TSH、FT3、FT4、T3、T4、TPOAb、TgAb、TRAb、甲状腺球蛋白 | 激素类高频 |
| `sex_hormone` | 性激素 | 激素 | 孕酮、HCG、雌二醇、睾酮、FSH、LH、泌乳素等 | 儿童性早熟 / 青春期随访高频 |
| `uric_acid` | 尿酸 | 生化 | 血尿酸 | 与肾功分开 |
| `electrolyte` | 电解质 | 生化 | K、Na、Cl、Ca、P、Mg、CO2 等 | 生化常一起出具 |
| `vitanemia` | 维生素/贫血 | 生化 | 25-OH-VD、铁蛋白、叶酸、维生素B12 等 | 儿童维生素 D / 贫血随访高频 |
| `infection` | 感染筛查 | 免疫 | 乙肝五项、丙肝抗体、梅毒、HIV 等 | 体检常见传染病筛查 |
| `autoimmune` | 自身抗体 | 免疫 | ANA、ENA、ANCA、自免肝抗体等 | 样本里 ANA 报告不少 |
| `coag` | 凝血 | 临检 | PT、APTT、TT、FIB、D-二聚体、FDP 等 | 术前 / 出血倾向随访 |
| `imaging` | 影像 | 影像 | 超声 / 影像（无指标项） | 不加会把影像单退化成「其他」 |
| `other` | 其他 | 任意 | 血型、肝纤维化、产前筛查、未归类 | 唯一跨类别兜底；UI 一行，显示来源报告类别 |

**首版不做独立项目组的：** 肿瘤标志物（AFP 归肝功；WS/T 645.2—2026 将 AFP 与 CEA/CA19-9 同组，本 spec 的产品偏离见 §5.5.4）、血型（一次性事实）、肝纤维化（归其他）、产前筛查（归其他）、微量元素 / 过敏原 / 心肌酶（归其他）。有足够真实报告再拆。

### 5.3 排序与主标签

- 报告级 `panels` 排序：**命中指标项数降序**；同数按固定优先级：
  `liver` < `kidney` < `lipid` < `glucose` < `uric_acid` < `electrolyte` < `vitanemia` < `thyroid` < `sex_hormone` < `infection` < `autoimmune` < `coag` < `cbc` < `urinalysis` < `imaging` < `other`（数值小优先）。
- `panels[0]` 是卡片主标签；报告卡展示前 2 个，超过 2 个显示 `+N`。
- 项目列表排序：最近报告日期降序（日期未知沉底），同日期按上述优先级。
- 项目详情指标排序：最新值为 high / low / abnormal 的在前；其余按最新报告内指标顺序；同序按名称稳定排序。

### 5.4 歧义与边界

| `report_type` | 指标规范名 | `panel` | 说明 |
|---------------|------------|---------|------|
| 临检 | 葡萄糖 | `urinalysis` | 尿糖；不能进血糖 |
| 生化 | 空腹葡萄糖 / 餐后葡萄糖 / 糖化血红蛋白 | `glucose` | 血糖 |
| 临检 | 胆红素 / 蛋白质 | `urinalysis` | 尿胆红素 / 尿蛋白 |
| 生化 | 总胆红素 / 直接胆红素 / 总蛋白 / 白蛋白 | `liver` | 肝功 |
| 激素 | 促甲状腺素 / 游离甲状腺素 / TPOAb / TRAb | `thyroid` | 含抗体 |
| 激素 | 孕酮 / HCG | `sex_hormone` | 性激素 |
| 生化 | 甲胎蛋白 | `liver` | 首版不设肿瘤标志物 |
| 生化 | 25-OH-VD / 铁蛋白 / 叶酸 / B12 | `vitanemia` | 维生素/贫血 |
| 免疫 | 乙肝表面抗原 / 丙肝抗体 / 梅毒螺旋体抗体 / 艾滋病抗体抗原 | `infection` | 感染筛查 |
| 免疫 | ANA / 抗 dsDNA / 抗线粒体抗体 | `autoimmune` | 自身抗体 |
| 免疫 | 透明质酸 / 层粘连蛋白 / 三型前胶原N端肽 / 四型前胶原 | `other` | 父类是免疫；首版不拆肝纤维化 |
| 临检 | ABO血型 / RH血型 | `other` | 一次性事实 |
| 影像 | 无指标项 | `imaging` | 叙述型报告特例 |
| 失败 / unknown | 无 / 未归类 | `other` | 失败不进项目列表 |

- **混合报告：** 不强行归一个主项目；`panels` 保留全部项目。`lft_02` 同时出现在肝功、肾功、尿酸；`lft_06` 同时出现在肝功、血脂；`lft_08` 同时出现在肝功、肾功、尿酸、电解质。
- **`other`：** 一行「其他」，行内显示来源报告类别（如「其他 · 临检、免疫」）；详情展示全部其他项。不按报告类别拆多行。
- **影像：** 项目列表一行「影像」；点击直接进最新影像报告详情，不做项目详情 / 趋势。
- **失败报告：** `panels=[other]`，但不进项目列表；按时间视图展示失败状态与原因。
- **未检测：** 不产生趋势点；项目详情显示「未检测」；不计入异常数；不进入核对。
- **家长修正指标名：** 首版不自动重算 `panel`；项目组以识别时的指标名为准。`/classify` 已具备按当前名字重算的能力，但核对路径本期不调用。

### 5.5 医学依据与映射生成

#### 5.5.1 生成规则（声明式映射，不是运行时推理）

1. **`category.panel`**：词典类别的默认项目组。适用于单一项目组词典（`cbc_dict` → `cbc`、`ua_dict` → `urinalysis`、`coag_dict` → `coag`、`imaging_dict` → `imaging`），或为混合词典提供安全默认（`chemistry_dict` → `other`、`bloodtype_dict` / `special_dict` → `other`）。
2. **`item.panel`**：混合词典（`chemistry` / `endocrine` / `immunology`）的逐项覆盖。依据是该指标项的规范名 / 别名、所在词典的分组注释，以及 §5.5.2 / §5.5.3 的官方与临床依据。
3. **解析**：`panel = item.panel ?? category.panel ?? other`；校验该 panel 是否属于 `report_type` 的允许集合；统计命中项数；按 §5.3 排序生成报告级 `panels`。
4. **`/classify`** 复用同一张词典映射：输入 `report_type` + 当前指标名 / 别名，输出 `item_panels` 与 `panels`；不调用 LLM / 外部术语服务，不读原图。
5. **`panels` 只是报告级聚合**：`panels[0]` 只做卡片主标签；项目组本身是指标项的属性，不是报告的唯一分类。

**这不是「AI 自动分组」或「按报告标题猜项目组」：** 报告标题 / 词典匹配只负责选 `report_type`（父类）；项目组由词典的 `category.panel` / `item.panel` 声明式决定，因此可回归、可审计、离线可重复。

#### 5.5.2 官方 / 权威依据

| 文档 | 性质 | 它定义了什么 | 在本 spec 中的用途 | 不能从它推出什么 |
|------|------|--------------|--------------------|------------------|
| 《成人健康体检项目推荐指引（2025年版）》（国家卫生健康委办公厅） | 国家卫生健康委文件 | 基本体检项目类别：血常规、尿常规、肝功能、肾功能、血脂、血糖、血尿酸、甲状腺功能（TSH / FT4）等 | 支持 `cbc` / `urinalysis` / `liver` / `kidney` / `lipid` / `glucose` / `uric_acid` / `thyroid` 作为官方基本类别；支持尿酸独立 | 不定义 16 个 code；不覆盖性激素 / 维生素贫血 / 凝血 / 感染筛查 / 自身抗体；不规定混合生化报告的 item 归属 |
| 《健康体检基本项目专家共识（2022）》（中华医学会健康管理学分会等） | 专家共识 | 基本项目：血常规、尿常规、粪便常规+潜血；生化：肝功能、肾功能、血脂、血糖、尿酸；1+X 专项 | 支持 `cbc` / `urinalysis` / `liver` / `kidney` / `lipid` / `glucose` / `uric_acid` 七个基本类别（不含甲状腺）；支持尿酸独立；支持「基本 + 专项」的产品分层 | 不是国家强制标准；不定义项目组 code；不覆盖全部儿童专项 |
| 《医疗机构临床检验项目目录（2013年版）》（国卫医发〔2013〕9号） | 国家卫生计生委文件 | 1462 项临床检验项目名称，按临床体液 / 血液、临床化学、临床免疫 / 血清学等专业分类 | 作为指标项名称与项目存在性的官方目录；支持血型、凝血、感染、自免、性激素等专项项 | 按专业 / 方法分类，不是器官 / 系统项目组；不定义「肝功 / 肾功 / 甲状腺」panel |
| WS/T 886—2026《临床检验常用项目名称及代码》 | 卫生行业标准（2026-11-01 实施） | 临床检验常用项目名称及代码 | 作为规范名 / 代码的对齐方向；新词典项优先对齐；别名保留历史印形 | 不定义项目组；不覆盖报告级 panel 聚合 |
| WS/T 404 系列《临床常用生化检验项目参考区间》 | 卫生行业标准 | 按项目组给出参考区间：404.1 肝酶、404.2 总蛋白 / 白蛋白、404.3 钾钠氯、404.4 胆红素、404.5 尿素 / 肌酐、404.6 钙磷镁铁、404.9 CRP / 前白蛋白等、404.10 甲状腺激素 | 支持 `liver` / `kidney` / `electrolyte` / `thyroid` 的项组成与参考区间；支持按系统分组 | 不覆盖血脂 / 血糖 / 尿酸全部项目；不定义产品 code / 标签；不是儿童参考区间 |
| WS/T 780—2021《儿童临床常用生化检验项目参考区间》 | 卫生行业标准 | 28 天–18 岁儿童肝功、肾功 / 电解质、骨代谢等参考区间 | 支持儿童 `liver` / `kidney` / `electrolyte` 的项组成与年龄分层 | 不覆盖血脂 / 血糖 / 尿酸 / 甲状腺全部项目；不定义 panel code |
| WS/T 779—2021《儿童血细胞分析参考区间》；WS/T 405—2012《血细胞分析参考区间》 | 卫生行业标准 | 儿童 / 成人血细胞分析参考区间 | 支持 `cbc` 的项组成与年龄分层 | 不定义 panel code |
| WS/T 406—2024《临床血液检验常用项目分析质量标准》 | 卫生行业标准 | 血细胞分析与凝血初筛（PT / APTT / TT / FIB）的分析质量要求 | 支持 `cbc` 与 `coag` 的项组成 | 不定义 D-二聚体 / FDP 全部；不定义 panel code |
| WS/T 461—2024《糖化血红蛋白检测指南》；WS/T 477—2026《D-二聚体定量检测标准》；WS/T 478—2024《血清25-羟基维生素D2和D3检测》 | 卫生行业标准 | HbA1c、D-二聚体、25-OH-VD 的检测 / 质量要求 | 支持 `glucose` / `coag` / `vitanemia` 的相应项 | 不定义 panel code；不覆盖同组其他项 |
| WS/T 645.2—2026《临床常用免疫学检验项目参考区间 第2部分》 | 卫生行业标准 | 血清甲胎蛋白（AFP）、CEA、CA19-9、CA15-3、CA125 的参考区间 | 说明 AFP 在官方标准中与肿瘤标志物同组；本 spec 的 AFP→肝功是**产品偏离**，必须在 §5.5.4 记录 | 不能推出「AFP 必须归肝功」；相反，它支持「AFP 属于肿瘤标志物」 |
| LOINC（Logical Observation Identifiers Names and Codes） | 国际术语标准 | 实验室观察与 panel 概念；示例：24325-3 Hepatic function 2000 panel、24362-6 Renal function 2000 panel、24326-1 Electrolytes 1998 panel、57698-3 Lipid panel with direct LDL、57021-8 CBC W Auto Differential panel、24357-6 Urinalysis macro (dipstick) panel | 支持「panel 是实验室术语中的一等概念」；作为未来 LOINC 映射的参考 | 不是中国法规标准；panel 边界 / 中文标签与国内报告不完全一致；不能直接替代产品 code |
| 专科指南 / 共识（见 §5.5.6） | 学会指南 / 共识 | 血脂、血糖、甲状腺、性早熟、缺铁性贫血、乙肝、自身免疫性肝炎等项目的临床组合与随访 | 支持 `sex_hormone` / `vitanemia` / `infection` / `autoimmune` 及 `thyroid` / `lipid` / `glucose` 的项组成 | 不是体检基本项目目录；不定义 16 个产品 code；不能覆盖混合报告的主标签 |
| 产品文档：`CONTEXT.md` 报告段落、ADR-0009、`2026-10-13-viz-and-client-spec.md` | 产品定义 | 影像单为叙述型报告，无指标项；段落为检查项目 / 检查所见 / 提示 | 支持 `imaging` 作为叙述型报告桶 | 不是实验室项目组；无官方 panel 定义 |
| `other` | 产品定义 | 无法归入上述项目组的指标 / 报告类别 | 唯一跨类别兜底 | 无官方定义；不能反过来当作医学分类 |

#### 5.5.3 逐项目组的依据

| 项目组 | 主要依据 | 支持程度 | 产品决定 / 说明 |
|--------|----------|----------|------------------|
| `cbc` 血常规 | 2025 指引；2022 共识；WS/T 405—2012；WS/T 779—2021；WS/T 406—2024；LOINC 57021-8 | 官方基本项目 + 标准 | 无 |
| `urinalysis` 尿常规 | 2025 指引；2022 共识；2013 目录；LOINC 24357-6 | 官方基本项目 + 目录 | 尿糖 / 尿胆红素按 `report_type=临检` 归尿常规（§5.4） |
| `liver` 肝功 | 2025 指引；2022 共识；WS/T 404.1/2/4；WS/T 780—2021；LOINC 24325-3 | 官方基本项目 + 参考区间 | AFP 归肝功是对 WS/T 645.2 的产品偏离；铜蓝蛋白 / 前白蛋白 / 胆碱酯酶等按临床肝功归入 |
| `kidney` 肾功 | 2025 指引；2022 共识；WS/T 404.5；WS/T 780—2021；LOINC 24362-6 | 官方基本项目 + 参考区间 | eGFR / 胱抑素C 为临床常用增项；尿酸独立 |
| `lipid` 血脂 | 2025 指引；2022 共识；《中国血脂管理指南（2023年）》；LOINC 57698-3 | 官方基本项目 + 指南 | 载脂蛋白等归血脂 |
| `glucose` 血糖 | 2025 指引；2022 共识；《中国2型糖尿病防治指南（2020年版）》；WS/T 461—2024 | 官方基本项目 + 指南 / 标准 | C肽 / 胰岛素归血糖；糖化不拆 |
| `thyroid` 甲状腺 | 2025 指引（TSH / FT4 基本）；WS/T 404.10—2022；《中国甲状腺功能亢进症和其他原因所致甲状腺毒症诊治指南（2022）》 | 官方基本项目（部分）+ 标准 / 指南 | 甲功三项 / 五项 / 七项按医院习惯；T3 / T4 / 抗体归甲状腺 |
| `sex_hormone` 性激素 | 《中枢性性早熟诊断与治疗专家共识（2022）》；《性早熟诊疗指南（试行）》（卫办医政发〔2010〕195号）；2013 目录 | 专项指南 + 目录 | 不在成人基本套餐；儿童性早熟 / 青春期随访的产品增项 |
| `uric_acid` 尿酸 | 2025 指引（血尿酸独立）；2022 共识；2013 目录 | 官方基本项目 | 与肾功分开；样本里 lft_02 / lft_08 同时有肾功 + 尿酸 |
| `electrolyte` 电解质 | WS/T 404.3—2012；WS/T 404.6—2015；WS/T 780—2021；2013 目录 | 标准 + 临床惯例 | 不在 2025 基本套餐；生化常一起出具 |
| `vitanemia` 维生素 / 贫血 | 《儿童铁缺乏和缺铁性贫血防治专家共识（2023）》；WS/T 478—2024；2013 目录 | 专项共识 + 标准 | 25-OH-VD + 铁蛋白 / 叶酸 / B12 的产品组合；不在基本套餐 |
| `infection` 感染筛查 | 2013 目录；《慢性乙型肝炎防治指南（2022年版）》；丙肝 / 梅毒 / HIV 项目见 2013 目录 | 目录 + 专项指南 | 乙肝五项 / 丙肝 / 梅毒 / HIV 的筛查组合是医院惯例；不在基本套餐 |
| `autoimmune` 自身抗体 | 2013 目录；《自身免疫性肝炎诊断和治疗指南（2021）》 | 目录 + 专项指南 | ANA / ENA / ANCA / 自免肝的产品组合；不在基本套餐；ENA / ANCA 项目见 2013 目录 |
| `coag` 凝血 | WS/T 406—2024（凝血初筛）；WS/T 477—2026；2013 目录 | 标准 + 临床惯例 | PT / APTT / TT / FIB / D-二聚体 / FDP 的组合；不在基本套餐 |
| `imaging` 影像 | `CONTEXT.md` 报告段落；ADR-0009；viz spec | 产品定义 | 叙述型报告桶，不是实验室项目组；无官方 panel |
| `other` 其他 | — | 产品兜底 | 血型 / 肝纤维化 / 产前筛查 / 未归类；唯一跨类别兜底 |

#### 5.5.4 明确的产品决定（没有单一官方标准）

- **16 个 code / 中文 label / `other` 兜底**是本产品的稳定词表，不是国家标准的代码表；官方文档提供类别、项目名、参考区间和临床依据，不提供这套 code。
- **基本项目 vs 专项**：2025 指引覆盖血常规、尿常规、肝功、肾功、血脂、血糖、血尿酸、甲状腺（TSH / FT4）；2022 共识覆盖前七项（不含甲状腺）。性激素、维生素 / 贫血、凝血、感染筛查、自身抗体是产品 / 临床专项增项，不能宣称为「体检基本项目」。
- **官方基本项目中当前词典未覆盖的**：粪便常规 + 潜血、HPV / TCT、血同型半胱氨酸等，当前样本 / 词典没有对应报告；首版不做独立项目组，按「其他」兜底或未来按真实报告扩展。
- **AFP 归肝功**：WS/T 645.2—2026 将 AFP 与 CEA/CA19-9 等肿瘤标志物同组；本 spec 首版不设肿瘤标志物项目组，AFP 随报告归肝功，避免独立标签造成不必要的惊吓。这是产品决定；如果未来真实数据 / 临床复核要求独立，另立项目组。
- **尿酸独立**：2025 指引把血尿酸单列，2022 共识也单列；本 spec 不把尿酸并入肾功。
- **尿糖 / 尿胆红素**：官方项目名与血清葡萄糖 / 胆红素相同，靠 `report_type=临检` 与词典类别区分；不是按名字硬猜。
- **性激素 / 维生素贫血 / 凝血 / 感染 / 自免 / 影像 / 其他**：项目组边界、项组成、中文标签均为产品决定；依据是专项指南 / 目录 / 临床惯例，不是单一国家标准。
- **主标签与排序**：`panels[0]` 取命中项数最多，是卡片展示决定；临床报告没有「主项目组」的官方定义。
- **影像 / 其他**：影像不是实验室 panel；其他是兜底，不是医学分类。

#### 5.5.5 词典标注与医学复核

- **第一单落地：** 每个 `*_dict.yaml` 顶部写 `# panel 依据` 块，逐 panel 列出 §5.5.3 的来源；单一项目组词典（cbc / ua / coag / imaging / bloodtype / special）在 `category.panel` 上方写 `# 依据:`；混合词典（chemistry / endocrine / immunology）**不重排条目**，靠每个 item 的 `panel:` 归组，顶部 block 是分组依据；条目本身连续的 panel 段可在段首加 `# ---- panel: <code> ----` + `# 依据:` 作为局部提示。注释不参与解析，不新增 `basis` 字段，避免把文档引用塞进运行时数据。格式示例：
  ```yaml
  # panel 依据（逐项见 item.panel）：
  #   liver: WS/T 404.1/2/4; WS/T 780—2021(儿童); 2025 成人健康体检指引
  #   kidney: WS/T 404.5; WS/T 780—2021(儿童); 2025 成人健康体检指引
  #   ...
  ```
- 新指标项没有明确依据时默认 `other`，并在 review 中标记；不猜、不硬塞。
- `test_dict_coverage` 只校验结构（panel 合法、父子约束、聚合一致），**不能替代医学判断**。
- 医学复核：1 名临床 / 检验专业人员按 §5.5.3 表逐项复核 16 个项目组与 golden 样本映射，重点 AFP / 尿糖 / 尿酸 / 性激素 / 维生素贫血 / 其他；记录在 PR / issue。未复核前，项目组表视为产品草案，不宣称医学定论。
- 「其他」占比按真实归档监控；过高时按数据拆子标签，而不是回到报告级单标签。

#### 5.5.6 参考文档

> 访问日期 2026-10-27。标准 / 指南更新后应重新评审本表；引用只用于说明项目组的医学依据，不构成诊断或治疗建议。

**官方文件 / 标准**

1. 国家卫生健康委办公厅. 成人健康体检项目推荐指引（2025年版）[EB/OL]. (2025-11)[2026-10-27]. https://www.nhc.gov.cn/yzygj/c100068/202511/4feaeb6de63e44cb9fd4ac8f579ca279.shtml.
2. 国家卫生计生委. 医疗机构临床检验项目目录（2013年版）: 国卫医发〔2013〕9号[EB/OL]. (2013-08-07)[2026-10-27]. https://www.nhc.gov.cn/zwgk/wtwj/201308/3f8e4ad3639b419595c310325b571fa3.shtml.
3. 国家卫生健康委员会. WS/T 886—2026 临床检验常用项目名称及代码[S]. 2026-05-25 发布, 2026-11-01 实施. https://www.nhc.gov.cn/wjw/s9492/wsbz.shtml.
4. WS/T 404.1—2012 / 404.2—2012 / 404.3—2012 / 404.4—2012 / 404.5—2015 / 404.6—2015 / 404.9—2018 / 404.10—2022 临床常用生化检验项目参考区间[S].
5. WS/T 780—2021 儿童临床常用生化检验项目参考区间[S]. 2021-04-09 发布, 2021-10-01 实施. https://www.nhc.gov.cn/fzs/c100048/202105/481e639c307a4effa14957f4d0d738fd/files/1733125306220_90260.pdf.
6. WS/T 779—2021 儿童血细胞分析参考区间[S]; WS/T 405—2012 血细胞分析参考区间[S]; WS/T 406—2024 临床血液检验常用项目分析质量标准[S]. WS/T 779 / 780 发布通告: https://www.nhc.gov.cn/fzs/c100048/202105/481e639c307a4effa14957f4d0d738fd.shtml.
7. WS/T 461—2024 糖化血红蛋白检测指南[S]; WS/T 477—2026 D-二聚体定量检测标准[S]; WS/T 478—2024 血清25-羟基维生素D2和D3检测 同位素稀释液相色谱串联质谱法[S]; WS/T 645.2—2026 临床常用免疫学检验项目参考区间 第2部分[S].
8. 卫生部办公厅. 性早熟诊疗指南（试行）: 卫办医政发〔2010〕195号[EB/OL]. (2010-12)[2026-10-27]. https://www.nhc.gov.cn/wjw/gfxwj/201012/faddf413711e457a82a82fedf0010e8e.shtml.

**专家共识 / 指南**

9. 中华医学会健康管理学分会, 《中华健康管理学杂志》编辑委员会. 健康体检基本项目专家共识（2022）[J]. 中华健康管理学杂志, 2023, 17(9): 649-660. DOI: 10.3760/cma.j.cn115624-20230628-00395.
10. 中国血脂管理指南修订联合专家委员会. 中国血脂管理指南（2023年）[J]. 中华心血管病杂志, 2023, 51(3).
11. 中华医学会糖尿病学分会. 中国2型糖尿病防治指南（2020年版）[J]. 中华糖尿病杂志, 2021, 13(4): 315-409. DOI: 10.3760/cma.j.cn115791-20210221-00095.
12. 中华医学会内分泌学分会, 等. 中国甲状腺功能亢进症和其他原因所致甲状腺毒症诊治指南[J]. 中华内分泌代谢杂志, 2022, 38(8). DOI: 10.3760/cma.j.cn311282-20220624-00404.
13. 中华医学会儿科学分会内分泌遗传代谢学组, 中华儿科杂志编辑委员会. 中枢性性早熟诊断与治疗专家共识（2022）[J]. 中华儿科杂志, 2023, 61(1). DOI: 10.3760/cma.j.cn112140-20220802-00693.
14. 中华预防医学会儿童保健分会, 等. 儿童铁缺乏和缺铁性贫血防治专家共识[J]. 中国实用儿科杂志, 2023, 38(3). DOI: 10.19538/j.ek2023030601.
15. 中华医学会肝病学分会, 中华医学会感染病学分会. 慢性乙型肝炎防治指南（2022年版）[J]. 中华临床感染病杂志, 2022, 15(6): 401-427. DOI: 10.3760/cma.j.issn.1674-2397.2022.06.001.
16. 中华医学会肝病学分会. 自身免疫性肝炎诊断和治疗指南（2021）[J]. 中华肝脏病杂志, 2022, 30(5): 482-492. DOI: 10.3760/cma.j.cn112138-20211112-00796.

**国际术语**

17. Regenstrief Institute. LOINC (Logical Observation Identifiers Names and Codes)[EB/OL]. [2026-10-27]. https://loinc.org. 本文引用 panel 示例：24325-3 Hepatic function 2000 panel；24362-6 Renal function 2000 panel；24326-1 Electrolytes 1998 panel；57698-3 Lipid panel with direct LDL；57021-8 CBC W Auto Differential panel；24357-6 Urinalysis macro (dipstick) panel.

**产品文档**

18. `CONTEXT.md`（报告段落 / 项目组 / 报告类别）；`docs/adr/0009-narrative-reports-zero-items.md`；`docs/design/2026-10-13-viz-and-client-spec.md`（影像单叙述型报告）。

---

## 6. 识别服务

### 6.1 词典配置

`recognizer/recognizer/*.yaml` 增加 `category.panel`（默认）与 `item.panel`（覆盖）：

- `cbc_dict.yaml`：`category.panel: cbc`
- `ua_dict.yaml`：`category.panel: urinalysis`
- `coag_dict.yaml`：`category.panel: coag`
- `bloodtype_dict.yaml`：`category.panel: other`
- `imaging_dict.yaml`：`category.panel: imaging`
- `special_dict.yaml`：`category.panel: other`
- `chemistry_dict.yaml`：`category.panel: other`（安全默认），逐项覆盖 `liver` / `kidney` / `lipid` / `glucose` / `uric_acid` / `electrolyte` / `vitanemia` / `other`。
- `endocrine_dict.yaml`：甲状腺项 `thyroid`；孕酮 / HCG 等 `sex_hormone`；其余 `other`。
- `immunology_dict.yaml`：感染项 `infection`；自免项 `autoimmune`；肝纤维化等 `other`。

解析规则：`item.panel` 缺省用 `category.panel`；再缺省 `other`。解析出的 `panel` 必须在该 `report_type` 的允许集合内。`category.panel` / `item.panel` 是词典的声明式映射，不是运行时医学推理；报告标题 / 词典匹配只负责选 `report_type`，项目组不按标题猜。医学依据与产品决定见 §5.5。

- 每个词典文件顶部写 `# panel 依据` 块；单一项目组词典在 `category.panel` 上方写 `# 依据:`；注释不参与解析。格式见 §5.5.5。

### 6.2 输出契约

- `schemas/report.schema.json`：
  - `reportItem` 增加必填 `panel`（`PanelCode` 枚举）。
  - `report` 增加必填 `panels`（`PanelCode` 数组，`minItems: 1`，有序；`panels[0]` 为主标签）。
- `recognizer/recognizer/contract.py`：
  - `ReportItem.panel: Panel`
  - `Report.panels: list[Panel]`
  - `Panel` 枚举 16 个 code。
- 校验：
  - 每个 item 的 `panel` 属于其 `report_type` 的允许集合；`other` 例外。
  - `panels` 等于 item 面板去重后按 §5.3 排序；无 item 时按失败 / 影像规则。
  - `panels` 不允许重复；`panels[0]` 是命中项数最多的项目。
- `Report` 的 `model_validator` 或 `postprocess` 负责校验；契约测试锁定枚举与父子关系。

### 6.3 postprocess

- 选定词典 / `report_type` 后，对每个命中指标项解析 `panel`。
- 统计每个 `panel` 的命中项数；按 §5.3 排序；写入 `Report.panels`。
- 无指标项：
  - `report_type=影像` 且 `sections` 非空 → `panels=[imaging]`，`status=success`。
  - `status=failed` → `panels=[other]`。
  - 其他无指标项且非影像 → 现有「零指标项必须 failed」不变量拦截。
- 输出前校验父子约束；失败则报错，不落库。

### 6.4 `/classify`（存量回填与未来重算）

识别服务新增内部端点 `POST /classify`；家长侧 OpenAPI 不暴露。

```json
// 请求
{ "report_type": "生化", "items": [{ "name": "丙氨酸转氨酶", "unit": "U/L" }, { "name": "肌酐", "unit": "μmol/L" }] }

// 响应
{ "panels": ["liver", "kidney"], "item_panels": ["liver", "kidney"] }
```

- `item_panels` 与请求 `items` 等长、同序；每项属于 `report_type` 的允许集合。
- `panels` 是 `item_panels` 去重后按 §5.3 排序；无 items 时：`report_type=影像` → `[imaging]`，否则 `[other]`。
- 按当前指标名 / 别名查词典；未命中 → `other`。不做 OCR，不需要原图。
- 与 postprocess 共用同一个 panel 解析器，避免两套规则漂移。
- 契约测试：正常、混合、尿糖 / 尿胆红素歧义、影像、未知 report_type、空 items、parent-child 越界。

### 6.5 golden 与测试

- 更新 `samples/expected/report/*.json`：每个 item 加 `panel`，每个 report 加 `panels`；`python -m recognizer.golden` 重新生成并人工核对。
- `recognizer/tests/test_dict_coverage.py`：每个 item 的 `panel` 在允许集合内；所有 code 都有 label / 父类映射。
- `test_postprocess.py`：标签选择、排序、混合报告、尿糖 / 尿胆红素、影像、失败、无指标项。
- `test_contract.py`：schema 枚举 / 必填 / 父子约束。
- `test_api.py`：`/classify` 正常 / 边界 / 错误。
- 样本主标签预期（完整以 golden 为准）：
  - `lft_02 → [liver, kidney, uric_acid]`
  - `lft_06 → [liver, lipid]`
  - `lft_08 → [liver, kidney, uric_acid, electrolyte]`
  - `lft_09 → [liver, kidney]`
  - `glu_01 → [glucose]`
  - `tft_01 → [thyroid]`
  - `proghcg_01 → [sex_hormone]`
  - `vitanemia_01 → [vitanemia]`
  - `cbc_04 → [cbc]`
  - `ua_05 → [urinalysis]`
  - `coag_01 → [coag]`
  - `bloodtype_01 → [other]`
  - `fibrosis_01 → [other]`
  - `unknown_01 → [other]`
  - `us_01 → [imaging]`
  - `infectious_01 → [infection]`
  - `ana_01 → [autoimmune]`

---

## 7. 数据与契约

### 7.1 数据库

新增迁移 `internal/storage/migrations/0011_report_panels.sql`：

```sql
ALTER TABLE report_items ADD COLUMN panel text NOT NULL DEFAULT 'other';
ALTER TABLE reports ADD COLUMN panels text[] NOT NULL DEFAULT '{}';

CREATE INDEX report_items_report_panel_idx ON report_items (report_sha256, panel);
CREATE INDEX reports_member_date_idx ON reports (member_id, report_date DESC);
```

- `report_items.panel` 存稳定 code；存量行默认 `other`，回填后纠正。
- `reports.panels` 存有序 code 数组；存量行默认 `{}`（App 回退 `report_type`）；新报告由识别服务输出。
- 不加 CHECK 约束；枚举与父子关系由识别端 / Go 契约测试校验，新增 code 不强制迁移。
- 不在 `reports` 上存 `primary_panel`；`panels[0]` 即主标签，避免两份真相。
- 回填只更新 `report_items.panel` 与 `reports.panels`，不触碰 `review_status`、修正对、value/unit/ref_range。

### 7.2 识别契约

- `schemas/report.schema.json`：`reportItem.panel`、`report.panels`（见 §6.2）。
- `recognizer/recognizer/contract.py`：`Panel` 枚举、`ReportItem.panel`、`Report.panels`。
- Go `internal/contract` 镜像：`ReportItem.Panel string`、`Report.Panels []string`。
- golden / replay 样例全部更新；契约测试锁定字段与枚举。

### 7.3 HTTP API

新增 `PanelCode` 枚举（16 个）：

```text
cbc | urinalysis | liver | kidney | lipid | glucose | thyroid | sex_hormone |
uric_acid | electrolyte | vitanemia | infection | autoimmune | coag | imaging | other
```

**报告摘要 / 详情：**

- `ReportHeader`（`ReportSummary` / `ReportDetail` 共用）新增 `panels: PanelCode[]`（必填；未回填存量可为 `[]`）。
- `ReportDetail` 的 `ReportItem` 新增 `panel: PanelCode`（必填）。
- 不新增 `ReportSummary.pathology_tag` / `pathology_tags` / item 级 `pathology_tag`。
- 失败报告：新识别输出 `panels=[other]`；未回填存量可为 `[]`；App 对 failed 直接回退 `report_type`。
- 影像成功 `panels=[imaging]`。
- 契约 fixtures 更新：summary / detail / failed / imaging / upload 响应。

**项目列表：**

```text
GET /members/{member_id}/panels
→ { "panels": PanelSummary[] }

PanelSummary:
  panel: PanelCode
  report_count: integer          # 该成员非 failed 报告里包含该项目的份数
  item_count: integer            # 该项目下 name+unit 去重后的指标数
  abnormal_count: integer        # 每个 name+unit 取最近非 not_tested 值，flag ∈ high/low/abnormal 的数量
  latest_report_date: string|null
  latest_report_id: string|null
  report_types: string[]         # 来源报告类别；other 行显示「其他 · 临检、免疫」
  narrative: boolean             # imaging=true；点击直接进最新报告详情
```

- 范围：当前成员 `status != failed` 的报告。
- 排序：`latest_report_date` 降序（null 沉底），同日按 §5.3 优先级。
- 只返回 `report_count > 0` 的项目；没有报告的成员返回空数组。
- 失败报告不参与项目列表 / 项目详情 / `?panel=` 筛选；只在按时间视图或 `filter=failed` 出现。
- `imaging` / `narrative` 项目不做项目详情；App 用 `latest_report_id` 直接打开报告详情。
- `other` 的 `report_types` 用于行内副标题；非 `other` 项目通常一个来源类别。

**项目详情：**

```text
GET /members/{member_id}/panels/{panel}
→ PanelDetail

PanelDetail:
  panel: PanelCode
  member_id: integer
  report_count: integer
  latest_report_date: string|null
  latest_report_id: string|null
  report_types: string[]         # 来源报告类别；other 详情头部显示
  narrative: boolean
  items: PanelItem[]

PanelItem:
  name: string
  unit: string|null
  ref_range: string|null          # 取 latest 点的参考范围
  latest: TrendPoint|null         # 最近一个非 not_tested 点；没有则 null
  points: TrendPoint[]            # 同 name+unit 的跨报告点，日期升序；不含 not_tested
  report_count: integer           # 有非 not_tested 点的报告数
  not_tested_count: integer       # 该指标被标为 not_tested 的报告数
  latest_report_not_tested: boolean  # 最近一份含该指标的报告中，此项是否为 not_tested
```

- 按 `name + unit` 分组；同名的跨单位行分开，不合并、不换算。
- `points` 复用现有 `TrendPoint` 形状（report_id / report_date / value / unit / ref_range / method / flag / low_confidence / reviewed）。
- 排序：`latest.flag ∈ {high, low, abnormal}` 的在前；其余按最新报告内顺序 / 名称稳定排序。
- `latest` 为 null 且 `not_tested_count > 0`：UI 显示「未检测」；不产生趋势。
- `latest` 非 null 且 `latest_report_not_tested=true`：UI 在数值旁标「最近一次未检测」，数值是最近一次已知值，不冒充本次结果。
- `narrative=true` 时 `items=[]`；App 不打开该详情，项目列表行直接进报告详情。
- 未知 member → 404；非法 `panel` → 400。

**时间线筛选：**

- `/timeline?panel=kidney` 是 App 内部筛选，不是 API 参数。
- `ReportSummary.panels` 已包含项目数组，客户端 `panels.includes(panel)` 过滤。
- `?filter=partial|failed` 保留；两者可同时出现，按 AND 过滤；「清除筛选」回 `/timeline`。
- `?panel=` 只看非 failed 报告；`panels=[]` 的未回填存量（非 failed）按 `[other]` 参与筛选，与项目列表把存量项归「其他」一致；failed 不参与。

### 7.4 Go 数据服务

- `internal/contract`：`ReportItem.Panel`、`Report.Panels`。
- `pipeline.ReportSummary.Panels []string`；`ReportRow.Panels []string`；`ReportItemRow.Panel string`。
- `pipeline` 在 ingest / reparse 时读取识别服务输出的 `panel` / `panels`，随 `WriteReport` 写入。
- `storage`：
  - 摘要 / 详情投影读 `r.panels`、`i.panel`。
  - 新增 `PanelSummariesByMember`、`PanelItemsByMemberPanel`、`AllReportsForBackfill`、`UpdateReportPanels`。
  - 项目列表 / 详情在 SQL 侧聚合；不在 HTTP 层逐报告 N+1。
- `httpapi`：新增两条 GET 路由与 `PanelCode` / `PanelSummary` / `PanelDetail` / `PanelItem` 类型；contract fixtures。
- `hdd backfill-panels`：
  - 遍历报告（可 `--limit` / `--dry-run`），读取当前 `report_type` + `report_items`（id/name/unit）。
  - 调识别服务 `POST /classify`。
  - 用返回的 `item_panels` 按 item id 更新 `report_items.panel`；用 `panels` 更新 `reports.panels`。
  - 不删除 / 重插指标项；不碰 `review_status`、修正对、值 / 单位 / 参考范围。
  - 幂等；失败逐份报告并继续；最终打印成功 / 跳过 / 失败计数。
  - 识别服务不可用时命令失败，提示部署顺序。
- **不用 `/reparse` 回填：** reparse 会删除并重插 `report_items`，丢失核对状态与修正对；回填走 `/classify` 只更新 panel 列。

### 7.5 App 数据

- `openapi-typescript` 重新生成 `schema.d.ts`。
- 新增 `apps/mobile/src/reports/panels.ts`：
  - `PanelCode` 类型（来自 schema）
  - `PANEL_LABELS: Record<PanelCode, string>`（穷尽）
  - `PANEL_ORDER: PanelCode[]`（§5.3）
  - `panelLabel(code)`
- 新增 `apps/mobile/src/reports/panelOverview.ts`：
  - `reportPanelLabels(report, max=2)`：失败回退 `report_type`；`panels=[]` 回退 `report_type`；否则前 2 + `+N`。
  - `reportHasPanel(report, panel)` / `filterReportsByPanel(reports, panel)`：failed 不匹配；`panels=[]` 的非 failed 存量按 `[other]` 处理，与项目列表归「其他」一致。
  - `parsePanelParam(raw)`：数组取首值，只接受合法 code。
  - `familyAbnormalLabel(panels, max=2)`：`abnormal_count > 0`，按 count 降序 + `PANEL_ORDER`；`肝功 4 项 · 甲状腺 2 项 +N`；无则 undefined。
  - `totalAbnormalCount(panels)`。
- 新增 `apps/mobile/src/reports/relativeDate.ts`：`relativeDateLabel(date, today)`；今天 / 昨天 / N 天前 / N 周前 / N 个月前 / N 年前；null / 未来返回 undefined；测试传固定 today。
- 新增 hooks：
  - `useFamilyOverview()`：members + 每个成员的 reports + panels；loading / unconfigured / error / ready；聚焦重载。
  - `useMemberPanels(memberId)`：当前成员项目列表；只在「按项目」视图挂载时请求。
  - `usePanelDetail(memberId, panel)`：项目详情。
  - `useAbnormalPanelGroups(memberId, panels)`：异常指标页；只请求 `abnormal_count > 0` 的项目详情。
- `api/reports.ts` 增加 `PanelSummary` / `PanelDetail` / `PanelItem` 类型与 `listMemberPanels` / `getPanelDetail`。
- `ReportSummary` / `ReportDetail` 类型随 codegen 获得 `panels` / `item.panel`。

---

## 8. App 信息架构与页面

### 8.1 最终结构

```text
App
├── Tab 家庭（/）
│   ├── 成员卡片（孩子优先）：待核对 + 最近项目/日期 + 异常项目 + 最近报告偏高/偏低 + 报告总数
│   ├── 点卡片 = 选中该成员 + 进档案
│   ├── 管理成员
│   └── header：设置
├── Tab 档案（/timeline）
│   ├── 成员 chips（当前成员）
│   ├── 分段：按时间 | 按项目
│   ├── 按时间：待核对 / 识别失败入口 + 报告卡（日期 / 项目 / 状态 / 异常摘要）
│   ├── 按项目：全部异常入口 + 项目行（最近日期 / 份数 / 异常数 / 指标数）
│   ├── 上传 FAB（仅当前成员已选时）
│   └── header：设置
└── Stack：报告详情 / 核对 / 上传 / 设置 / 成员管理 / 项目详情 / 异常指标
```

- **双 Tab，不用 4 Tab。** 成员与档案共享同一成员上下文；上传是动作、设置是低频配置，都不占一级导航。
- **上传用 FAB，只在档案页出现。** 家庭页先选成员，进档案后上传，归属明确；上传页保留成员确认。
- **设置进 header。** 首页 / 档案页内容区不再出现设置链接；设置页本身不变（Base URL、保存并校验、连通状态）。
- **`/` 改家庭卡片首页，`/timeline` 保留为档案路径。** 概览必须真的是首页；保留 `/timeline` 不改删除回跳与现有深链。
- **`/trends` 保留为 Stack「异常指标」。** 不退役；分组从 `report_type` 改成项目；从档案「按项目」的「全部异常 N 项」进入。
- **`/panel/[panel]` 新增。** 路由参数用稳定 code；页面标题用项目 label；当前成员由全局上下文决定。

### 8.2 家庭首页

- Header「家庭」+ 右侧「设置」。
- 成员卡片 `sortMembers`（孩子优先、其余创建序）：
  - 名字 + 「孩子」徽标。
  - 待核对：`pendingSummary(reports)`；两项都为 0 → 「没有待核对」；否则按「待核对 N 份[ · M 项]」+（失败 > 0 时）「· 识别失败 K 份」拼接；N=0 时省略待核对段。
  - 最近：最新报告（`sortReports` 第一份）的 `panels[0]` + 日期 + 距今；最新报告失败 → 「最近报告识别失败 · 日期」；无报告 → 「这位成员还没有检查单」。
  - 异常项目：`familyAbnormalLabel(panels)`；如「肝功 4 项 · 甲状腺 2 项」；无异常 → 「没有异常项目」（muted）。
  - 最近报告偏高/偏低：最新报告为非 failed 时用其 `flag_counts.high/low`；「最近报告 偏高 4 · 偏低 0」；最新报告 failed 或无报告时省略。
  - 报告总数：「报告 N 份」。
- 点卡片：`selectMember(memberId)` + `router.push('/timeline')`。
- 底部「管理成员」→ `/members`。
- 空态 / 错误态：
  - 无成员 → 「还没有家庭成员。先到家庭成员建一个。」+ 链接；
  - 未配置后端 → 「尚未配置后端地址，请先到设置填写。」；
  - 加载失败 → 「加载失败：{detail}」+ 重试；
  - 成员无报告 → 卡片只显示名字 + 「这位成员还没有检查单」；待核对 / 异常项目 / 偏高偏低 / 总数不显示，不隐藏成员卡。
- 文案只陈述事实，不做建议。

### 8.3 档案页

- Header「档案」+ 右侧「设置」。
- 成员 chips：沿用 `MemberChips` / `CurrentMemberContext`；切换成员刷新。
- 分段：「按时间」/「按项目」；默认按时间；分段状态不持久化（回档案默认按时间）。
- **按时间：**
  - 待核对 / 识别失败入口（沿用 `pendingSummary` 口径）→ `/timeline?filter=partial|failed`；
  - 报告卡：日期、项目名（`reportPanelLabels`）、状态、`flagBadges`、`unreviewedBadge`；失败回退 `report_type` + 失败原因；
  - `?filter` / `?panel` 筛选横幅：`只看待核对 · N 份` / `只看识别失败 · N 份` / `只看肾功 · N 份`；「清除筛选」→ `/timeline`；
  - 筛选后为空：`没有符合筛选条件的报告。` + 「查看全部档案」；
  - 无报告：沿用现有空态。
- **按项目：**
  - 顶部 `全部异常 N 项 ›` → `/trends`；N = `totalAbnormalCount(panels)`；0 时不显示或显示「没有异常项目」。
  - 项目行：项目 label、最近日期（`reportDateLabel` 口径；null → 日期未知）、报告份数、异常数、指标数；`other` 行加来源报告类别副标题；`imaging` 行显示「叙述型报告」；
  - 点项目 → `/panel/[panel]`；`imaging` → 最新影像报告详情；
  - 无报告 / 全部 failed：`还没有可归类的报告。识别失败的记录可在按时间查看。` + 切回按时间；
  - 加载 / 错误 / 未配置 / 未选成员态与按时间同口径。
- 上传 FAB：当前成员已选时显示；点击 `/upload`；未选成员时隐藏。
- 未选成员：`选择一个成员查看其档案。` + 回家庭。

### 8.4 项目详情

- Header：项目 label；成员名、最近日期、报告份数、异常数（= `items` 中 `latest.flag ∈ high/low/abnormal` 的数量，与 `PanelSummary.abnormal_count` 同口径）；`other` 额外显示来源报告类别。
- 指标区：
  - **异常项（latest.flag ∈ high/low/abnormal）**：内联 `TrendSparkline`；名称、最新值 + 单位、参考范围、flag、低置信空心点 / 待核对标签；元信息「N 份报告 · 起止日期」；
  - **正常 / 未知 / 未检测项**：紧凑行（名称、最新值、参考范围、flag）；点按打开 `TrendDetailSheet`（用已加载的 `points`，不额外请求）；`unknown` 用 `itemFlagLabel` 文案；`not_tested` 显示「未检测」，无点不打开趋势；`latest_report_not_tested=true` 时在数值旁标「最近一次未检测」；
  - 分组标题「异常 N 项」「其余 M 项」；其余默认展开；单项目其余项超过 12 项时默认折叠为「其余 N 项」，点按展开（CBC 等大项目）。
- 底部：`查看包含该项目的报告（N 份）›` → `/timeline?panel=code`。
- 最新一份报告 failed：头部提示「另有更新的一份识别失败报告」+ 链接；数值取最近非 failed。
- 无可用项：`这个项目还没有可展示的指标。` + 返回按项目。
- 加载 / 错误 / 未配置 / 未选成员态与现有页面同口径。
- `imaging`：项目列表直接进报告详情，不进入此页。

### 8.5 异常指标

- Header「异常指标」+ 成员 chips（沿用现有；切换刷新）。
- 范围：当前成员 `GET /members/{id}/panels` 中 `abnormal_count > 0` 的项目；逐个取项目详情，列出 latest flag 为 high/low/abnormal 的指标。
- 分组：项目 label + 最新报告日期 + `N 项异常`；项目顺序按 `latest_report_date` 降序 / §5.3。
- 行：名称、最新值 + 单位、参考范围、flag、低置信标签、`TrendSparkline`、`查看趋势 ›`；点按打开 `TrendDetailSheet`。
- 空态：无报告 → 现有「这位成员还没有检查单」；有报告无异常 → 「这位成员暂时没有异常指标。」+ 「查看全部档案」；错误 / 未配置 / 未选成员同现有。
- 不新增 `report_type` 分组；`report_type` 只作为项目详情的次要信息。

### 8.6 报告详情

- 头部沿用 `ReportCard`（项目名 + 日期 + 状态 + 徽标）；短 ID；影像段落；失败原因；待核对入口。
- 指标项按 `report.panels` 顺序分组：
  - 组头：项目 label + `N 项 · M 项异常`；
  - 组内保持报告内指标顺序；展示报告全部指标（不按项目过滤）；
  - 若某 item 的 panel 不在 `report.panels`，归入末尾「其他」组；
- 核对动作 / 修正 / 改归属 / 删除不变；核对页仍逐项平铺，不在本期分组。
- 影像单：仍以段落 + 原图为主，不呈现指标项 / 项目分组。

### 8.7 路由与状态

- `src/app/(tabs)/_layout.tsx`：Tabs（家庭 / 档案）；header 右侧「设置」。
- `src/app/(tabs)/index.tsx`：家庭首页（替换现有 `index.tsx` 内容）。
- `src/app/(tabs)/timeline.tsx`：档案页（现有 `timeline.tsx` 内容 + 分段 / 项目视图）。
- `src/app/panel/[panel].tsx`：项目详情（Stack）。
- `src/app/trends.tsx`：异常指标（Stack，保留）。
- 根 `_layout.tsx`：`(tabs)` 关闭 Stack header；详情 / 设置 / 成员 / 上传 / 报告 / 核对 / 项目 / 趋势用 Stack header。
- URL 保持 `/` 与 `/timeline`；`/panel/kidney` 用 code；`/timeline?panel=kidney` 为内部筛选。
- 当前成员上下文不变；家庭卡点击 = 选中 + 进档案；档案 chips 切换；上传默认当前成员。
- 设置页不变；从 header 进入；主路径不再有设置链接。

---

## 9. 怎么实施（三单）

> 执行票见 `.scratch/panel-browse/issues/`（01–08）；本节「三单」是产品交付分组，实施票是更细的 tracer bullet 拆分，依赖关系以票的 `Blocked by` 为准。01–07 为 agent 实施票；08 为人工医学复核。

### 9.1 第一单：识别 + 数据 + API + 回填

1. **识别端**
   - 词典 `panel` 配置（§6.1）；`Category` / `Item` 解析。
   - 词典分组依据注释（§5.5.5）：每个 `*_dict.yaml` 顶部 `# panel 依据` 块；单一项目组词典 `category.panel` 上方 `# 依据:`；混合词典用 `item.panel` 归组，不重排条目，连续段可加局部 `# ---- panel: <code> ----` / `# 依据:`；不改 YAML 结构。
   - `postprocess` 解析 / 计数 / 排序 / 校验；`ReportItem.panel`、`Report.panels`。
   - `contract.py` / `report.schema.json` 更新；`Panel` 枚举。
   - `POST /classify`（§6.4）+ 与 postprocess 共用解析器。
   - 更新 golden / replay 样例；`python -m recognizer.golden`；人工核对。
   - 测试：`test_dict_coverage` / `test_postprocess` / `test_contract` / `test_api`。
   - 验证：`cd recognizer && pytest`；golden diff；样本主标签预期。
2. **迁移与 Go**
   - `0011_report_panels.sql`；`WriteReport` / 查询 / 投影写读 panel。
   - `internal/contract` 镜像；`pipeline` / `storage` / `httpapi` 类型。
   - 项目列表 / 详情查询 + 路由 + fixtures。
   - `hdd backfill-panels`（`/classify`，只更新 panel 列，保留核对状态）。
   - 迁移测试、存储测试、回填幂等测试、契约测试。
   - 验证：`go test ./...`；`npm run codegen`。
3. **OpenAPI / App 类型**
   - `schemas/openapi.yaml`：`PanelCode`、`ReportHeader.panels`、`ReportItem.panel`、`PanelSummary` / `PanelDetail` / `PanelItem`、两条路径。
   - `schemas/testdata/http/*` fixtures；`apps/mobile/src/api/schema.d.ts`。
   - 同步更新 App 测试里构造 `ReportSummary` / `ReportItem` 的 factory：加 `panels: []` / `panel: 'other'` 默认值；否则 codegen 后的 required 字段会让 typecheck 失败。
   - 验证：Go 契约测试；`npm run typecheck`（App 类型先可用）。

### 9.2 第二单：App 项目视图

1. **纯逻辑**
   - `panels.ts` / `panelOverview.ts` / `relativeDate.ts` + 测试。
   - 测试数据用真实 golden（lft_02 / lft_06 / lft_08 / tft_01 / ua_05 / us_01 / coag_01 / fibrosis_01）。
2. **数据层**
   - `api/reports.ts` 类型 / 函数；`useMemberPanels` / `usePanelDetail` / `useAbnormalPanelGroups`。
   - `ReportSummary.panels` / `ReportDetail.items[].panel` 消费。
3. **页面 / 组件**
   - `ReportCard` 项目标签（前 2 + `+N`）；失败回退。
   - 档案页分段 + 项目列表；`/panel/[panel]`；`?panel=` 筛选横幅。
   - 报告详情项目分组。
   - `/trends` 改项目分组 + 项目详情数据。
   - `TrendChart` / `TrendDetailSheet` 复用；项目详情不再逐项请求趋势。
4. **验证**
   - `npm test`、`npm run typecheck`、`npx expo lint`。
   - E2E：项目列表 / 项目详情 / 筛选 / 歧义 / 影像 / 失败 / 其他；现有核对 / 信任 / 趋势用例。

### 9.3 第三单：IA 壳

1. `(tabs)/_layout.tsx` + `(tabs)/index.tsx` + `(tabs)/timeline.tsx`；根 Stack 调整。
2. 家庭首页 `useFamilyOverview`；卡片字段 / 空态 / 点击进档案；「管理成员」。
3. 设置进 header；移除首页底部设置链接；上传 FAB（仅档案页、当前成员已选）。
4. `/trends` 入口改为档案「按项目」的「全部异常」；更新深链与回跳。
5. E2E 更新：家庭卡、双 Tab、设置 header、FAB、`/timeline?panel=`；`docs/agents/app-e2e.md`。
6. 验证：`npm test`、`npm run typecheck`、`npx expo lint`、`npm run e2e`；手工 Expo Go / iOS 或 Android 一端。

### 9.4 改动面（预估）

| 区域 | 文件 |
|------|------|
| 识别词典 | `recognizer/recognizer/{cbc,ua,coag,bloodtype,chemistry,endocrine,immunology,imaging,special}_dict.yaml`（含 `# panel 依据` / `# 依据:` 注释） |
| 识别代码 | `recognizer/recognizer/{contract.py,postprocess.py,api.py,golden.py}`、`recognizer/tests/*` |
| 识别契约 | `schemas/report.schema.json`、`samples/expected/report/*.json`、replay 样例 |
| 存储 | `internal/storage/migrations/0011_report_panels.sql`、`store.go`、`store_reports_test.go`、`migrate_test.go` |
| 管道 | `internal/pipeline/{types.go,ingest.go,reports.go,review.go,backfill.go}`、`internal/contract/*` |
| HTTP | `internal/httpapi/{types.go,handlers.go,handlers_test.go}`、`schemas/openapi.yaml`、`schemas/testdata/http/*` |
| 命令 | `cmd/hdd/{main.go,commands.go,serve*.go}` |
| App 数据 | `apps/mobile/src/api/{reports.ts,schema.d.ts}`、`apps/mobile/src/reports/{panels.ts,panelOverview.ts,relativeDate.ts,useMemberPanels.ts,usePanelDetail.ts,useAbnormalPanelGroups.ts,useFamilyOverview.ts}` |
| App 页面 | `apps/mobile/src/app/(tabs)/{_layout,index,timeline}.tsx`、`panel/[panel].tsx`、`trends.tsx`、`report/[report_id].tsx`、`_layout.tsx` |
| App 组件 | `apps/mobile/src/reports/{components.tsx,TrendChart.tsx,TrendDetailSheet.tsx}`、`apps/mobile/src/members/components.tsx` |
| E2E | `apps/mobile/e2e/{support.ts,compose.yml,tests/*}`、`docs/agents/app-e2e.md` |
| 文档 | `CONTEXT.md`、`docs/adr/0012-item-level-panels.md`、本 spec |

无新前端依赖；识别端沿用现有词典 / postprocess；Go 沿用现有存储与 HTTP 层；项目详情复用 `react-native-svg` 趋势组件。

---

## 10. 怎么验收

### 10.1 识别服务

- 每条 golden 报告都有 `panels`；每个 item 都有 `panel`；父子约束合法（除 `other` 外不跨类别）。
- 样本主标签 / 面板集合回归（§6.5）；尿糖 / 尿胆红素 / 血型 / 肝纤维化 / 影像 / 失败边界。
- 词典依据注释齐全：9 个 `*_dict.yaml` 都有 `# panel 依据` 块或 `# 依据:` 行；混合词典 block 覆盖其所有 `panel:` 值；连续段局部 banner 可选；人工对照 §5.5.3。
- `/classify`：混合、歧义、空 items、未知 report_type、越界拒绝。
- 命令：`cd recognizer && pytest`；golden diff 人工核对。

### 10.2 数据 / 契约

- 迁移后 `report_items.panel='other'`、`reports.panels='{}'`；新报告写入正确；摘要 / 详情读 panel。
- 项目列表：`lft_02` 在肝功 / 肾功 / 尿酸；`lft_06` 在肝功 / 血脂；`lft_08` 在肝功 / 肾功 / 尿酸 / 电解质；无空分类。
- 项目详情：肾功包含尿素 / 肌酐 / eGFR / 胱抑素C（单份 lft_02 为 4 行；跨报告同名跨单位按现有门控分行，不合并）；单份 lft_02 的肝功 10 项、4 项异常；单份 tft_01 的甲状腺 8 项。
- 回填：`hdd backfill-panels` 后旧报告面板正确；重复执行幂等；核对状态 / 修正对不变；`--dry-run` 可预检。
- 契约：summary / detail / failed / imaging / upload 的 `panels` / `panel`；项目列表 / 详情 fixtures；父子关系合法。
- 命令：`go test ./...`。

### 10.3 App / E2E

- **家庭首页**：3 成员卡；待核对 / 最近项目 / 异常项目 / 偏高偏低 / 总数；点卡进档案并选中该成员；设置 header；无成员 / 未配置 / 错误态。
- **双 Tab**：家庭 / 档案切换；URL `/` 与 `/timeline`；上传 FAB 仅档案页且当前成员已选；设置不再出现在内容区。
- **按项目**：肝功 / 肾功 / 尿酸 / 甲状腺 / 血常规 / 尿常规 / 血脂 / 血糖 / 电解质 / 感染筛查 / 凝血 / 影像 / 其他；`other` 副标题；影像直接进报告。
- **项目详情**：肾功包含尿素 / 肌酐 / eGFR / 胱抑素C + 趋势；肝功默认展示 lft_02 的 4 项异常 + 6 项正常；甲状腺 TSH/TPOAb；点正常项打开趋势层；「包含该项目的报告」→ `/timeline?panel=`。跨报告单位不同会分行，不强行合并。
- **歧义**：`ua_05` 归尿常规，不出现血糖 / 肝功；`tft_01` 归甲状腺；`fibrosis_01` / `bloodtype_01` 归其他；`coag_01` 归凝血；`vitanemia_01` 归维生素/贫血。
- **影像 / 失败**：`us_01` 归影像，点开进报告详情；失败报告不进项目列表 / `?panel=` 筛选，按时间视图可见。
- **报告详情**：混合报告按项目分组，展示全部指标；核对入口仍可用。
- **异常指标**：按项目分组；只列 latest 异常项；点行打开趋势层；无异常空态。
- **回归**：核对、修正、改派、删除、趋势单位门控、上传归属、多图上传全绿。
- 命令：

```bash
go test ./...
cd recognizer && pytest
cd apps/mobile
npm test
npm run typecheck
npx expo lint
npm run e2e
```

### 10.4 人工验收

- 3–5 位家长，5 份真实报告；任务：从家庭首页找到孩子最近一次肾功 / 肝功 / 甲状腺结果。4/5 无帮助、4 次点按内完成。
- 观察：是否把「项目组」理解成体检项目组；是否接受「混合报告在多个项目下出现」；是否能在项目详情看懂异常 / 正常 / 未检测 / 待核对；家庭卡是否能说出「谁需要处理」。
- 医学复核：1 名临床 / 检验专业人员按 §5.5.3 表逐项复核 16 个项目组与 golden 样本映射，重点 AFP / 尿糖 / 尿酸 / 性激素 / 维生素贫血 / 其他；记录在 PR / issue。未复核前，项目组表是产品草案，不宣称医学定论。
- 部署 runbook：识别服务 → 迁移 → `hdd backfill-panels` → App；未回填前项目视图把旧报告归「其他」，App 卡片回退 `report_type`。

---

## 11. 非目标（本期不做）

- 报告级单标签 / 报告级多标签作为主分类；报告级 `pathology_tag`。
- Go 数据服务分类 / Go 维护「指标名 → 项目组」映射。
- 指标项级用户可编辑标签 / 家长自定义项目组。
- 家长修正指标名后自动重算 `panel`（`/classify` 已具备能力，但核对路径不调用；未来另立需求）。
- 搜索、年份分组、时间线压缩、跨成员时间线。
- 报告详情按项目过滤 / 只展示主项目指标；项目详情始终展示该项目全部指标，报告详情始终展示报告全部指标。
- 多标签报告级浏览（同一报告在多个项目下出现已由指标项级实现；不需要额外「多标签」概念）。
- 肿瘤标志物 / 血型 / 肝纤维化 / 产前筛查独立项目组。
- 诊断、建议、复查提醒、风险评分、跨单位换算。
- 公网鉴权 / 共享 / 第二用户。
- 服务端分页 / 搜索 / 家庭聚合端点（首版 N+1 可接受；超 2s 再议）。
- 多页检查单合并 / 报告合并 / 就诊事件分组。
- 新前端依赖。

---

## 12. 风险与假设

| 风险 / 假设 | 影响 | 验证 / 缓解 |
|-------------|------|-------------|
| 识别端契约 / golden 变更范围大，跨 Python / Go / App 协调 | 回归 / 版本错配 | 先识别 + golden，再 Go 契约，再 App；契约测试 + replay 全绿；部署顺序写入 runbook |
| 项目组依赖词典逐项标注，约 90 条 | 标注错误 / 漏项 | `test_dict_coverage` 校验每项 panel 合法；真实 golden 人工核对；代码 review |
| 项目组映射是产品抽象；官方文档只定义基本类别 / 项目名 / 参考区间，不定义 16 个 code | 医学上不准确 / 争议 | §5.5 依据 + 临床复核；新增项默认 `other`；真实数据迭代 |
| 混合报告在多个项目下出现，列表变长 | 项目列表 / 家庭卡密度 | 项目列表只显示实际出现的项目；卡片前 2 + `+N`；真实数据观察 |
| `other` 混多个报告类别（血型 / 肝纤维化 / 产前） | 行内信息不足 | 行内显示来源类别；详情展示全部项；有数据再拆 |
| 项目详情一次返回所有点，数据量大 | 首屏 / 内存 | 20–50 份/年、单项目 4–25 项；先实测，超 2s 再加分页 / 惰性 |
| 家庭首页 N+1（members + reports + panels） | 首屏慢 | 2–5 成员实测 <2s；超时再加 family 聚合端点 |
| 回填 `/classify` 用当前名字，修正名可能不在词典 | 修正项归 `other` | 回填只更新 panel；`other` 可接受；未来 `/classify` 重算 |
| `/classify` 新端点增加识别面 | 维护成本 | 与 postprocess 共用解析器；只服务回填 / 未来重算；契约测试锁定 |
| 主标签取命中项数最多，混合报告主标签与家长预期不一致 | 卡片主标签偏差 | 项目视图展示全部项目；详情不隐藏其他指标；用真实报告验证 |
| 双 Tab / 文件移动破坏深链 / E2E | 回归 | `(tabs)` 组保持 URL；第三单单独一单；E2E 全量回归 |
| 无真实用户数据 | 假设失真 | 样本语料 + 真实归档；所有假设显式列出；先验证再扩大 |
| `panels` 数组与 `report_items.panel` 漂移 | 数据不一致 | 识别端校验 `panels` = item 面板聚合；Go 契约测试；回填幂等 |
| 单位不同导致同名指标分行 | 项目详情行数变多 | 复用现有单位门控；跨单位不连线、不换算；如实分行 |
| 失败 / 未回填报告在项目视图不可见 | 家长找不到 | 按时间视图保留失败；项目视图空态提示切回按时间；部署 runbook |

---

## 13. 与现有文档的关系

- **替代并删除**：
  - `docs/design/2026-10-26-app-ia-and-pathology-tags.md`（v1）的「报告级多标签、4 Tab、时间线 chips、P2 才做系统视图」。
  - `docs/design/2026-10-26-app-ia-and-pathology-tags-v2.md`（v2.2）的「报告级单标签、标签详情只看最近一份报告、`/trends` 退役、12 + 其他固定集合」。
  - 两篇的「病理标签」用词由「项目组」替代；两篇保留的产品背景、家长心智、成员主轴、文案中性原则继续有效。
- **对齐** `docs/design/2026-10-13-viz-and-client-spec.md`（v1.3）：成员主轴、M3 成员卡片首页（本 spec 落为家庭卡片首页）、M4 趋势（本 spec 改为按项目分组）；信任纵切、核对状态机、趋势单位门控、报告详情 / 修正对等已定稿决策不变。
- **接续** `.scratch/member-overview/spec.md`：`/` 从「当前成员概览」升级为「家庭卡片首页」；`/timeline` 保留为档案；待核对 / 失败 / 最近一次 / 设置口径沿用。
- **接续** `.scratch/abnormal-trends/spec.md`：该 spec 明确把「指标级检查大项」推迟，并否决「客户端维护指标名→大项映射」与「识别端新增 panel 字段」。本 spec 是它的后续需求：**接受识别端输出项目组**（维护者架构决定），但仍是报告 / 指标项级，不做指标项上的用户标签；Go 不维护映射。
- **不修改** `CONTEXT.md` 的「报告类别 / `report_type`」定义；新增「项目组」词条，并给「报告」「指标项」补一句项目组归属。
- **新增** `docs/adr/0012-item-level-panels.md`：记录「为什么不用报告级单标签、为什么分类在识别端、为什么回填走 `/classify` 而不是 reparse」。
- **保留** `.scratch/viz-and-client/spec.md` 与既有 ADR；本 spec 不重开信任 / 核对 / 单位门控决策。

---

## 14. 决策记录（2026-10-27）

| 轮次 | 问题 | 选择 |
|------|------|------|
| 1 | 浏览模型 | B 指标项级项目组 + 项目视图 |
| 1 | 导航 | A 双 Tab（家庭 / 档案） |
| 1 | 术语 | A 项目组（panel），UI「按项目」 |
| 1 | 分类集合 | B 12 + 其他 + 性激素 / 维生素/贫血 / 凝血 |
| 1 | spec 位置 | A `.scratch/panel-browse/spec.md` |
| 2 | 项目详情 | C 混合（异常项内联趋势，正常项最新值 + 点开） |
| 2 | 家庭卡 | B 项目口径 |
| 2 | 报告卡标签 | B 前 2 + `+N` |
| 2 | 项目详情报告入口 | A 完整报告列表 `/timeline?panel=` |
| 2 | 异常指标页 | A 保留 Stack，按项目分组 |
| 2 | 交付切分 | A 三单 |
| 2 | ADR | A 新增 0012 |
| 2 | `other` 行 | A 一行 + 来源类别 |

**证据边界：** 49 份 golden + 现有代码核查；没有真实用户使用数据。单标签「0 份」是按命中项数最多的模拟，具体计数会随词典标注微调，但肾功 / 血脂 / 尿酸 / 电解质在混合生化报告里被肝功吞并的结构性结论不变。人工验收仍是最终判据。
