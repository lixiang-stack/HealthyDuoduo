# 2026-10-21 指标项「未知」成因审计

## 为什么做

时间线与详情把 `flag=unknown` 显示为「未知」，但没有回答两件事：**为什么未知**、**值不值得让家长处理**。票 02（是否进入核对队列）和票 03（原图定位）都依赖同一份事实底稿。本票只审计数据与代码路径，不改产品行为。

## 结论

1. **未知不是罕见边角**：`member=aaa` 43/188 项（22.9%），`samples/expected/report` 130/559 项（23.3%）。它也不是低置信的同义词——两组数据里低置信的未知项分别只有 2 项和 4 项。
2. **未知主要是一个结果语义，不是待办**：约 87% 的样本未知项无法靠现有「修正」消除。它们要么是定性结果（阴性/阳性/颜色）、检出限比较式（`<2.00`）、区间型结果（`70-80`），要么纸面本来就没有参考范围（比值、计算项、部分镜检项）。
3. **能靠「修正」消除的只有两类**：缺数值但参考范围可解析（10/43、9/130），以及参考范围上下界写反（1/43、8/130）；合计 `member=aaa` 11/43（25.6%）、samples 17/130（13.1%）。前提是原图上确实印着值/范围。
4. **可消除项里混着真实抽取缺陷**：例如抗 Jo-1 抗体原图印着 `1.53`，`raw_text` 与 `value` 都没有；淋巴细胞百分比原图印着 `22.8%`，抽取只拿到参考范围；cbc_04 的多栏版式把参考范围拼成 `13.5--9.5`（原图 `3.5--9.5`）。这些应当在识别端修，而不是长期靠家长逐项补录。
5. **审计未发现「参考范围不可解析」这一档**（0/43、0/130）；影像单在 ADR-0009 之后不再产出指标项，因此也不贡献未知（0/43、0/130）。当前未知的成因集中在缺值、非数值、缺参考范围、上下界写反四类。

## 数据边界与口径

- **`member=aaa`**：2026-10-21 读取共享 deploy 栈 `http://127.0.0.1:8080/api/v1`（`member_id=9`）。18 份报告、188 个指标项、43 个未知；全部 `status=success`，`unreviewed_low_confidence_count=0`。代表样本的原图已下载并按 OCR 行框裁切核对；本文只引用报告短 ID 与指标项事实，不引用患者信息。
- **samples**：`samples/expected/report/*.json` 全量 49 份、559 项、130 个未知。掩码语料（如 `ana_04`）没有原图，涉及抽取缺陷的判断只能依据 `raw_text` 与版式，已在样本条目中注明。
- **分类口径**与 `recognizer/recognizer/postprocess.py` 的 `_flag` / `internal/pipeline/flag.go` 的 `ComputeFlag` 一致：先看 value 能否数值化，再看参考范围能否解析、上下界是否写反。分类脚本见文末「复现」。

## 分布

### 按成因

| 成因 | `member=aaa` | 占未知 | samples | 占未知 |
| --- | ---: | ---: | ---: | ---: |
| 缺数值，且无参考范围 | 11 | 25.6% | 43 | 33.1% |
| value 非纯数值（定性文本） | 10 | 23.3% | 36 | 27.7% |
| 缺数值，但参考范围可解析 | 10 | 23.3% | 9 | 6.9% |
| 有数值，缺参考范围 | 8 | 18.6% | 24 | 18.5% |
| value 为区间型结果（`70-80`） | 2 | 4.7% | 2 | 1.5% |
| 参考范围上下界写反 | 1 | 2.3% | 8 | 6.2% |
| value 为单侧比较式（`<2.00`） | 1 | 2.3% | 8 | 6.2% |
| 参考范围不可解析 | 0 | 0% | 0 | 0% |
| 影像测量（影像单指标项） | 0 | 0% | 0 | 0% |
| 其他 | 0 | 0% | 0 | 0% |
| **合计** | **43** | **100%** | **130** | **100%** |

### 按报告类别

`member=aaa`：临检 25、免疫 15、激素 3、生化 0、影像 0、其他专项 0。

samples：临检 81、免疫 25、生化 9、激素 10、其他专项 5、影像 0（其中「其他专项」5 项来自 `unknown_01` 的产筛字段，value 缺失且无参考范围）。

### 与报告状态的关系

- `unknown` **不参与** `status` 派生。`run_postprocess` 只在「任一 value 为空 / 日期缺失 / 低置信占比 ≥ 50%」时降为 `partial`。
- 因此 43 个未知的 `member=aaa` 报告全部是 `success`；samples 里有 31 份报告含未知，其中 14 份是 `partial`，另有 2 份 `partial`（`glu_01`、`lft_01`）完全不含未知。全部 16 份 `partial` 中只有 6 份含低置信项。
- 结论：**未知 ↔ 待核对 不是一一对应关系**；把未知直接塞进核对队列会让「待核对」同时表示两种不同任务。

## 代表样本与代码路径

### 1. 缺数值，但参考范围可解析（可修正）

| 字段 | 值 |
| --- | --- |
| 报告 / 类别 | `c9ce457c37ec` / 免疫（`member=aaa`） |
| 指标项 | 抗 Jo-1 抗体 |
| `value` | `null` |
| `ref_range` | `0-20` |
| `raw_text` | `抗Jo-1抗体(Jo-1)  0-20  AU  流式荧光发光法` |
| `low_confidence` | `false` |
| 原图 | 同一行右侧印着 `1.53`（已裁切核对） |
| 命中路径 | `_extract_items` 的列配对/`_decompose` 未把值格并入该行 → `_flag(None, "0-20")` → unknown |
| 修正后 | 补 `value=1.53` → `ComputeFlag("1.53","0-20")=normal` |

samples 同类：`ana_04` 的抗 Jo-1/CENP B/PCNA/Nucleosome/Histone/Ribosomal P/C1q（掩码语料无原图，按 `raw_text` 判断为漏值）。

### 2. 缺数值，且无参考范围（多数不可判定）

| 字段 | 值 |
| --- | --- |
| 报告 / 类别 | `9da58c22c107` / 临检（`member=aaa`） |
| 指标项 | 尿潜血 |
| `value` | `null` |
| `ref_range` | `null` |
| `raw_text` | `尿潜血` |
| `low_confidence` | `false` |
| 原图 | 同一行左侧印着 `3+`，参考范围 `阴性` |
| 命中路径 | 双栏版式只把项目名列进 `raw_text`；`_extract_items` 未配对右栏结果格 → unknown |
| 修正后 | 即使补 `value=3+`，`ComputeFlag` 仍为 unknown（定性值不可数值化）；补参考范围也不能改变 |

同类：`cbc_01` 的中性细胞数 `4.19`（原图行内只有 `NEUT 中性细胞数 4.19`，没有参考范围）、`cbc_02` 的血小板比积 `0.13`、`lft_03` 的谷草/谷丙比值 `1.35`。这一类的「未知」是**正确结果**：没有可比较的数值对。

### 3. 有数值，缺参考范围（不可判定）

| 字段 | 值 |
| --- | --- |
| 报告 / 类别 | `139fde0e0b0c` / 临检（即 `cbc_01`，`member=aaa`） |
| 指标项 | 中性细胞数 |
| `value` | `4.19` |
| `ref_range` | `null` |
| `raw_text` | `NEUT  中性细胞数  4.19` |
| `low_confidence` | `false` |
| 命中路径 | `_flag("4.19", None)` → unknown；详情 `ocr_lines` 为 169–171 行 |
| 修正后 | 若纸面没有参考范围，修正无法消除 unknown；卡片应直说「参考范围未识别」 |

### 4. value 非纯数值：定性文本（不可判定）

| 字段 | 值 |
| --- | --- |
| 报告 / 类别 | `ua_04` / 临检（samples） |
| 指标项 | 尿葡萄糖 |
| `value` | `阴性` |
| `ref_range` | `null` |
| `raw_text` | `1 尿葡萄糖(GLU)  阴性` |
| `low_confidence` | `false` |
| 命中路径 | `_decompose` 保留定性值 → `_num` 失败 → `_flag` unknown |
| 修正后 | 仍为 unknown（语义上没有数值可比较） |

同类的非纯数值还包括 `B型`、`RhD(+)`、`淡黄`、`清澈`、`未见`、`Normal`、`核均质型`。`member=aaa` 的 10 项 text-value 全部属于这一类。

### 5. value 为单侧比较式（不可判定）

| 字段 | 值 |
| --- | --- |
| 报告 / 类别 | `ana_02` / 免疫（samples） |
| 指标项 | 抗 dsDNA 抗体 |
| `value` | `<2.00` |
| `ref_range` | `0-30` |
| `raw_text` | `16 抗dsDNA抗体(dsDNA)  <2.00  0-30  IU/mL` |
| `low_confidence` | `false` |
| 命中路径 | `_num("<2.00")` 失败 → unknown；原图语义是「低于检出限」，不是「值等于 2.00」 |
| 修正后 | 不能把 `<2.00` 改成 `2.00`（篡改纸面事实），因此现有修正无法消除 unknown。要支持需要扩展 flag 语义（单侧值比较），属产品决策 |

同类：`cbc_06`/`cbc_07` 的 C 反应蛋白 `<6.00`、`fibrosis_01` 的透明质酸 `<50.0000`。

### 6. value 为区间型结果（不可判定）

| 字段 | 值 |
| --- | --- |
| 报告 / 类别 | `ua_01` / 临检（samples） |
| 指标项 | 红细胞镜检 |
| `value` | `70-80` |
| `ref_range` | `0-3` |
| `raw_text` | `尿红细胞镜检  70-80  /HP  0-3` |
| `low_confidence` | `false` |
| 命中路径 | `_num("70-80")` 失败 → unknown |
| 修正后 | 不能擅自选 `70` 或 `80`；要支持需要区间比较语义 |

同类：`ua_01` 的尿白细胞镜检 `1-2`。

### 7. 参考范围上下界写反（抽取缺陷，可修正）

| 字段 | 值 |
| --- | --- |
| 报告 / 类别 | `cbc_04` / 临检（samples） |
| 指标项 | 白细胞计数 |
| `value` | `17.3` |
| `ref_range` | `13.5--9.5`（原图 `3.5--9.5`） |
| `raw_text` | `WBC  白细胞  17.3  13.5--9.5 109/L` |
| `low_confidence` | `false` |
| 命中路径 | `_decompose` 把相邻栏的序号 `1` 并入下限；`_flag` 检测 `lo > hi` → unknown |
| 修正后 | 改 `ref_range=3.5--9.5` → `ComputeFlag("17.3","3.5--9.5")=high` |

同族的 cbc_04 项还有 MCV（`8210--1`）、PLT（`130--31`）、淋巴细胞比率（`120--50`）、单核细胞比率（`13--10`）、嗜酸性粒细胞数（`10--0.06`）。全部是**同一份多栏版式的列归属缺陷**。

### 8. 参考范围不可解析（当前无样本）

两组数据均为 0。代码路径仍存在：`_decompose` 产出的范围串若不能被 `_RANGE_RE`/`_CMP_RE` fullmatch，`_flag` 落 unknown。当前不需要为它设计产品动作。

### 9. 影像测量（当前无样本）

ADR-0009 之后影像单走叙述段落、不产出指标项；samples 的 `us_01`/`us_02`/`us_03` 与 `member=aaa` 的 3 份影像报告都是 0 items。历史影像指标项若仍存在，会走同样 `_flag` 路径，但不在本次可观测范围内。

## unknown、低置信、partial 三者的关系

| 概念 | 来源 | 含义 | 是否会进核对队列 |
| --- | --- | --- | --- |
| `flag=unknown` | `_flag` / `ComputeFlag` | 值与参考范围无法比较 | 否 |
| `low_confidence=true` | OCR 行 score 低于阈值 | 这次抽取不可信 | 未核对的低置信项会 |
| `status=partial` | 任一 value 为空 / 日期缺失 / 低置信占比 ≥ 50% | 报告级「有缺口」 | 概览按 partial 计份数 |

- **确认**只改 `review_status`，不重算 flag；对 unknown 点确认不会消除未知。
- **真实数据证据**：`member=aaa` 的 43 个 unknown 中有 6 个 `review_status=confirmed`，flag 仍是 unknown（如红细胞计数 `3.78` / 范围 `80~5.10`、尿潜血、促甲状腺素、孕酮）。这直接证明确认与 flag 是两条独立的轴。
- **修正**才按 value × ref_range 重算 flag；只有「补上缺失的数值/范围」或「修正被拼错的数值/范围」才可能把 unknown 变成 normal/high/low。
- `pendingItems` 只收 `low_confidence && review_status === 'unreviewed'`；高置信 unknown 不在核对页，但详情页仍展示它。`member=aaa` 的 6 个低置信项里 5 个已确认，实际 `pendingItems` 只有 1 个（ANA(IIF)，unknown）。

## 哪些 unknown 能被现有「修正」消除

| 类别 | `member=aaa` | samples | 结论 |
| --- | ---: | ---: | --- |
| 缺数值 + 参考范围可解析 | 10 | 9 | **可消除**（前提：原图印着值；本文核对的 aaa 样本都印着） |
| 参考范围上下界写反 | 1 | 8 | **可消除**（照原图改范围） |
| 缺数值 + 无参考范围 | 11 | 43 | 不可消除（补值仍缺范围；部分补值可让显示更真实） |
| 有数值 + 缺参考范围 | 8 | 24 | 不可消除（纸面没有范围，不能编） |
| 定性文本 | 10 | 36 | 不可消除（无比较语义；修正只能让显示值更真实） |
| 单侧比较式 | 1 | 8 | 不可消除（现有 flag 不比较单侧值；改值即篡改） |
| 区间型结果 | 2 | 2 | 不可消除（现有 flag 不比较区间值） |
| **合计** | **43** | **130** | 可消除 11/43（25.6%）、17/130（13.1%） |

> 「可消除」不等于「应该让家长做」。其中相当一部分是识别端列归属/配对缺陷，正确处置是修数据链路；票 02 据此决定是否提供独立的「待补录」入口。

## 复现

### 1. 一条脚本（`member=aaa` + samples）

```bash
curl -sS http://127.0.0.1:8080/api/v1/members
# 找到 name=aaa 的 member_id（本次为 9）
```

```python
import glob, json, re, urllib.request

BASE = "http://127.0.0.1:8080/api/v1"
MEMBER = 9
NUM = r"\d+(?:\.\d+)?"
RANGE = re.compile(rf"^({NUM})\s*[-–—~]{{1,2}}\s*({NUM})$")
CMP = re.compile(rf"^[<≤>≥]\s*{NUM}$")
SEX = re.compile(r"^[男女][::：]\s*")

def num(s):
    try:
        return float(s.strip())
    except (TypeError, ValueError):
        return None

def cause(item):
    value, ref = item["value"], item["ref_range"]
    ref_clean = SEX.sub("", ref.strip()) if ref else None
    if value is None:
        if ref_clean and (RANGE.fullmatch(ref_clean) or CMP.fullmatch(ref_clean)):
            return "missing-value-ref-readable"
        return "missing-value-ref-null" if ref is None else "missing-value-ref-unreadable"
    if num(value) is None:
        if CMP.match(value.strip()):
            return "value-bound"
        if RANGE.fullmatch(value.strip()):
            return "value-interval"
        return "value-text"
    if ref is None:
        return "missing-ref"
    m = RANGE.fullmatch(ref_clean)
    if m and float(m.group(1)) > float(m.group(2)):
        return "ref-reversed"
    return "ref-unreadable"

def operable(item):
    """现有修正能否把 flag 从 unknown 变成可比结果(票 02 口径)。"""
    if item["flag"] != "unknown":
        return False
    value, ref = item["value"], item["ref_range"]
    ref_clean = SEX.sub("", ref.strip()) if ref else None
    if value is None and ref_clean:
        return bool(RANGE.fullmatch(ref_clean) or CMP.fullmatch(ref_clean))
    if value is not None and num(value) is not None and ref_clean:
        m = RANGE.fullmatch(ref_clean)
        return bool(m and float(m.group(1)) > float(m.group(2)))
    return False

def summarize(reports, label):
    counts, total, unknown, pending = {}, 0, 0, 0
    confirmed_unknown = operable_count = c2 = 0
    for detail in reports:
        for item in detail["items"]:
            total += 1
            unreviewed = item.get("review_status", "unreviewed") == "unreviewed"
            if item["low_confidence"] and unreviewed:
                pending += 1
            if item["flag"] == "unknown":
                unknown += 1
                if not unreviewed:
                    confirmed_unknown += 1
                if operable(item):
                    operable_count += 1
                    if not (item["low_confidence"] and unreviewed):
                        c2 += 1
                key = cause(item)
                counts[key] = counts.get(key, 0) + 1
    print(
        label, len(reports), "reports", total, "items", unknown, "unknown",
        pending, "pending", confirmed_unknown, "confirmed-unknown",
        operable_count, "operable", c2, "c2", counts,
    )

# member=aaa: 摘要 + 详情
summaries = json.load(urllib.request.urlopen(f"{BASE}/reports?member_id={MEMBER}"))["reports"]
summarize(
    [json.load(urllib.request.urlopen(f"{BASE}/reports/{s['report_id']}")) for s in summaries],
    "member=aaa",
)

# samples: expected report 已含 items
summarize(
    [json.load(open(path)) for path in sorted(glob.glob("samples/expected/report/*.json"))],
    "samples",
)
```

samples 的期望报告不含 `flag=unknown` 以外的派生字段，分类只看 `value` / `ref_range` / `flag`，无需 OCR 服务。`ana_04` 等掩码语料没有原图，只能按 `raw_text` 判断。

### 2. 原图核对（可选）

以 `cbc_01` 的中性细胞数为例：详情 API 返回 `ocr_lines` 的 169–171 行框，下载 `/api/v1/images/{report_id}` 后按行框并集裁切，即可看到 `NEUT 中性细胞数 4.19`。本文的代表样本均按此法核对；掩码语料无原图，已注明。

## 后续

- **票 02**：基于本审计判断未知是否进入核对队列（建议见该票决策文档）。
- **票 03**：让每个有 `ocr_lines` 的指标项都能打开原图定位，高置信 unknown 无需先点「修正」。
- **识别质量**：cbc_04 的列归属、`value+%` 粘连、双栏/竖排漏值应另立识别端小票修复；不要用产品核对流程长期兜底数据链路缺陷。
