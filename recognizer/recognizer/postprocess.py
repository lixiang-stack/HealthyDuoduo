"""规则化后处理 v1(实施计划 P2-1/2):OCR 结果 → 结构化报告。

纯函数、无 IO:输入 contract.OCRResult(+可选日期兜底),输出 contract.Report。
词典是数据文件 cbc_dict.yaml(决策 #8:只有规范名/别名/已知单位集,不内置参考值;
参考范围一律从检查单抽取)。

版式覆盖(P2 的 5 张真实脱敏样本):
- 表头行(项目名称/检验项目 + 结果 + 单位/参考值/参考区间)定义列角色中心;
- 行按 OCR box 的 y 聚带,格按 x 中心就近归到列角色;词典匹配到「项目名称」
  或「代号/缩写」列(名称列失败时用代号兜底,如 cbc_04 的 MCH/名称误写);
- 熔断行按内容拆解:名熔值(平均血红蛋白浓度324)、单位熔范围(*10~9/L 3.5-9.5)、
  范围熔单位(130--175 g/L)、值熔范围熔单位(11.61|1.1--3.2|10^9/L→OCR 乱珠)。

低置信(决策 #7 原义,验收轮已收敛):任一组成行 score < LOW_SCORE_THRESHOLD
→ 该项 low_confidence、报告 ≥partial。单位解析损耗(如 109/L、1012/L)一律
保留原文记录,不改变置信标记;threshold 与单位语义供人工核对与后续规一化用。
未解析日期且无兜底 → report_date=null+partial;类别失败或无产出 → failed。

样本收窄边界(review):规则与词典目前由 5 张已入库真实样本驱动;新样本若引入
新版式/新指标名,预期行为是「未命中的行被跳过、指标缺失 → partial/failed」,
不会崩溃。扩展点按优先级:
1. 词典数据文件 cbc_dict.yaml;
2. 改动后重跑 `python -m recognizer.golden` 并人工核对 raw_text;
3. 新版式(表头关键词/列距/熔断形态不匹配)才动本文件解析规则,
   且须先以真实样本落 samples/expected/ocr(golden)后再改。
"""

import re
from dataclasses import dataclass, field
from functools import lru_cache
from pathlib import Path

import yaml

from .contract import Flag, OCRResult, Report, ReportItem, Status

DICT_PATH = Path(__file__).parent / "cbc_dict.yaml"

# 决策 #7:任一组成行 score < 阈值 → 项 low_confidence、报告 ≥partial(固定常量)
LOW_SCORE_THRESHOLD = 0.8

# 类别不识别时的报告类别(review 决定用英文占位,区别于「血常规」等正名)
UNKNOWN_REPORT_TYPE = "unknown"

# 日期标签,按优先级降序取首个命中行里的日期(打印/申请最低;检验最高)
_DATE_LABELS = (
    "检验时间", "检验日期", "检测时间", "检测日期", "采样时间", "采集时间", "标本采集时间",
    "核收时间", "报告时间", "报告日期", "打印时间", "申请时间",
)
_DATE_RE = re.compile(r"(\d{4})[-/.年](\d{1,2})[-/.月]?(\d{1,2})")
_DATE_LINE_RE = re.compile(r"^(\d{4}-\d{2}-\d{2})\b")

_NUM = r"\d+(?:\.\d+)?"
_NUMBER_RE = re.compile(rf"^{_NUM}$")  # 纯正数(含小数)
_RANGE_RE = re.compile(rf"({_NUM})\s*[-–—~]{{1,2}}\s*({_NUM})")  # a-b / a--b / a~b
_CMP_RE = re.compile(rf"([<≤>≥])\s*({_NUM})")  # <v / ≤v / >v / ≥v
_NUMBER_TAIL_RE = re.compile(rf"({_NUM})$")  # 比较式熔断格首部的数值(38.201<8 mg/L)
_ARROW_RE = re.compile(r"[↑↓▲▼]+")
_SERIAL_RE = re.compile(r"^\d+[.、．]?[\s]*")  # 行首序号前缀(8 嗜酸性粒细胞)
_STAR_RE = re.compile(r"^[*＊※✱]+")  # 行首星号前缀(*白细胞)
_SEX_PREFIX_RE = re.compile(r"^[男女][::：]\s*")  # 性别条件范围前缀(男:0-15)
_SEX_LABEL_RE = re.compile(r"[男女][::：]")  # 独立性别标签(男:/女:)
_HINT_RE = re.compile(r"[\d%/^~\-–—<≤>≥/]")  # 含任一数值/范围/单位符号 → 值得拆解
_UNIT_WORD_RE = re.compile(r"[A-Za-z]{1,4}")  # 短纯字母单位(fL、pg)
_ASCII_UNIT_RE = re.compile(r"[0-9A-Za-z.%^~/*\-]+")  # ASCII 数值/单位字符串(g/L、109/L、1012/L)

# 表头关键词 → 列角色
ROLE_HEADERS: dict[str, tuple[str, ...]] = {
    "name": ("项目名称", "检验项目"),
    "value": ("结果", "测定值"),
    "unit": ("单位",),
    "ref": ("参考值", "参考区间", "参考范围"),
    "aux": ("序号", "代码", "代号", "缩写", "简称"),
}

# 行带容差系数 × 该带当前最大行高;0.7 经 5 张已入库样本实测校准:
# 过小(≤0.55)会拆散左右半栏垂直错位 ≤ 行高的同一视觉行,过大(≥0.8)会把紧凑排版的相邻行并带。
_BAND_TOLERANCE_FACTOR = 0.7

# 碎片配对 dy 上限 = 该半栏名称行距中位数 × 系数;防止跨行碎片污染(咬合到错误行)。
_PIECE_CAP_FACTOR = 0.6
_PIECE_CAP_MIN_PX = 6.0
_PIECE_CAP_DEFAULT_PX = 25.0

# cell 归列时允许的最大偏离(像素);页面级垃圾行(标题/落款)不应占用表列
_ASSIGN_TOLERANCE_PX = 180


@dataclass(frozen=True)
class DictItem:
    name: str
    aliases: tuple[str, ...]
    units: frozenset[str]


@dataclass
class CBCDict:
    report_type: str
    title_keywords: tuple[str, ...]
    min_matched_items: int
    items: tuple[DictItem, ...]
    alias_map: dict[str, DictItem] = field(default_factory=dict)


@lru_cache(maxsize=1)
def load_dict() -> CBCDict:
    """加载/缓存 cbc_dict.yaml;改词典文件后重启进程即生效。"""
    raw = yaml.safe_load(DICT_PATH.read_text(encoding="utf-8"))
    items = tuple(
        DictItem(
            name=it["name"],
            aliases=tuple(it.get("aliases", [])) + (it["name"],),
            units=frozenset(u.lower() for u in it.get("units", [])),
        )
        for it in raw["items"]
    )
    alias_map: dict[str, DictItem] = {}
    for it in items:
        for a in it.aliases:
            alias_map.setdefault(a, it)
    d = CBCDict(
        report_type=raw["report_type"],
        title_keywords=tuple(raw["category"]["title_keywords"]),
        min_matched_items=int(raw["category"]["min_matched_items"]),
        items=items,
        alias_map=alias_map,
    )
    # 同长相位最长别名优先(前缀熔断匹配用)
    return d


@dataclass
class _Cell:
    text: str
    score: float
    x0: float
    y0: float
    x1: float
    y1: float

    @property
    def cx(self) -> float:
        return (self.x0 + self.x1) / 2

    @property
    def cy(self) -> float:
        return (self.y0 + self.y1) / 2

    @property
    def h(self) -> float:
        return self.y1 - self.y0


def _cells(ocr: OCRResult) -> list[_Cell]:
    """行文本 + box 多边形 → 以包络矩形表达的格。"""
    out = []
    for text, box, score in zip(ocr.txts, ocr.boxes, ocr.scores):
        xs = [p[0] for p in box]
        ys = [p[1] for p in box]
        out.append(_Cell(text=text, score=score, x0=min(xs), y0=min(ys), x1=max(xs), y1=max(ys)))
    return out


def _bands(cells: list[_Cell]) -> list[list[_Cell]]:
    """按 y 聚带:相邻格中心差 ≤ 0.55×最大行高视为同一视觉行。
    行内按 x 排序,重建印刷表格的阅读顺序。"""
    ordered = sorted(cells, key=lambda c: (c.cy, c.cx))
    bands: list[list[_Cell]] = []
    for c in ordered:
        if bands:
            tol = _BAND_TOLERANCE_FACTOR * max(x.h for x in bands[-1])
            if c.cy - bands[-1][0].cy <= tol:
                bands[-1].append(c)
                continue
        bands.append([c])
    for b in bands:
        b.sort(key=lambda c: c.cx)
    return bands


@lru_cache(maxsize=1)
def _unit_literals() -> tuple[str, ...]:
    """全部词典单位的字面量(含 * 前缀变体), longest-first 供前缀剥离。"""
    units: set[str] = set()
    for it in load_dict().items:
        units |= it.units
    out: list[str] = []
    for u in units:
        out.append(u)
        if not u.startswith("*"):
            out.append("*" + u)
    return tuple(sorted(set(out), key=len, reverse=True))


def _decompose(text: str) -> tuple[float | None, str | None, str | None]:
    """把一格内容尽力拆成 (value, ref_range, unit)。

    支持纯值(128)、纯单位(g/L)、纯范围(4--10)、单位熔范围(*10~9/L 3.5-9.5)、
    范围熔单位(130--175 g/L)、值熔范围熔单位(11.61|1.1--3.2|109/L);
    拆不出的残片不进结构化字段,由 raw_text 保留原文供人工核对。
    """
    s = text.strip()
    # 纯中文非数字文本(如「结果」「审核者：」)不产生任何 fragment,防止伪单位;
    # 短 ASCII 字母串(如 fL、pg)是合法单位形
    if not s or (
        not _HINT_RE.search(s) and not _UNIT_WORD_RE.fullmatch(s)
    ):
        return None, None, None
    s = _ARROW_RE.sub("", s).strip()
    if not s:
        return None, None, None
    value: float | None = None
    ref: str | None = None
    unit: str | None = None
    # 词典单位字面前缀(含 * 前缀形态);单位按命中的原文切片保留大小写
    rest = s
    unit_from_prefix = None
    for lit in _unit_literals():
        if rest.lower().startswith(lit.lower()):
            unit_from_prefix = rest[: len(lit)]
            rest = rest[len(lit) :].strip()
            break
    m = _RANGE_RE.search(rest)
    if m:
        head, tail = rest[: m.start()].strip(), rest[m.end() :].strip()
        rtext = m.group(0)
        if head and _SEX_LABEL_RE.fullmatch(head):
            rtext = head + m.group(0)  # 性别前缀参考范围保留原文(如 男:0-15)
        ref = rtext
        if head and _NUMBER_RE.fullmatch(head):
            value = float(head)
        if tail:
            unit = tail
    else:
        # 比较(值熔断比较式):38.201<8 mg/L
        m = _CMP_RE.search(rest)
        if m:
            head, tail = rest[: m.start()].strip(), rest[m.end() :].strip()
            hv = re.search(rf"({_NUM})$", head)
            if hv:
                value = float(hv.group(1))
            ref = m.group(0)
            if tail:
                unit = tail
        elif _NUMBER_RE.fullmatch(rest.strip()):
            value = float(rest.strip())
        elif rest.strip() and _ASCII_UNIT_RE.fullmatch(rest.strip()):
            # 单位字面:ASCII 数字/字母/%/^~// 形态(g/L、fL、pg、f1、109/L、1012/L);
            # 纯中文词(审核者：、结果、赵志佳)或含中文的垃圾不成立
            unit = rest.strip()
    return value, ref, unit or unit_from_prefix


def _flag(value: float | None, ref: str | None) -> int:
    """数值 vs 参考范围(4.2):normal/high/low/unknown。

    支持 a-b(含 --、~、性别前缀 男:0-15)与 <v / ≤v / >v / ≥v;
    范围上下界写反(OCR 乱珠)或不可解析 → unknown,不猜。
    """
    if value is None or ref is None:
        return "unknown"
    r = _SEX_PREFIX_RE.sub("", ref.strip())
    m = _RANGE_RE.fullmatch(r)
    if m:
        lo, hi = float(m.group(1)), float(m.group(2))
        if lo > hi:
            return Flag.UNKNOWN
        if lo <= value <= hi:
            return Flag.NORMAL
        return Flag.HIGH if value > hi else Flag.LOW
    m = _CMP_RE.match(r)
    if m:
        op, v = m.group(1), float(m.group(2))
        if op in "<≤":
            return Flag.NORMAL if value < v else Flag.HIGH
        return Flag.NORMAL if value > v else Flag.LOW
    return Flag.UNKNOWN


def _match_name(text: str) -> tuple[DictItem | None, float | None]:
    """词典名匹配:支持序号/星号前缀剥离与「名称熔断数值」尾缀。

    返回 (词典项, 熔断出的数值);匹配不上 → (None, None)。
    精确匹配优先(如 嗜酸性粒细胞 vs 嗜酸性粒细胞比率 的并列);熔断时最长别名优先。
    """
    t = text.strip()
    t = _ARROW_RE.sub("", t).strip()
    t = _STAR_RE.sub("", t).strip()
    t = _SERIAL_RE.sub("", t).strip()
    d = load_dict()
    it = d.alias_map.get(t)
    if it is not None:
        return it, None
    for alias in sorted(d.alias_map, key=len, reverse=True):
        if t.startswith(alias):
            rest = t[len(alias) :]
            m = _NUMBER_RE.fullmatch(rest.strip())
            if m:
                return d.alias_map[alias], float(m.group(0))
    return None, None


@dataclass
class _RowPiece:
    """一行指标项的抽取产物;value/ref/unit 为结构化字段。"""

    cy: float = 0.0
    item: DictItem | None = None
    fused_value: float | None = None
    value: float | None = None
    ref: str | None = None
    unit: str | None = None
    low_score: bool = False
    raw_cells: list[_Cell] = field(default_factory=list)

    def absorb(self, cell: _Cell, v: float | None, r: str | None, u: str | None) -> None:
        self.value = self.value if self.value is not None else v
        self.ref = self.ref if self.ref is not None else r
        self.unit = self.unit if self.unit is not None else u
        self.low_score |= cell.score < LOW_SCORE_THRESHOLD
        self.raw_cells.append(cell)


def _find_date(cells: list[_Cell], date_hint: str | None) -> str | None:
    """检查单日期:按标签优先级在行文本里搜日期;解析失败 → 用 date_hint 兜底。"""
    for label in _DATE_LABELS:
        for c in cells:
            if label in c.text:
                m = _DATE_RE.search(c.text)
                if m:
                    y, mo, day = int(m.group(1)), int(m.group(2)), int(m.group(3))
                    if 1 <= mo <= 12 and 1 <= day <= 31:
                        return f"{y:04d}-{mo:02d}-{day:02d}"
    if date_hint:
        return date_hint
    for c in cells:  # 独立日期行(标签与日期被 OCR 拆开时的兜底,如 cbc_05)
        m = _DATE_LINE_RE.match(c.text.strip())
        if m:
            return m.group(1)
    return None


def _role_centers(header_bands: list[list[_Cell]]) -> dict[str, list[tuple[float, _Cell]]]:
    """表头行的各列中心(按 x 升序,第 k 个 = 第 k 栏/半栏);列缺失的半栏自然缺位。"""
    centers: dict[str, list[tuple[float, _Cell]]] = {}
    for band in header_bands:
        for c in band:
            t = c.text.strip()
            for role, kws in ROLE_HEADERS.items():
                if any(kw in t for kw in kws):
                    centers.setdefault(role, []).append((c.cx, c))
                    break
    out: dict[str, list[tuple[float, _Cell]]] = {}
    for role, lst in centers.items():
        lst.sort(key=lambda t: t[0])
        out[role] = lst
    return out


def _extract_items(cells: list[_Cell], roles) -> list[tuple[DictItem, _RowPiece]]:
    """半栏内抽取:名称(名称列/代号列兜底)→ 与同半栏的结果/单位/参考范围碎片
    按 y 距离贪心二分配对(小 |dy| 先占),处理左右半栏行错位与熔断格。
    同一视觉行的中文名 + 代号命中同一词典项时合并为一行。"""
    # 1. 逐格归(角色, 半栏)
    assigned: dict[tuple[str, int], list[_Cell]] = {}
    for c in cells:
        best_role, best_k, best_d = None, 0, _ASSIGN_TOLERANCE_PX + 1
        for role, lst in roles.items():
            for k, (cx, _) in enumerate(lst):
                d = abs(c.cx - cx)
                if d < best_d:
                    best_role, best_k, best_d = role, k, d
        if best_role is not None:
            assigned.setdefault((best_role, best_k), []).append(c)

    items: list[tuple[DictItem, _RowPiece]] = []
    halves = {k for (role, k) in assigned if role in ("name", "aux", "value", "unit", "ref")}
    for k in sorted(halves):
        # 2. 名称候选(名称列/代号列兜底;结果列的名称熔断格也纳入,如 cbc_04 血清淀粉样蛋白A17.05)
        names: list[tuple[float, DictItem, float | None, _Cell]] = []
        for role in ("name", "aux", "value"):
            for c in assigned.get((role, k), []):
                it, fv = _match_name(c.text)
                if it is not None:
                    names.append((c.cy, it, fv, c))
        names.sort(key=lambda t: t[0])
        # 3. 相邻(近距)同典名合并为一行;项目名熔断的数值优先作为本行 value
        rows: list[_RowPiece] = []
        for cy, it, fv, c in names:
            if rows and rows[-1].item is not None and rows[-1].item.name == it.name:
                rows[-1].item = it
                if rows[-1].fused_value is None:
                    rows[-1].fused_value = fv
                rows[-1].raw_cells.append(c)
                rows[-1].low_score |= c.score < LOW_SCORE_THRESHOLD
            else:
                rows.append(
                    _RowPiece(cy=cy, item=it, fused_value=fv, raw_cells=[c],
                              low_score=c.score < LOW_SCORE_THRESHOLD)
                )
        for row in rows:
            if row.fused_value is not None:
                row.value = row.fused_value  # 名称熔断数值优先于外围碎片
        # 4. 结构化碎片按 y 贪心配对(|dy| 最小先占,上限 0.6×行距;空片不参与,
        # 防止跨行/落款碎片污染)
        if len(rows) >= 2:
            pitches = sorted(b - a for a, b in zip([r.cy for r in rows], [r.cy for r in rows[1:]]))
            cap = max(_PIECE_CAP_MIN_PX, _PIECE_CAP_FACTOR * pitches[len(pitches) // 2])
        else:
            cap = _PIECE_CAP_DEFAULT_PX
        pieces: list[tuple[_Cell, _RowPiece, float]] = []
        for role in ("value", "unit", "ref"):
            for c in assigned.get((role, k), []):
                v, r, u = _decompose(c.text)
                if v is None and r is None and u is None:
                    continue
                for row in rows:
                    d = abs(c.cy - row.cy)
                    if d <= cap:
                        pieces.append((c, row, d))
        pieces.sort(key=lambda t: (t[2], t[0].cx))
        used: set[int] = set()
        for c, row, _ in pieces:
            if c in row.raw_cells or id(c) in used:
                continue
            v, r, u = _decompose(c.text)
            if (v is not None and row.value is None) or (r is not None and row.ref is None) or (
                u is not None and row.unit is None
            ):
                row.absorb(c, v, r, u)
                used.add(id(c))
        # 5. 出项
        for row in rows:
            if row.value is None and row.fused_value is not None:
                row.value = row.fused_value
            items.append((row.item, row))
    return items


def run_postprocess(ocr: OCRResult, date_hint: str | None = None) -> Report:
    """OCR 结果 → 报告。date_hint:ingest --date 兜底;检查单已解析出日期时兜底不生效。"""
    d = load_dict()
    cells = _cells(ocr)
    date = _find_date(cells, date_hint)

    header_bands = [
        b
        for b in _bands(cells)
        if sum(1 for c in b for role, kws in ROLE_HEADERS.items() if any(kw in c.text.strip() for kw in kws)) >= 3
    ]
    roles = _role_centers(header_bands)
    pairs = _extract_items(cells, roles)

    # 类别识别:标题关键词优先,词典命中项数兜底
    title_hit = any(kw in c.text for c in cells for kw in d.title_keywords)
    category = d.report_type if (title_hit or len(pairs) >= d.min_matched_items) else None

    if not title_hit and len(pairs) < d.min_matched_items:
        return Report(report_type=UNKNOWN_REPORT_TYPE, report_date=date, status=Status.FAILED, items=[])

    items: list[ReportItem] = []
    partial = False
    for it, piece in pairs:
        raw_cells = sorted(piece.raw_cells, key=lambda c: c.cx)
        raw_text = "  ".join(c.text.strip() for c in raw_cells) or (piece.ref or "")
        flag = _flag(piece.value, piece.ref)
        low_conf = piece.low_score
        items.append(
            ReportItem(
                name=it.name,
                value=piece.value,
                unit=piece.unit,
                ref_range=piece.ref,
                flag=flag,
                low_confidence=low_conf,
                raw_text=raw_text,
            )
        )
        if low_conf or piece.value is None:
            partial = True

    if not items:
        status = Status.FAILED
    elif partial or date is None:
        status = Status.PARTIAL
    else:
        status = Status.SUCCESS
    return Report(
        report_type=category or UNKNOWN_REPORT_TYPE,
        report_date=date,
        status=status,
        items=items,
    )
