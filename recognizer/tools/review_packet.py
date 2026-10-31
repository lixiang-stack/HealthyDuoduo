"""生成票 14/15 的医学复核工作包(不改变 review_status)。

用法(recognizer/ 下):
  uv run python tools/review_packet.py
  uv run python tools/review_packet.py --out ../.scratch/medical-standardization/review

输出:
  batch-1.csv / batch-2.csv  逐项复核表(decision/reviewer/reviewed_at/comment 留空)
  summary.md                 计数、缺口与争议项

复核人填完 CSV 后,实施侧只接受 review_decision=approve 且来源/plausible_range
完整的行;approved 前不得声称 unit_status=verified。
"""

import argparse
import csv
from collections import Counter
from pathlib import Path

import yaml

from recognizer import paths

DICT_DIR = paths.REPO_ROOT / "recognizer" / "recognizer"

REVIEW_COLUMNS = [
    "batch",
    "panel",
    "dict",
    "current_name",
    "standard_name",
    "code",
    "code_type",
    "loinc",
    "loinc_status",
    "result_kind",
    "canonical_unit",
    "unit_display",
    "accepted_units",
    "result_basis",
    "plausible_range",
    "source_status",
    "sources",
    "notes",
    "needs_plausible_range",
    "review_decision",
    "reviewer",
    "reviewed_at",
    "review_comment",
]

def load_dict_items() -> dict[str, dict]:
    items: dict[str, dict] = {}
    for path in sorted(DICT_DIR.glob("*_dict.yaml")):
        data = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
        for item in data.get("items") or []:
            items[item["name"]] = {"dict": path.name, **item}
    return items


def load_mapping() -> list[dict]:
    with (paths.REPO_ROOT / ".scratch" / "medical-standardization" / "mapping.csv").open(
        encoding="utf-8"
    ) as f:
        return list(csv.DictReader(f))


def code_of(row: dict, item: dict) -> tuple[str, str]:
    if row["wst886_code"].strip():
        return row["wst886_code"].strip(), "wst886"
    if item.get("local_code"):
        return str(item["local_code"]).strip(), "local"
    if row["loinc"].strip():
        return "", "loinc-only"
    return "", "none"


def accepted_units_of(item: dict) -> str:
    parts = []
    for accepted in item.get("accepted_units") or []:
        conversion = accepted.get("conversion") or {}
        kind = conversion.get("type", "?")
        if kind == "linear":
            parts.append(
                f"{accepted.get('unit')}->{accepted.get('ucum')} "
                f"(linear x{conversion.get('factor')}; {conversion.get('source')})"
            )
        elif kind == "equation":
            parts.append(
                f"{accepted.get('unit')}->{accepted.get('ucum')} "
                f"(equation {conversion.get('id')}; {conversion.get('source')})"
            )
        else:
            parts.append(f"{accepted.get('unit')}->{accepted.get('ucum')} ({kind})")
    return "; ".join(parts)


def plausible_of(item: dict) -> str:
    plausible = item.get("plausible_range") or {}
    if "low" not in plausible or "high" not in plausible:
        return ""
    return f"{plausible['low']}..{plausible['high']}"


def packet_rows(mapping: list[dict], items: dict[str, dict]) -> list[dict]:
    rows = []
    for row in mapping:
        item = items[row["standard_name"]]
        code, code_type = code_of(row, item)
        needs_plausible = row["result_kind"] == "quantitative" and not plausible_of(item)
        rows.append(
            {
                "batch": row["batch"],
                "panel": row["panel"],
                "dict": row["dict"],
                "current_name": row["current_name"],
                "standard_name": row["standard_name"],
                "code": code,
                "code_type": code_type,
                "loinc": row["loinc"],
                "loinc_status": row["loinc_status"],
                "result_kind": row["result_kind"],
                "canonical_unit": row["canonical_unit"],
                "unit_display": row["unit_display"],
                "accepted_units": accepted_units_of(item),
                "result_basis": str(item.get("result_basis") or ""),
                "plausible_range": plausible_of(item),
                "source_status": row["source_status"],
                "sources": row["sources"],
                "notes": row["notes"],
                "needs_plausible_range": "yes" if needs_plausible else "",
                "review_decision": "",
                "reviewer": "",
                "reviewed_at": "",
                "review_comment": "",
            }
        )
    return rows


def write_batch(path: Path, rows: list[dict]) -> None:
    with path.open("w", encoding="utf-8", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=REVIEW_COLUMNS, lineterminator="\n")
        writer.writeheader()
        writer.writerows(rows)


def summary(rows: list[dict]) -> str:
    lines = [
        "# 医学复核工作包（票 14/15）",
        "",
        "本文件由 `recognizer/tools/review_packet.py` 生成；所有条目仍为 `review_status=pending`，",
        "未经具名临床检验复核人签核不得翻 `approved`，也不得声称 `unit_status=verified`。",
        "",
        "已收到的技术复核结果：`docs/batch-1-reviewed.csv`、`docs/batch-2-reviewed.csv`、",
        "`docs/summary-reviewed.md`。复核人明确为非持证检验医师，正式医学签核仍待完成；",
        "因此本工作包只记录技术结论，不改 review_status。",
        "",
        "## 填写方式",
        "",
        "1. 在 `batch-1.csv` / `batch-2.csv` 每行填写 `review_decision`：`approve` / `reject` / `needs-info`。",
        "2. `approve` 必须同时填写 `reviewer`、`reviewed_at`，并确认 `sources` 非空；",
        "   定量项还必须有 `plausible_range`。",
        "3. 有异议或需补充来源的行填 `review_comment`，不要直接改词典。",
        "4. 复核完成后由实施侧按批准清单翻 `review_status`、重生成 golden，并跑票 13 的全量验证。",
        "",
    ]
    for batch in ("1", "2"):
        batch_rows = [r for r in rows if r["batch"] == batch]
        lines += [
            f"## Batch {batch}（{len(batch_rows)} 项）",
            "",
            "| panel | 项数 | 缺 plausible_range（定量） | source_status=pending | 仅 LOINC | 无任何码 |",
            "| --- | ---: | ---: | ---: | ---: | ---: |",
        ]
        panels = sorted({r["panel"] for r in batch_rows})
        for panel in panels:
            panel_rows = [r for r in batch_rows if r["panel"] == panel]
            lines.append(
                f"| {panel} | {len(panel_rows)} | "
                f"{sum(r['needs_plausible_range'] == 'yes' for r in panel_rows)} | "
                f"{sum(r['source_status'] == 'pending' for r in panel_rows)} | "
                f"{sum(r['code_type'] == 'loinc-only' for r in panel_rows)} | "
                f"{sum(r['code_type'] == 'none' for r in panel_rows)} |"
            )
        attention = [r for r in batch_rows if r["needs_plausible_range"] == "yes"]
        if attention:
            lines += ["", "**量化项缺 `plausible_range`：**", ""]
            lines += [f"- {r['standard_name']}（{r['canonical_unit']}）" for r in attention]
        lines.append("")

    lines += [
        "## 争议项（需明确结论）",
        "",
        "### PDW",
        "",
        "技术结论：拆成 PDW-SD（fL）与 PDW-CV（%）；当前无后缀条目保留为 local 历史兼容。",
        "实施约束：现有样本只有无后缀 PDW（fL），没有 PDW-SD/CV 真实印形证据；",
        "按 corpus gate 暂不注册新条目，待真实样本后拆分并重新复核。",
        "",
        "### D-二聚体",
        "",
        "当前：canonical `ug/L`，`result_basis=FEU`；`ng/mL FEU` 等值、`mg/L FEU` ×1000，DDU 不换算。",
        "待裁决：FEU 基准是否作为唯一 canonical；DDU 报告是否保留 `unit=null` + `unverified`。",
        "",
        "### 二氧化碳结合力",
        "",
        "技术结论：补充 `plausible_range 10..40`（成人常见参考约 22–29，放宽以挡数量级错误）；",
        "LOINC 2028-9 为 total CO2，实际测 CO2CP，保持 notes 标注。",
        "",
        "### eGFR",
        "",
        "当前：canonical `mL/min/1.73m2`；`mL/min` → `conversion: none`，输出 `unit=null` + `unverified`。",
        "待裁决：是否接受 `mL/min/1.73m²` 为唯一 canonical；`mL/min` 是否永不换算。",
        "",
        "### U/L vs IU/L",
        "",
        "当前：酶类 canonical `U/L`，`IU/L` 不默认等值；无权威换算时保持 `unverified`。",
        "待裁决：是否确认两者不可等价；若某方法学有权威换算，需逐项给出 source。",
        "",
        "### Batch 2 伪单位建模",
        "",
        "- `S/CO`：当前按 `ratio` + `unit=1` 处理；待确认是否保留 `S/CO` 仅作 `raw_unit`。",
        "- `AU`：当前按 `arbitrary` + `[arb'U]`；待确认各免疫项的 arbitrary 语义。",
        "- 滴度：当前按 `titer`；待确认条码/稀释度是否进 `value` 还是 `raw_value`。",
        "- `/HP`、`/LPF`：当前按 `microscopy` + `field: HPF|LPF`；待确认字段表达。",
        "- `个/uL`：当前 canonical `/[uL]` 计数；待确认与 `/HP` 不混用。",
        "",
        "## 复核完成后的实施清单",
        "",
        "- 按批准清单把对应 `review_status` 翻 `approved`；未批准的保持 `pending`。",
        "- 定量项必须有 `plausible_range`；无来源项不得 approved。",
        "- 重生成受影响 golden，跑 `pytest` / `go test ./...` / mobile / `e2e.py verify --golden`。",
        "- 在票 14/15 记录复核人、日期、批准/拒绝数与争议项结论。",
        "",
    ]
    return "\n".join(lines)


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--out",
        type=Path,
        default=paths.REPO_ROOT / ".scratch" / "medical-standardization" / "review",
    )
    args = parser.parse_args()

    mapping = load_mapping()
    items = load_dict_items()
    rows = packet_rows(mapping, items)
    args.out.mkdir(parents=True, exist_ok=True)
    for batch in ("1", "2"):
        write_batch(args.out / f"batch-{batch}.csv", [r for r in rows if r["batch"] == batch])
    (args.out / "summary.md").write_text(summary(rows), encoding="utf-8")
    counts = Counter(r["batch"] for r in rows)
    print(f"review packet: batch-1={counts['1']} batch-2={counts['2']} -> {args.out}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
