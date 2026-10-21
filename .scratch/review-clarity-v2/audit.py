"""Re-audit `flag=unknown` for review-clarity-v2 ticket 01.

The audit has two datasets:

* samples: `samples/expected/report/*.json` + `samples/expected/ocr/*.json`
* member:  `GET /reports?member_id=9` + `GET /reports/{id}` on the shared stack
  (optional; samples-only runs still work offline)

Usage:
    python3 .scratch/review-clarity-v2/audit.py --self-test
    python3 .scratch/review-clarity-v2/audit.py --summary
    python3 .scratch/review-clarity-v2/audit.py --items > /tmp/unknown-items.tsv
    python3 .scratch/review-clarity-v2/audit.py --member 9 --summary

Input boundaries:
    * `samples/expected/ocr/*.json` is the OCR expectation (`txts` / `boxes` /
      `scores`); `txts` indexes are 0-based.
    * Samples without an image are masked OCR corpora. Their paper evidence is
      not available; they are marked `未核图`.
    * Member data is live and may change. The audit snapshot is 2026-10-21.
    * Image crops are manual evidence recorded in `audit.md`; this script does
      not decode images (no third-party dependencies).

Output:
    * `--summary` prints overlap, cause distribution and the operable residual.
    * `--items` prints one TSV row per unknown item.
    * `--self-test` anchors ana_04 (OCR miss) and cbc_04 (OCR line merge), plus
      the ComputeFlag vector subset used by the audit.

Cause rules are first-match (see `classify_item`):
    1. 不可比较 (value is qualitative / one-sided / interval)
    2. 参考范围不可解析 (ref is scalar or otherwise not interval/bound)
    3. OCR 行合并 (reversed/glued range string is already in OCR text and is
       recoverable from adjacent cells or a handwritten annotation)
    4. 提取器配对 (value/range is in OCR output but assigned to another item or
       field)
    5. OCR 漏识别 (value/range is absent from OCR output, or only a garbled
       fragment is present)
    6. 纸面无参考范围 (numeric value, paper image has no range in that row;
       requires image evidence, applied via EVIDENCE)
    7. 其他 (insufficient evidence / narrative report / no image)

Rules 1-5 are automated; `纸面无参考范围` and some `OCR 行合并`/`OCR 漏识别`
cases need the image evidence table. A single-cell range misread (e.g.
`80~5.10` for `3.50~5.10`) is `OCR 漏识别/范围误读`, not `OCR 行合并`; the
evidence table records that distinction.
"""

from __future__ import annotations

import argparse
import glob
import hashlib
import json
import os
import re
import sys
import urllib.request
from collections import Counter

NUM = r"\d+(?:\.\d+)?"
RANGE_RE = re.compile(rf"^({NUM})\s*[-–—~]{{1,2}}\s*({NUM})$")
CMP_RE = re.compile(rf"^([<≤>≥])\s*({NUM})")
SEX_RE = re.compile(r"^[男女][::：]\s*")
RANGE_TOKEN = re.compile(rf"{NUM}\s*[-–—~]{{1,2}}\s*{NUM}")
NUM_TOKEN = re.compile(NUM)

OCR_MISS = 'OCR 漏识别'
OCR_MERGE = 'OCR 行合并'
EXTRACTOR = '提取器配对'
PAPER_NO_RANGE = '纸面无参考范围'
INCOMPARABLE = '不可比较'
REF_UNPARSEABLE = '参考范围不可解析'
OTHER = '其他'

MEMBER_LABELS = {9: "member=aaa (id=9)"}

CAUSES = [
    OCR_MISS,
    OCR_MERGE,
    EXTRACTOR,
    PAPER_NO_RANGE,
    INCOMPARABLE,
    REF_UNPARSEABLE,
    OTHER,
]

# Hash-confirmed overlaps: sample image sha256 == member report_id.
# Offline fallback only; live runs use find_hash_overlaps() and this set is
# replaced by the computed stems.
HASH_OVERLAP_SNAPSHOT = {
    "cbc_01",
    "cbc_02",
    "glu_01",
    "tft_01",
    "ua_01",
    "us_01",
    "us_02",
    "us_03",
}

# Masked copies without an image. ana_04 has 16/16 identical items to c9ce;
# ana_05 has 9/10 identical items to 2b9319 (ANA(IIF) value differs).
MASKED_OVERLAP = {"ana_04": "c9ce457c37ec", "ana_05": "2b9319f14c5a"}

# Samples-samples same-source candidates (identical name/value/ref sets, likely
# another OCR run of the same paper). The fully deduped set excludes these; the
# member-independent 96 set keeps them and is shown as a sensitivity.
SAME_SOURCE = {
    "cbc_07": "cbc_06 同 25 项 name/value/ref；疑为同一血常规的另一 OCR 运行，1 unknown",
    "ua_07": "ua_06 同 18 项 name/value/ref；疑为同一尿常规的另一 OCR 运行，12 unknown",
    "ana_02": "c9ce457c37ec 同 16 项/同参考区间；值齐全，疑为同一报告的另一 OCR/拍摄版本，1 unknown",
    "ana_03": "2b9319f14c5a/ana_05 同 10 项/同参考区间；疑为同一报告的另一 OCR/拍摄版本，4 unknown",
    "tft_04": "770c4903bf85 同 1 项；方法字段缺失，0 unknown",
    "vitanemia_01": "c482100427ca 同 4 项；方法字段缺失，0 unknown",
    "vitanemia_02": "ffc0359a1a1a 同 4 项；方法字段缺失，0 unknown",
}
SAMPLE_DUPLICATES = set(SAME_SOURCE)

# ---------------------------------------------------------------------------
# Evidence overrides. Key = (sample stem / report_id[:12], item name).
# `verified`: image / ocr / none. `note` is the audit evidence.
# Member and samples entries are intentionally separate: member notes use the
# shared-stack ocr_lines, sample notes use golden txts indexes.
# ---------------------------------------------------------------------------

EVIDENCE: dict[tuple[str, str], dict] = {}


def add_evidence(stem: str, name: str, cause: str, subtype: str, verified: str, note: str) -> None:
    EVIDENCE[(stem, name)] = {
        "cause": cause,
        "subtype": subtype,
        "verified": verified,
        "note": note,
    }


# --- samples with images ---------------------------------------------------
add_evidence("cbc_01", "中性细胞数", PAPER_NO_RANGE, "原图空白", "image",
  "原图 NEUT 行 4.19 后无参考范围列；纸面本身无范围")
add_evidence("cbc_02", "血小板压积", PAPER_NO_RANGE, "原图空白", "image",
  "原图 血小板比积 0.13 % 行参考区间空白")
add_evidence("cbc_04", "平均红细胞体积", OCR_MERGE, "跨格粘连", "image",
  "OCR txts[46]=8210--1；原图范围 82--100")
add_evidence("cbc_04", "血小板", OCR_MERGE, "跨格粘连", "image",
  "OCR txts[50]=1/6.0 130--31；原图 125--350")
add_evidence("cbc_04", "白细胞计数", OCR_MERGE, "跨格粘连", "image",
  "OCR txts[98]=13.5--9.5 109/L；原图 3.5--9.5")
add_evidence("cbc_04", "淋巴细胞比率", OCR_MERGE, "跨格粘连", "image",
  "OCR txts[65/76/85] 含 120--50；原图 20--50")
add_evidence("cbc_04", "单核细胞比率", OCR_MERGE, "跨格粘连", "image",
  "OCR txts[77/86] 含 13--10；原图 3--10")
add_evidence("cbc_04", "淋巴细胞数", OCR_MERGE, "跨格粘连", "image",
  "OCR txts[79]=11.6111.1--3.2 109/L；值 11.61 与范围 1.1--3.2 粘连")
add_evidence("cbc_04", "嗜酸性粒细胞数", OCR_MERGE, "跨格粘连", "image",
  "OCR txts[82]=0.14 10--0.06 109/L；原图 EO# 0.03 0.02--0.52")
add_evidence("cbc_04", "嗜碱性粒细胞数", EXTRACTOR, "值/范围未归位", "image",
  "OCR txts[82] 的 0.14/10--0.06 属 BASO# 行，但被归到 EO#；原图 BASO# 0.14 0--0.06")
add_evidence("cbc_04", "血清淀粉样蛋白A", OCR_MISS, "范围误读", "image",
  "原图 SAA 17.05 参考 <10；OCR 只出 17.05，范围被误读为 410")
add_evidence("cbc_08", "红细胞计数", OCR_MISS, "范围误读", "none",
  "OCR txts[69]=80~5.10；无原图（掩码语料），原范围无法核验")
add_evidence("cbc_08", "有核红细胞绝对值", OTHER, "未核图", "none",
  "掩码语料无原图；OCR 无范围 token；b208 同类行纸面为标量 0")
add_evidence("cbc_08", "有核红细胞百分比", OTHER, "未核图", "none",
  "掩码语料无原图；OCR 无范围 token；b208 同类行纸面为标量 0")
add_evidence("lft_03", "谷草/谷丙比值", PAPER_NO_RANGE, "原图空白", "image",
  "原图 AST/ALT 1.35 行参考区间空白（计算法）")
add_evidence("lft_03", "间接胆红素", PAPER_NO_RANGE, "原图空白", "image",
  "原图 间接胆红素 10.00 行参考区间空白（计算法）")
add_evidence("lft_04", "谷草/谷丙比值", PAPER_NO_RANGE, "原图空白", "image",
  "原图 AST/ALT 0.86 行参考区间空白")
add_evidence("lft_08", "间接胆红素", OTHER, "未核图", "none",
  "掩码语料无原图；OCR 行无范围 token")
add_evidence("lft_08", "谷草/谷丙比值", OTHER, "未核图", "none",
  "掩码语料无原图；OCR 行无范围 token")
add_evidence("tft_01", "促甲状腺素", OCR_MISS, "值+范围误读", "image",
  "原图 TSH 3.39、范围 0.56-5.91；OCR 出 .9 与 0.-1")
add_evidence("tft_01", "抗甲状腺球蛋白抗体", OCR_MISS, "范围误读", "image",
  "原图 TG-Ab 0.00、范围 0.00-4.00；OCR 出 10/a1 与 0.00-.'0")
add_evidence("tft_03", "促甲状腺素", EXTRACTOR, "范围未归位", "image",
  "OCR txts[43]=0.55-4.78 uIU/ml；未归到 TSH 行")
add_evidence("tft_03", "抗甲状腺球蛋白抗体", EXTRACTOR, "范围未归位", "image",
  "OCR txts[42]=1<60；未归到 TGAb 行")
add_evidence("tft_05", "游离甲状腺素", OCR_MISS, "值缺失", "none",
  "OCR 只有行号/名称，无结果；无原图")
add_evidence("tft_05", "游离三碘甲状腺原氨酸", OCR_MISS, "值+范围缺失", "none",
  "OCR 只有名称，无结果/范围；无原图")
add_evidence("tft_05", "促甲状腺素", OCR_MISS, "值+范围缺失", "none",
  "OCR 只有名称，无结果/范围；无原图")
add_evidence("ua_01", "尿潜血", OCR_MISS, "值+范围缺失", "image",
  "原图 3+ / 阴性；OCR 行只有名称；定性语义")
add_evidence("ua_01", "尿白细胞酯酶", OCR_MISS, "值+范围缺失", "image",
  "原图 - / 阴性；OCR 行只有名称；定性语义")
add_evidence("ua_01", "尿蛋白质", OCR_MISS, "值+范围缺失", "image",
  "原图 +- / 阴性；OCR 行只有名称；定性语义")
add_evidence("ua_01", "尿葡萄糖", OCR_MISS, "值+范围缺失", "image",
  "原图 - / 阴性；OCR 行只有名称；定性语义")
add_evidence("ua_01", "尿胆原", OCR_MISS, "值+范围缺失", "image",
  "原图 norm. / 阴性；OCR 行只有名称；定性语义")
add_evidence("ua_01", "尿胆红素", OCR_MISS, "值+范围缺失", "image",
  "原图 - / 阴性；OCR 行只有名称；定性语义")
add_evidence("ua_01", "抗坏血酸", OCR_MISS, "值+范围缺失", "image",
  "原图 3+ / 阴性；OCR 行只有名称；定性语义")
add_evidence("ua_01", "尿酮体", OCR_MISS, "值+范围缺失", "image",
  "原图 - / 阴性；OCR 行只有名称；定性语义")
add_evidence("ua_01", "尿亚硝酸盐", OCR_MISS, "值+范围缺失", "image",
  "原图 - / 阴性；OCR 行只有名称；定性语义")
add_evidence("ua_01", "结晶", OCR_MISS, "范围缺失", "image",
  "原图 结晶 0 个/uL 参考 0-3；OCR 行只有 0，无范围")
add_evidence("ua_01", "非透明管型", REF_UNPARSEABLE, "标量范围", "image",
  "原图 非透明管型 0 个/uL 参考 0（标量，ComputeFlag 不接受）")
add_evidence("ua_01", "异形红细胞率", EXTRACTOR, "值未归位", "image",
  "原图 异性红细胞率 31%，参考区间空白；抽取把 31% 放进 unit")
add_evidence("ua_04", "抗坏血酸", REF_UNPARSEABLE, "标量范围", "image",
  "原图 维C 0.0 参考 0.0（标量）")
add_evidence("ua_04", "尿红细胞计数", OCR_MISS, "范围缺失", "image",
  "原图 红细胞计数 38.90 参考 0.00-25；OCR 只出 0.00，范围截断")
add_evidence("ua_05", "微白蛋白", EXTRACTOR, "范围未归位", "image",
  "OCR txts[47]=<=30；未归到微白蛋白行")
add_evidence("ua_05", "尿胆原", OCR_MISS, "范围缺失", "image",
  "原图 尿胆原 3.4 参考 <=17；OCR 未出该范围")
add_evidence("ua_05", "抗坏血酸", PAPER_NO_RANGE, "原图空白", "image",
  "原图 19 维生素C 0，参考区间空白")
add_evidence("ua_05", "颗粒管型", REF_UNPARSEABLE, "标量范围", "image",
  "原图 颗粒管型 0 /LP 参考 0（标量）")
for _name in ["尿亚硝酸盐", "尿葡萄糖", "尿蛋白质", "尿潜血", "尿酮体", "尿胆红素"]:
    add_evidence("ua_05", _name, OCR_MISS, "值+范围缺失", "image",
      "原图 - / 阴性；OCR 行只有名称；定性语义，补录后仍 unknown")
for _stem in ["ua_06", "ua_07"]:
    for _name in ["尿白细胞酯酶", "尿亚硝酸盐", "尿蛋白质", "尿潜血",
                  "尿酮体", "尿胆红素", "尿葡萄糖", "抗坏血酸"]:
        add_evidence(_stem, _name, OCR_MISS, "值+范围缺失", "none",
          "OCR 行只有名称/缩写，无结果/范围；无原图")
for _name in ["18-三体综合征", "唐氏综合征", "母龄风险",
              "人绒毛膜促性腺激素β亚单位", "开放性神经管缺陷"]:
    add_evidence("unknown_01", _name, OTHER, "未核图", "none",
      "产筛叙述体，OCR 有低风险/MOM 值但报告不承载；无原图")
add_evidence("proghcg_01", "孕酮", OCR_MISS, "值+范围缺失", "none",
  "OCR 行只有名称/单位；无原图")
add_evidence("proghcg_01", "人绒毛膜促性腺激素及β亚单位", OCR_MISS, "值缺失", "none",
  "OCR 有参考范围 0-5，无结果；无原图")
for _name in ["空腹葡萄糖", "餐后二小时葡萄糖", "餐后一小时葡萄糖"]:
    add_evidence("glu_04", _name, OCR_MISS, "值+范围缺失", "none",
      "OCR 只有名称/缩写；无原图")
add_evidence("glu_02", "餐后二小时葡萄糖", OCR_MISS, "范围误读", "image",
  "原图范围 3.9-7.8；OCR txts[43]=82-8")
for _name in ["抗Jo-1抗体", "抗CENP B抗体", "抗PCNA抗体", "抗Nucleosome抗体",
              "抗Histone抗体", "抗Ribosomal P抗体", "抗C1q抗体"]:
    add_evidence("ana_04", _name, OCR_MISS, "值缺失", "image",
      "ana_04 txts[62..98] 只有名称/AU/方法/范围，无值；原图为拍屏摩尔纹")
    add_evidence("c9ce457c37ec", _name, OCR_MISS, "值缺失", "image",
      "ana_04 = c9ce 掩码副本；ana_04 txts[62..98] 无值；原图为拍屏摩尔纹，重拍是诚实路径")

# --- member evidence -------------------------------------------------------
add_evidence("139fde0e0b0c", "中性细胞数", PAPER_NO_RANGE, "原图空白", "image",
  "原图 NEUT 行 4.19 后无参考范围列")
add_evidence("ce9a85874d67", "血小板压积", PAPER_NO_RANGE, "原图空白", "image",
  "原图 血小板比积 0.13 % 行参考区间空白")
for _name, _val in [("尿潜血", "3+"), ("尿白细胞酯酶", "-"), ("尿蛋白质", "+-"),
                    ("尿葡萄糖", "-"), ("尿胆原", "norm."), ("尿胆红素", "-"),
                    ("抗坏血酸", "3+"), ("尿酮体", "-"), ("尿亚硝酸盐", "-")]:
    add_evidence("9da58c22c107", _name, OCR_MISS, "值+范围缺失", "image",
      f"原图 {_val} / 阴性；OCR 行只有名称；定性语义")
add_evidence("9da58c22c107", "结晶", OCR_MISS, "范围缺失", "image",
  "原图 结晶 0 个/uL 参考 0-3；OCR 行只有 0，无范围")
add_evidence("9da58c22c107", "非透明管型", REF_UNPARSEABLE, "标量范围", "image",
  "原图 非透明管型 0 个/uL 参考 0（标量）")
add_evidence("9da58c22c107", "异形红细胞率", EXTRACTOR, "值未归位", "image",
  "原图 异性红细胞率 31%，参考空白；抽取把 31% 放进 unit")
add_evidence("b208c12ffe01", "淋巴细胞比率", EXTRACTOR, "值未归位", "image",
  "原图 LYMPH% 18.30，范围 20.00~50.00；抽取 value=null、unit=%")
add_evidence("b208c12ffe01", "红细胞计数", OCR_MISS, "范围误读", "image",
  "原图 3.50~5.10；OCR 行 80~5.10")
add_evidence("b208c12ffe01", "有核红细胞绝对值", REF_UNPARSEABLE, "标量范围", "image",
  "原图 NRBC# 0.00 参考 0（标量）")
add_evidence("b208c12ffe01", "有核红细胞百分比", REF_UNPARSEABLE, "标量范围", "image",
  "原图 NRBC% 0.00 参考 0（标量）")
add_evidence("98c10b840968", "淋巴细胞比率", EXTRACTOR, "值未归位", "image",
  "原图 淋巴细胞百分比 22.8%，范围 20.0-40.0；抽取 value=null、unit=22.8%")
add_evidence("bd3d484d2785", "孕酮", OCR_MERGE, "值粘连", "image",
  "OCR raw 17.97225.；原图孕酮 17.97，范围 4.73-50.74（225. 为手写批注）")
add_evidence("2b9319f14c5a", "ANA(IIF)", INCOMPARABLE, "定性文本", "image",
  "原图 阳性(+) / 阴性(-)；当前 value=1 为邻格杂散，补成定性后仍 unknown")
add_evidence("6866beaf9aeb", "促甲状腺素", OCR_MISS, "值+范围误读", "image",
  "原图 TSH 3.39、范围 0.56-5.91；OCR 出 .9 与 0.-1")
add_evidence("6866beaf9aeb", "抗甲状腺球蛋白抗体", OCR_MISS, "范围误读", "image",
  "原图 TG-Ab 0.00、范围 0.00-4.00；OCR 出 10/a1 与 0.00-.'0")

# Items where a correction can clear unknown (paper/OCR gives a numeric value
# plus a parseable interval/bound, or a recoverable reversed range).
OPERABLE_FALSE = {
    # image-verified: numeric value but paper has no range, so correction
    # cannot clear unknown.
    ("9da58c22c107", "异形红细胞率"),
}

OPERABLE_TRUE = {
    # member
    ("c9ce457c37ec", "抗Jo-1抗体"),
    ("c9ce457c37ec", "抗CENP B抗体"),
    ("c9ce457c37ec", "抗PCNA抗体"),
    ("c9ce457c37ec", "抗Nucleosome抗体"),
    ("c9ce457c37ec", "抗Histone抗体"),
    ("c9ce457c37ec", "抗Ribosomal P抗体"),
    ("c9ce457c37ec", "抗C1q抗体"),
    ("6866beaf9aeb", "促甲状腺素"),
    ("6866beaf9aeb", "抗甲状腺球蛋白抗体"),
    ("9da58c22c107", "结晶"),
    ("98c10b840968", "淋巴细胞比率"),
    ("b208c12ffe01", "淋巴细胞比率"),
    ("b208c12ffe01", "红细胞计数"),
    ("bd3d484d2785", "孕酮"),
    # samples
    ("cbc_04", "平均红细胞体积"),
    ("cbc_04", "血小板"),
    ("cbc_04", "白细胞计数"),
    ("cbc_04", "淋巴细胞比率"),
    ("cbc_04", "单核细胞比率"),
    ("cbc_04", "淋巴细胞数"),
    ("cbc_04", "嗜酸性粒细胞数"),
    ("cbc_04", "嗜碱性粒细胞数"),
    ("cbc_04", "血清淀粉样蛋白A"),
    ("glu_02", "餐后二小时葡萄糖"),
    ("tft_01", "促甲状腺素"),
    ("tft_01", "抗甲状腺球蛋白抗体"),
    ("tft_03", "促甲状腺素"),
    ("tft_03", "抗甲状腺球蛋白抗体"),
    ("ua_01", "结晶"),
    ("ua_04", "尿红细胞计数"),
    ("ua_05", "微白蛋白"),
    ("ua_05", "尿胆原"),
}


def num(s):
    if s is None:
        return None
    try:
        return float(s.strip())
    except (TypeError, ValueError):
        return None


def parse_ref(ref):
    if ref is None:
        return None
    r = SEX_RE.sub("", ref.strip())
    m = RANGE_RE.fullmatch(r)
    if m:
        lo, hi = float(m.group(1)), float(m.group(2))
        return {"kind": "interval", "lo": lo, "hi": hi, "reversed": lo > hi}
    m = CMP_RE.match(r)
    if m:
        return {"kind": "cmp", "op": m.group(1), "b": float(m.group(2))}
    return {"kind": "other"}


def compute_flag(value, ref):
    """Same semantics as internal/pipeline/flag.go ComputeFlag."""
    v = num(value)
    if v is None or ref is None:
        return "unknown"
    parsed = parse_ref(ref)
    if parsed is None or parsed["kind"] == "other":
        return "unknown"
    if parsed["kind"] == "interval":
        if parsed["reversed"]:
            return "unknown"
        if parsed["lo"] <= v <= parsed["hi"]:
            return "normal"
        return "high" if v > parsed["hi"] else "low"
    bound = parsed["b"]
    upper = parsed["op"] in "<≤"
    if upper and v < bound:
        return "normal"
    if upper:
        return "high"
    return "normal" if v > bound else "low"


def norm(s):
    s = (s or "").replace("–", "-").replace("—", "-").replace("～", "~")
    return re.sub(r"\s+", "", s.replace("−", "-"))


def load_json(path):
    with open(path) as handle:
        return json.load(handle)


def sha256_file(path):
    with open(path, "rb") as handle:
        return hashlib.sha256(handle.read()).hexdigest()


def row_text(item, txts):
    parts = []
    for n in item.get("ocr_line_numbers") or []:
        if isinstance(n, int) and 0 <= n < len(txts):
            parts.append(txts[n])
    if item.get("raw_text"):
        parts.append(item["raw_text"])
    return " ".join(parts)


def tokens_outside(item, text):
    """Numeric tokens that are not name/ref/unit or a 1..30 row index."""
    excluded = norm(item.get("name", "")) + norm(item.get("ref_range", ""))
    excluded += norm(item.get("unit", "")) + norm(item.get("value", ""))
    out = []
    for m in NUM_TOKEN.finditer(norm(text)):
        tok = m.group(0)
        if tok in excluded:
            continue
        if len(tok) == 1 and 1 <= int(tok) <= 30:
            continue
        out.append(tok)
    return out


def classify_item(item, txts, key_prefix, use_evidence=True):
    """First-match classification. See module docstring for the order."""
    override = EVIDENCE.get((key_prefix, item["name"])) if use_evidence else None
    if override:
        return dict(override, source="evidence")

    v = item.get("value")
    ref = item.get("ref_range")
    rp = parse_ref(ref)
    text = row_text(item, txts)
    nr = norm(text)
    nref = norm(ref) if ref else ""
    nv = norm(v) if v else ""

    # 1. semantic incomparability
    if v is not None and num(v) is None:
        if CMP_RE.match(v.strip()):
            return _result(INCOMPARABLE, "单侧比较", "value 不是纯数值", "rule")
        if RANGE_RE.fullmatch(v.strip()):
            return _result(INCOMPARABLE, "区间值", "value 不是纯数值", "rule")
        return _result(INCOMPARABLE, "定性文本", "value 不是纯数值", "rule")

    # 2. scalar / otherwise unparseable reference range
    if ref is not None and rp and rp["kind"] == "other":
        return _result(REF_UNPARSEABLE, "标量范围", "ref 不是区间或单侧比较", "rule")

    # 3. reversed interval / glued multi-number range
    if ref is not None and rp and rp["kind"] == "interval" and rp["reversed"]:
        if nref and nref in nr:
            return _result(OCR_MERGE, "跨格粘连", "ref 反向且原文出现在 OCR 行文本", "rule")
        return _result(OCR_MISS, "范围误读", "ref 反向但原串不在 OCR 行文本", "rule")
    if ref is not None and len(RANGE_TOKEN.findall(ref)) > 1:
        if nref and nref in nr:
            return _result(OCR_MERGE, "跨格粘连", "ref 粘连且原文出现在 OCR 行文本", "rule")
        return _result(OTHER, "未核图", "ref 粘连但原串不在 OCR 行文本", "rule")

    # 4. value missing, ref present
    if v is None and ref is not None:
        cands = tokens_outside(item, text)
        if cands:
            return _result(EXTRACTOR, "值未归位", f"OCR 行文本候选 {cands[:4]}", "rule")
        return _result(OCR_MISS, "值缺失", "OCR 行文本无值候选", "rule")

    # 5. numeric value, ref missing
    if v is not None and num(v) is not None and ref is None:
        rng = RANGE_TOKEN.findall(text)
        for t in rng:
            if norm(t) not in nv:
                return _result(EXTRACTOR, "范围未归位", f"OCR 行文本范围候选 {rng[:3]}", "rule")
        cands = tokens_outside(item, text)
        if cands:
            return _result(OCR_MISS, "范围缺失", f"OCR 行文本数值 {cands[:4]} 但无范围", "rule")
        return _result(OTHER, "未核图", "无范围 token", "rule")

    # 6. value and ref both missing
    if v is None and ref is None:
        cands = tokens_outside(item, text)
        if cands:
            return _result(EXTRACTOR, "值/范围未归位", f"OCR 行文本候选 {cands[:4]}", "rule")
        if nr:
            return _result(OCR_MISS, "值+范围缺失", "OCR 行文本只有名称", "rule")
        return _result(OTHER, "未核图", "OCR 无行文本", "rule")

    return _result(OTHER, "未核图", "", "rule")


def _result(cause, subtype, note, source):
    return {"cause": cause, "subtype": subtype, "verified": "none",
            "note": note, "source": source}


def validate_evidence(sample_rows, member_rows):
    """Catch typo'd evidence keys that would silently lower the headline."""
    known = {(r["report"], r["item"]["name"]) for r in sample_rows + member_rows}
    report_keys = {r["report"] for r in sample_rows + member_rows}
    for key in list(EVIDENCE) + list(OPERABLE_TRUE):
        if key[0] in report_keys:
            assert key in known, f"evidence key not found in unknown rows: {key}"


def is_backfill(item):
    """Same predicate as ticket 02's isBackfillItem (structural, not evidence)."""
    if compute_flag(item.get("value"), item.get("ref_range")) != "unknown":
        return False
    rp = parse_ref(item.get("ref_range"))
    if item.get("value") is None:
        return bool(rp and rp["kind"] in ("interval", "cmp"))
    if num(item.get("value")) is not None and rp and rp["kind"] == "interval":
        return rp["reversed"]
    return False


def operable(key, result):
    if key in OPERABLE_TRUE:
        return True
    if key in OPERABLE_FALSE:
        return False
    if result["cause"] in (INCOMPARABLE, PAPER_NO_RANGE, REF_UNPARSEABLE):
        return False
    # Qualitative OCR misses are data gaps, but capturing them still cannot
    # make ComputeFlag comparable. The note marker keeps the explicit
    # allowlist small.
    if "定性" in result["note"] or "参考区间空白" in result["note"] \
            or "参考空白" in result["note"]:
        return False
    return None


# ---------------------------------------------------------------------------
# Loaders
# ---------------------------------------------------------------------------

def make_row(dataset, report_key, item, texts):
    result = classify_item(item, texts, report_key)
    return {
        "dataset": dataset,
        "report": report_key,
        "item": item,
        "result": result,
        "operable": operable((report_key, item["name"]), result),
        "is_backfill": is_backfill(item),
    }


def load_samples(root):
    rows = []
    for path in sorted(glob.glob(os.path.join(root, "samples/expected/report/*.json"))):
        stem = os.path.basename(path)[:-5]
        report = load_json(path)
        ocr_path = os.path.join(root, "samples/expected/ocr", stem + ".json")
        ocr = load_json(ocr_path)
        for item in report["items"]:
            if compute_flag(item.get("value"), item.get("ref_range")) == "unknown":
                rows.append(make_row("samples", stem, item, ocr["txts"]))
    return rows


def find_hash_overlaps(root, member_ids):
    overlaps = {}
    for path in glob.glob(os.path.join(root, "samples/*")):
        if os.path.isdir(path) or path.endswith((".json", ".md")):
            continue
        stem = os.path.splitext(os.path.basename(path))[0]
        if stem.startswith("expected"):
            continue
        digest = sha256_file(path)
        if digest in member_ids:
            overlaps[stem] = digest
    return overlaps


def fetch_member(base_url, member_id):
    with urllib.request.urlopen(
            f"{base_url}/reports?member_id={member_id}", timeout=10) as response:
        summaries = json.load(response)["reports"]
    reports = []
    for summary in summaries:
        with urllib.request.urlopen(
                f"{base_url}/reports/{summary['report_id']}", timeout=20) as response:
            reports.append(json.load(response))
    return reports


def load_member_rows(reports):
    rows = []
    for report in reports:
        rid = report["report_id"][:12]
        for item in report["items"]:
            if compute_flag(item.get("value"), item.get("ref_range")) == "unknown":
                rows.append(make_row("member", rid, item, []))
    return rows


# ---------------------------------------------------------------------------
# Reporting
# ---------------------------------------------------------------------------

def count(rows, key="cause"):
    return Counter(r["result"][key] for r in rows)


def print_distribution(rows, label):
    print(f"\n## {label} ({len(rows)} unknown)")
    print("| 根因 | 数量 | 占 unknown |")
    print("| --- | ---: | ---: |")
    c = count(rows)
    for cause in CAUSES:
        n = c.get(cause, 0)
        if n:
            print(f"| {cause} | {n} | {n / len(rows):.1%} |")
    print(f"| **合计** | **{len(rows)}** | **100%** |")


def print_sources(rows, label):
    c = Counter(r["result"]["source"] for r in rows)
    print(f"\n{label} 分类来源：规则 {c.get('rule', 0)} / 证据表 {c.get('evidence', 0)}")


def print_operable(rows, label):
    print(f"\n## {label} 可操作残量")
    print("| 根因 | unknown | 可被现有修正消除 | 其中 isBackfillItem | "
          "其中低置信未核对* | 未核实/不可判定 | 不处理/不可消除 |")
    print("| --- | ---: | ---: | ---: | ---: | ---: | ---: |")
    for cause in CAUSES:
        subset = [r for r in rows if r["result"]["cause"] == cause]
        if not subset:
            continue
        op = [r for r in subset if r["operable"] is True]
        bf = [r for r in op if r["is_backfill"]]
        low_unreviewed = [
            r for r in op
            if r["item"].get("low_confidence")
            and r["item"].get("review_status", "unreviewed") == "unreviewed"
        ]
        unverified = sum(1 for r in subset if r["operable"] is None)
        no_action = sum(1 for r in subset if r["operable"] is False)
        print(f"| {cause} | {len(subset)} | {len(op)} | {len(bf)} | "
              f"{len(low_unreviewed)} | {unverified} | {no_action} |")
    op_all = [r for r in rows if r["operable"] is True]
    print(f"| **合计** | **{len(rows)}** | **{len(op_all)}** | "
          f"**{len([r for r in op_all if r['is_backfill']])}** | "
          f"**{len([r for r in op_all if r['item'].get('low_confidence') and r['item'].get('review_status', 'unreviewed') == 'unreviewed'])}** | "
          f"**{sum(1 for r in rows if r['operable'] is None)}** | "
          f"**{sum(1 for r in rows if r['operable'] is False)}** |")
    print("* samples 无 review_status，按 low_confidence 代理；member 为精确的 unreviewed 计数。")


def print_items(rows):
    header = (
        "dataset\treport\titem\tvalue\tref_range\tlow_confidence\t"
        "review_status\tcause\tsubtype\tverified\toperable\tis_backfill\t"
        "source\tnote"
    )
    print(header)
    for row in rows:
        item = row["item"]
        print("\t".join(str(x) for x in [
            row["dataset"], row["report"], item["name"], item.get("value"),
            item.get("ref_range"), item.get("low_confidence"),
            item.get("review_status", ""), row["result"]["cause"],
            row["result"]["subtype"], row["result"]["verified"],
            row["operable"], row["is_backfill"], row["result"]["source"],
            row["result"]["note"],
        ]))


def summarize(rows, reports, overlaps, label, member_rows=None,
              member_label="member", member_live=False):
    print(f"# {label}")
    print(f"\nreports={len(reports)} items={sum(len(r['items']) for r in reports)} "
          f"unknown={len(rows)}")
    print("\n## 重叠")
    for stem, digest in sorted(overlaps.items()):
        print(f"- {stem}: sha256={digest[:12]} (member report_id 完全相同)")
    for stem, rid in MASKED_OVERLAP.items():
        print(f"- {stem}: 掩码副本 -> {rid}（无原图；项目身份比对见 audit.md）")
    for stem, note in SAME_SOURCE.items():
        print(f"- {stem}: samples 内部同源候选（完全去重排除；member-去重 96 保留）：{note}")

    print_distribution(rows, "原始分布")
    print_sources(rows, "原始分布")
    print_operable(rows, "全部样本")

    overlap_stems = set(overlaps) if member_live else HASH_OVERLAP_SNAPSHOT
    independent = [r for r in rows if r["report"] not in overlap_stems
                   and r["report"] not in MASKED_OVERLAP]
    if len(independent) != len(rows):
        print_distribution(independent, "member-去重样本（保留 samples 内部同源）")
        print_sources(independent, "member-去重样本（保留 samples 内部同源）")
        print_operable(independent, "member-去重样本（保留 samples 内部同源）")

    fully_deduped = [r for r in independent if r["report"] not in SAMPLE_DUPLICATES]
    if len(fully_deduped) != len(independent):
        print_distribution(fully_deduped, "完全去重样本（再排除 samples 内部同源）")
        print_sources(fully_deduped, "完全去重样本（再排除 samples 内部同源）")
        print_operable(fully_deduped, "完全去重样本（再排除 samples 内部同源）")

    if member_rows:
        print_distribution(member_rows, member_label)
        print_sources(member_rows, member_label)
        print_operable(member_rows, member_label)
        overlap_prefixes = {digest[:12] for digest in overlaps.values()}
        member_independent = [r for r in member_rows
                              if r["report"] not in overlap_prefixes]
        combined = fully_deduped + member_independent
        print_distribution(combined, "完全去重合并（member + samples）")
        print_sources(combined, "完全去重合并（member + samples）")
        print_operable(combined, "完全去重合并（member + samples）")


# ---------------------------------------------------------------------------
# Self-test
# ---------------------------------------------------------------------------

def self_test(root):
    # ComputeFlag parity vectors (same subset as schemas/testdata/flag.vectors.json)
    assert compute_flag("7.33", "4--10") == "normal"
    assert compute_flag("17.3", "13.5--9.5") == "unknown"
    assert compute_flag("17.3", "3.5--9.5") == "high"
    assert compute_flag("17.05", "<10") == "high"
    assert compute_flag("0", "0") == "unknown"
    assert compute_flag("22.8", "20.0-40.0") == "normal"
    assert compute_flag("<2.00", "0-30") == "unknown"
    assert compute_flag("70-80", "0-3") == "unknown"
    assert compute_flag(None, "0-20") == "unknown"
    assert compute_flag("1", None) == "unknown"

    vectors = load_json(os.path.join(
        root, "schemas/testdata/flag.vectors.json"))["cases"]
    for case in vectors:
        assert compute_flag(case["value"], case["ref"]) == case["flag"], case

    cbc_01_image = os.path.join(root, "samples/cbc_01.jpeg")
    cbc_01_sha = sha256_file(cbc_01_image)
    assert cbc_01_sha.startswith("139fde0e0b0c"), cbc_01_sha
    assert find_hash_overlaps(root, {cbc_01_sha}) == {"cbc_01": cbc_01_sha}
    assert MASKED_OVERLAP["ana_04"] == "c9ce457c37ec"
    assert len(load_json(os.path.join(
        root, "samples/expected/report/ana_04.json"))["items"]) == 16
    assert len(load_json(os.path.join(
        root, "samples/expected/report/cbc_01.json"))["items"]) == 25

    ana = load_json(os.path.join(root, "samples/expected/report/ana_04.json"))
    ana_ocr = load_json(os.path.join(root, "samples/expected/ocr/ana_04.json"))
    ana_rows = load_samples(root)
    ana_rows = [r for r in ana_rows if r["report"] == "ana_04"]
    assert len(ana_rows) == 8
    jo = next(r for r in ana_rows if r["item"]["name"] == "抗Jo-1抗体")
    assert jo["result"]["cause"] == OCR_MISS
    jo_item = next(i for i in ana["items"] if i["name"] == "抗Jo-1抗体")
    assert classify_item(jo_item, ana_ocr["txts"], "ana_04",
                         use_evidence=False)["cause"] == OCR_MISS
    ana_tail = " ".join(ana_ocr["txts"][62:99])
    for value in ["1.53", "0.47", "1.02", "2.11", "1.38", "0.46"]:
        assert value not in ana_tail, (value, ana_tail)
    for name in ["抗Jo-1抗体", "抗CENP B抗体", "抗PCNA抗体",
                 "抗Nucleosome抗体", "抗Histone抗体", "抗Ribosomal P抗体"]:
        item = next(i for i in ana["items"] if i["name"] == name)
        row = next(r for r in ana_rows if r["item"]["name"] == name)
        assert row["result"]["cause"] == OCR_MISS
        assert item["value"] is None

    cbc = load_json(os.path.join(root, "samples/expected/report/cbc_04.json"))
    cbc_ocr = load_json(os.path.join(root, "samples/expected/ocr/cbc_04.json"))
    cbc_rows = [r for r in load_samples(root) if r["report"] == "cbc_04"]
    assert len(cbc_rows) == 9
    for name in ["平均红细胞体积", "血小板", "白细胞计数", "淋巴细胞比率",
                 "单核细胞比率", "淋巴细胞数", "嗜酸性粒细胞数"]:
        row = next(r for r in cbc_rows if r["item"]["name"] == name)
        assert row["result"]["cause"] == OCR_MERGE, (name, row["result"])
    assert sum(1 for r in cbc_rows if r["result"]["cause"] == OCR_MERGE) == 7
    wbc_item = next(i for i in cbc["items"] if i["name"] == "白细胞计数")
    assert classify_item(wbc_item, cbc_ocr["txts"], "cbc_04",
                         use_evidence=False)["cause"] == OCR_MERGE
    rule_merges = sum(
        1 for item in cbc["items"]
        if compute_flag(item.get("value"), item.get("ref_range")) == "unknown"
        and classify_item(item, cbc_ocr["txts"], "cbc_04",
                          use_evidence=False)["cause"] == OCR_MERGE
    )
    assert rule_merges == 7, rule_merges
    assert "8210--1" in cbc_ocr["txts"][46]
    assert "130--31" in cbc_ocr["txts"][50]
    assert "13.5--9.5" in cbc_ocr["txts"][98]
    baso = next(r for r in cbc_rows if r["item"]["name"] == "嗜碱性粒细胞数")
    assert baso["result"]["cause"] == EXTRACTOR
    saa = next(r for r in cbc_rows if r["item"]["name"] == "血清淀粉样蛋白A")
    assert saa["result"]["cause"] == OCR_MISS

    rows = load_samples(root)
    validate_evidence(rows, [])
    assert len(rows) == 130, len(rows)
    independent = [r for r in rows if r["report"] not in HASH_OVERLAP_SNAPSHOT
                   and r["report"] not in MASKED_OVERLAP]
    assert len(independent) == 96, len(independent)
    c = Counter(r["result"]["cause"] for r in independent)
    assert c[OCR_MISS] == 35, c
    assert c[INCOMPARABLE] == 35, c
    assert c[OCR_MERGE] == 7, c
    assert c[EXTRACTOR] == 4, c
    assert c[PAPER_NO_RANGE] == 4, c
    assert c[REF_UNPARSEABLE] == 2, c
    assert c[OTHER] == 9, c

    fully = [r for r in independent if r["report"] not in SAMPLE_DUPLICATES]
    assert len(fully) == 78, len(fully)
    c2 = Counter(r["result"]["cause"] for r in fully)
    assert c2[OCR_MISS] == 27, c2
    assert c2[INCOMPARABLE] == 25, c2
    assert c2[OCR_MERGE] == 7, c2
    assert c2[EXTRACTOR] == 4, c2
    assert c2[PAPER_NO_RANGE] == 4, c2
    assert c2[REF_UNPARSEABLE] == 2, c2
    assert c2[OTHER] == 9, c2
    print("self-test ok: ana_04 OCR miss, cbc_04 line merge, "
          "96 member-independent / 78 fully-deduped unknown")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", default=os.getcwd(),
                        help="repo root (default: cwd)")
    parser.add_argument("--member", type=int, default=None,
                        help="member id to fetch from the shared stack")
    parser.add_argument("--base-url", default="http://127.0.0.1:8080/api/v1")
    parser.add_argument("--summary", action="store_true")
    parser.add_argument("--items", action="store_true")
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args(argv)

    if args.self_test:
        self_test(args.root)
        return 0

    sample_rows = load_samples(args.root)
    sample_reports = [load_json(p)
                      for p in sorted(glob.glob(os.path.join(
                          args.root, "samples/expected/report/*.json")))]

    member_reports = []
    member_rows = []
    overlaps = {}
    if args.member is not None:
        try:
            member_reports = fetch_member(args.base_url, args.member)
        except (OSError, ValueError, KeyError) as exc:  # shared stack may be offline
            print(f"member={args.member} unavailable ({exc}); "
                  "continuing samples-only", file=sys.stderr)
        else:
            member_ids = {r["report_id"] for r in member_reports}
            overlaps = find_hash_overlaps(args.root, member_ids)
            member_rows = load_member_rows(member_reports)
            print(f"member={args.member}: {len(member_reports)} reports, "
                  f"{sum(len(r['items']) for r in member_reports)} items, "
                  f"{len(member_rows)} unknown", file=sys.stderr)
            print("hash overlaps: " + ", ".join(
                f"{k}->{v[:12]}" for k, v in sorted(overlaps.items())), file=sys.stderr)

    validate_evidence(sample_rows, member_rows)

    if args.items:
        print_items(sample_rows + member_rows)
        return 0

    if args.summary:
        member_label = MEMBER_LABELS.get(
            args.member, f"member={args.member}" if args.member is not None else "member")
        summarize(sample_rows, sample_reports, overlaps, "samples", member_rows,
                  member_label=member_label, member_live=bool(member_reports))
        return 0

    parser.print_help()
    return 1


if __name__ == "__main__":
    sys.exit(main())
