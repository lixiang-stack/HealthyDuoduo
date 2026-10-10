"""规则化后处理 v1(实施计划 P2-1/2):OCR 结果 → 结构化报告。

纯函数、无 IO:输入 contract.OCRResult(+可选日期兜底),输出 contract.Report。
词典是数据文件注册表 P3 起多类别并存(决策 #8:只有规范名/别名/已知单位集,
不内置参考值;参考范围一律从检查单抽取):DICT_DIR 下每个 *_dict.yaml 对应
一个报告类别,分发为内容优先(P3 A1;标题仅兜底/并列裁决,见 _select_category)。

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

样本收窄边界(review):规则与词典由已入库真实样本驱动;新样本若引入
新版式/新指标名,预期行为是「未命中的行被跳过、指标缺失 → partial/failed」,
不会崩溃。扩展点按优先级:
1. 词典数据文件 *_dict.yaml(新类别 = 注册表加文件);
2. 改动后重跑 `python -m recognizer.golden` 并人工核对 raw_text;
3. 新版式(表头关键词/列距/熔断形态不匹配)才动本文件解析规则,
   且须先以真实样本落 samples/expected/ocr(golden)后再改。
"""

import logging
import re
from dataclasses import dataclass, field
from functools import lru_cache
from pathlib import Path

import yaml

from .contract import Flag, OCRResult, Report, ReportItem, Status

logger = logging.getLogger(__name__)

# 词典注册表:每个报告类别一个数据文件(决策 #8:只有规范名/别名/已知单位集,
# 不内置参考值;参考范围一律从检查单抽取),按文件名序注册。
DICT_DIR = Path(__file__).parent
DICT_GLOB = "*_dict.yaml"

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

# 表结构网格(ADR-0004):TSR 产出的 HTML 表 → 单元格文本矩阵
_GRID_TR_RE = re.compile(r"<tr>(.*?)</tr>", re.DOTALL)
_GRID_TD_RE = re.compile(r"<td[^>]*>(.*?)</td>", re.DOTALL)
_GRID_TAG_RE = re.compile(r"<[^>]+>")

# 表头关键词 → 列角色(P3 扩类别后新增印形:中文名称(tft_03)、No项(lft_01 的
# 序号与项目名合并);测定结果(tft_01)/参考值单位(tft_03) 含既有关键词子串,不需单列)
ROLE_HEADERS: dict[str, tuple[str, ...]] = {
    "name": ("项目名称", "检验项目", "中文名称", "No项", "编号项目", "检验项"),
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

# 表头行判定:一行命中的 (格, 列角色关键词) 对 ≥ 此值才判为表头带;
# 3 = 名称+结果+单位/参考值等列角色同时在行内的最少组合。
_HEADER_MIN_ROLES = 3

# 分类标题区(P3 A1 内容优先):类别标题预期出现的页面位置——首条表头带上方;
# 无表头带(叙述体)时取页面高度上方该比例。标题只在此区匹配,避免正文提及误分发。
_TITLE_REGION_FRACTION = 0.25


@dataclass(frozen=True)
class DictItem:
    name: str
    aliases: tuple[str, ...]
    units: frozenset[str]


@dataclass
class LabDict:
    """一个报告类别的词典(数据文件 *_dict.yaml 的加载产物);单位字面量与
    别名索引在加载时一次派生。mode=table(默认,表头带/行带版式)或
    narrative(超声叙述体:别名 + 后随数值的行内配对,见 _extract_narrative)。"""

    report_type: str
    title_keywords: tuple[str, ...]
    min_matched_items: int
    items: tuple[DictItem, ...]
    alias_map: dict[str, DictItem] = field(default_factory=dict)
    unit_literals: tuple[str, ...] = ()
    mode: str = "table"


@lru_cache(maxsize=1)
def load_dicts() -> tuple[LabDict, ...]:
    """加载词典注册表(DICT_DIR 下全部 *_dict.yaml,文件名序);改词典文件后重启进程即生效。"""
    return tuple(_build_dict(p) for p in sorted(DICT_DIR.glob(DICT_GLOB)))


def _build_dict(path: Path) -> LabDict:
    raw = yaml.safe_load(path.read_text(encoding="utf-8"))
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
    # 全部词典单位的字面量(含 * 前缀变体),longest-first 供前缀剥离
    units: set[str] = set()
    for it in items:
        units |= it.units
    literals: list[str] = []
    for u in sorted(units):
        literals.append(u)
        if not u.startswith("*"):
            literals.append("*" + u)
    return LabDict(
        report_type=raw["report_type"],
        title_keywords=tuple(raw["category"]["title_keywords"]),
        min_matched_items=int(raw["category"]["min_matched_items"]),
        items=items,
        alias_map=alias_map,
        unit_literals=tuple(sorted(set(literals), key=len, reverse=True)),
        mode=str(raw["category"].get("mode", "table")),
    )


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


def _decompose(text: str, d: LabDict) -> tuple[float | None, str | None, str | None]:
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
    for lit in d.unit_literals:
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


def _flag(value: float | None, ref: str | None) -> Flag:
    """数值 vs 参考范围(4.2):normal/high/low/unknown。

    支持 a-b(含 --、~、性别前缀 男:0-15)与 <v / ≤v / >v / ≥v;
    范围上下界写反(OCR 乱珠)或不可解析 → unknown,不猜。
    """
    if value is None or ref is None:
        return Flag.UNKNOWN
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


def _name_forms(text: str) -> list[str]:
    """名称的候选规范化形态:原样 / 去序号 / 去星号 / 去序号+星号(两种顺序)。

    单据印形里序号与星号可任意组合(如 `1 *丙氨酸氨基转移酶` 需去序号+星号,
    而 `3*Y谷氨酰转肽酶` 的别名本身含 `*`,只该去序号),故全部形态都试。
    """
    t = _ARROW_RE.sub("", text.strip()).strip()
    cands = (
        t,
        _SERIAL_RE.sub("", t).strip(),
        _STAR_RE.sub("", t).strip(),
        _STAR_RE.sub("", _SERIAL_RE.sub("", t).strip()).strip(),
        _SERIAL_RE.sub("", _STAR_RE.sub("", t).strip()).strip(),
    )
    forms: list[str] = []
    for c in cands:
        if c and c not in forms:
            forms.append(c)
    return forms


def _match_name(text: str, d: LabDict) -> tuple[DictItem | None, float | None]:
    """词典名匹配:支持序号/星号前缀剥离(多形态)与「名称熔断数值」尾缀。

    返回 (词典项, 熔断出的数值);匹配不上 → (None, None)。
    精确匹配优先(如 嗜酸性粒细胞 vs 嗜酸性粒细胞比率 的并列);熔断时最长别名优先。
    """
    forms = _name_forms(text)
    for form in forms:
        it = d.alias_map.get(form)
        if it is not None:
            return it, None
    for form in forms:
        for alias in sorted(d.alias_map, key=len, reverse=True):
            if form.startswith(alias):
                rest = form[len(alias) :]
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
    raw_text: str | None = None

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


def _extract_items(
    cells: list[_Cell],
    header_bands: list[list[_Cell]],
    d: LabDict,
) -> list[tuple[DictItem, _RowPiece]]:
    """表内抽取:名称(名称列/代号列兜底)→ 与同表的结果/单位/参考范围碎片
    按 y 距离贪心二分配对(小 |dy| 先占),处理左右半栏行错位与熔断格。
    同一视觉行的中文名 + 代号命中同一词典项时合并为一行。

    分段(P3 多类别版式):数据格归属其上方最近的一条表头带——垂直堆叠的小表
    (如 glu_01 逐项独立小表)互不串位。角色中心取全部表头带的全局合集(P2 语义);
    每段按「本段表头带的名称中心数」分流:≥2 → 左右半栏(按最近中心归 (role, 半栏 k),
    半栏内配对,不串栏);≤1 → 整表:半栏 k 恒 0,名称/代号/碎片全池配对(本段表头带
    缺名称列时,沿用全局中心里其它表头带的名称列,如 lft_05 的第二张表)。"""
    if not header_bands:
        return []
    seg_cells: dict[int, list[_Cell]] = {i: [] for i in range(len(header_bands))}
    band_tops = [min(c.y0 for c in b) for b in header_bands]
    for c in cells:
        seg = 0
        for i, top in enumerate(band_tops):
            if c.y0 >= top:
                seg = i
        seg_cells[seg].append(c)

    all_roles = _role_centers(header_bands)
    items: list[tuple[DictItem, _RowPiece]] = []
    for si, band in enumerate(header_bands):
        seg = seg_cells[si]
        if not seg:
            continue
        multi_half = len(_role_centers([band]).get("name", [])) >= 2

        # 1. 逐格归(角色, 半栏)。整表模式一律 k=0(聚合全部角色中心)。
        assigned: dict[tuple[str, int], list[_Cell]] = {}
        for c in seg:
            best_role, best_k, best_d = None, 0, _ASSIGN_TOLERANCE_PX + 1
            for role, lst in all_roles.items():
                for k, (cx, _) in enumerate(lst):
                    dx = abs(c.cx - cx)
                    if dx < best_d:
                        best_role, best_k, best_d = role, k, dx
            if best_role is not None:
                if not multi_half:
                    best_k = 0  # 整表模式:本段表头带无/仅单名称中心,无左右半栏错位可言
                assigned.setdefault((best_role, best_k), []).append(c)

        halves = {k for (role, k) in assigned if role in ("name", "aux", "value", "unit", "ref")}
        for k in sorted(halves):
            # 2. 名称候选(名称列/代号列兜底;结果列的名称熔断格也纳入,如 cbc_04 血清淀粉样蛋白A17.05)
            names: list[tuple[float, DictItem, float | None, _Cell]] = []
            for role in ("name", "aux", "value"):
                for c in assigned.get((role, k), []):
                    it, fv = _match_name(c.text, d)
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
                    v, r, u = _decompose(c.text, d)
                    if v is None and r is None and u is None:
                        continue
                    for row in rows:
                        dy = abs(c.cy - row.cy)
                        if dy <= cap:
                            pieces.append((c, row, dy))
            pieces.sort(key=lambda t: (t[2], t[0].cx))
            used: set[int] = set()
            for c, row, _ in pieces:
                if c in row.raw_cells or id(c) in used:
                    continue
                v, r, u = _decompose(c.text, d)
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


def _extract_narrative(cells: list[_Cell], d: LabDict) -> list[tuple[DictItem, _RowPiece]]:
    """超声叙述体抽取(P3;category.mode=narrative):无表头带,别名 + 桥接符 +
    后随数值的行内配对(如 「（NT）2.6mm」「胎心率159次/分」)。

    值紧密性:别名后仅允许桥接符(冒号/括号/约/'值'/空白/句点等)再出现数字,
    「NT筛查(省免)、11-14周」之类远距数字不误认。同一 canonical 多次命中时
    取首个有数值者(标题提及在前、测值在后的版式);raw_text 为整行。
    """
    bridge_re = re.compile(r"^[：:()（）约≈。，、.\s值]*")
    best: dict[str, tuple[DictItem, _RowPiece, float | None]] = {}
    for c in cells:
        t = _ARROW_RE.sub("", c.text.strip()).strip()
        if not t:
            continue
        for alias in sorted(d.alias_map, key=len, reverse=True):
            idx = t.find(alias)
            if idx < 0:
                continue
            it = d.alias_map[alias]
            head = bridge_re.match(t[idx + len(alias):])
            value: float | None = None
            unit: str | None = None
            if head is not None:
                rest = t[idx + len(alias) + head.end():]
                m = re.match(rf"({_NUM})(.*)", rest)
                if m:
                    value = float(m.group(1))
                    tail = m.group(2).strip()
                    for lit in sorted(it.units, key=len, reverse=True):
                        if tail.lower().startswith(lit.lower()):
                            unit = tail[: len(lit)].strip()
                            break
            row = best.get(it.name)
            if row is not None and not (row[2] is None and value is not None):
                continue
            best[it.name] = (
                it,
                _RowPiece(cy=c.cy, item=it, value=value, unit=unit, raw_cells=[c],
                          low_score=c.score < LOW_SCORE_THRESHOLD),
                value,
            )
    return [(it, piece) for it, piece, _ in (best[k] for k in sorted(best))]


def parse_grid(html: str) -> list[list[str]]:
    """TSR 的 HTML 表 → 单元格文本矩阵(去标签、去空白);不展开 rowspan/colspan。

    本流程只取文本网格用于按列配对;合并格在 hybrid 择优输出里不构成主要噪声,
    简化处理以免引入跨模型差异(原型 clean 指标即用同名解析口径)。
    """
    return [
        [_GRID_TAG_RE.sub("", td).strip() for td in _GRID_TD_RE.findall(tr)]
        for tr in _GRID_TR_RE.findall(html)
    ]


def _grid_header(rows: list[list[str]]) -> tuple[int, dict[int, str]] | None:
    """定位表头行与 列序号→角色 映射:取命中表头关键词最多的行;
    需同时含「名称/代号」列与「结果」列,否则视为无表头(回退启发式)。"""
    best_idx, best_roles, best_n = -1, {}, 0
    for i, row in enumerate(rows):
        roles: dict[int, str] = {}
        for ci, cell in enumerate(row):
            t = cell.strip()
            for role, kws in ROLE_HEADERS.items():
                if any(kw in t for kw in kws):
                    roles[ci] = role
                    break
        if len(roles) > best_n:
            best_idx, best_roles, best_n = i, roles, len(roles)
    has_name = any(r in ("name", "aux") for r in best_roles.values())
    has_value = any(r == "value" for r in best_roles.values())
    if not (has_name and has_value):
        return None
    return best_idx, best_roles


def _scores_by_text(ocr: OCRResult) -> dict[str, float]:
    """OCR 行文本(忽略空白)→ 最低 score:供网格路径复原 low_confidence(决策 #7)。"""
    out: dict[str, float] = {}
    for t, s in zip(ocr.txts, ocr.scores):
        key = "".join(t.split())
        if key:
            out[key] = min(out.get(key, 1.0), s)
    return out


def _grid_low_score(row: list[str], scores: dict[str, float]) -> bool:
    """该网格行的任一格文本命中 OCR 低分行 → 低置信(网格本身不带 score)。"""
    for c in row:
        key = "".join(c.split())
        if key and scores.get(key, 1.0) < LOW_SCORE_THRESHOLD:
            return True
    return False


def _extract_items_from_grid(
    rows: list[list[str]],
    d: LabDict,
    scores: dict[str, float],
) -> list[tuple[DictItem, _RowPiece]]:
    """网格抽取:表头行定位列角色 → 名称列逐行匹配 → 同段(至下一名称列)的
    结果/单位/参考格 _decompose 配对。左右双栏(名称列 ≥2)按列分段,互不串位。

    名称可印在名称列或代号列;同一行同名(名称+代号双命中)合并为一项;
    名称熔断进结果格的（kind=名称列无果时整行兜底）用熔断值优先。
    """
    header = _grid_header(rows)
    if header is None:
        return []
    header_idx, roles = header
    name_cols = sorted(ci for ci, role in roles.items() if role in ("name", "aux"))
    out: list[tuple[DictItem, _RowPiece]] = []
    for row in rows[header_idx + 1:]:
        if not any(c.strip() for c in row):
            continue
        matches: list[tuple[int, DictItem, float | None]] = []
        for ci in name_cols:
            if ci < len(row):
                it, fv = _match_name(row[ci], d)
                if it is not None:
                    matches.append((ci, it, fv))
        if not matches:  # 名称列无果:整行兜底(名称熔断进结果格)
            for ci, cell in enumerate(row):
                it, fv = _match_name(cell, d)
                if it is not None:
                    matches.append((ci, it, fv))
        if not matches:
            continue
        merged: dict[str, _RowPiece] = {}
        for ci, it, fv in matches:
            piece = merged.get(it.name) or _RowPiece(item=it, cy=float(header_idx))
            if piece.value is None and fv is not None:
                piece.value = fv
            nxt = next((nc for nc in name_cols if nc > ci), len(row))
            for cc in range(ci + 1, min(nxt, len(row))):
                if roles.get(cc) not in (None, "value", "unit", "ref"):
                    continue
                v, r, u = _decompose(row[cc], d)
                piece.value = piece.value if piece.value is not None else v
                piece.ref = piece.ref if piece.ref is not None else r
                piece.unit = piece.unit if piece.unit is not None else u
            merged[it.name] = piece
        raw = "  ".join(c.strip() for c in row if c.strip())
        low = _grid_low_score(row, scores)
        for piece in merged.values():
            piece.raw_text = raw
            piece.low_score = low
            out.append((piece.item, piece))
    return out


def _extract_pairs(
    d: LabDict,
    cells: list[_Cell],
    header_bands: list[list[_Cell]],
    grid: list[list[str]] | None,
    scores: dict[str, float],
) -> tuple[list[tuple[DictItem, _RowPiece]], str, int, int]:
    """按词典类别抽取:叙述体走行内配对;表格类取「网格路径」与「现状启发式」
    的命中项数多者(ADR-0004:网格还原更稳,但不劣化——网格退化/无表头时自动退回)。
    返回 (命中项, 路径来源, 网格项数, 启发式项数):来源 ∈ narrative/grid/heuristic,
    两个项数供 run_postprocess 记日志(判断启发式能否下线)。"""
    if d.mode == "narrative":
        return _extract_narrative(cells, d), "narrative", 0, 0
    heuristic = _extract_items(cells, header_bands, d)
    if grid is not None:
        from_grid = _extract_items_from_grid(grid, d, scores)
        if len(from_grid) > len(heuristic):
            return from_grid, "grid", len(from_grid), len(heuristic)
        return heuristic, "heuristic", len(from_grid), len(heuristic)
    return heuristic, "heuristic", 0, len(heuristic)


def run_postprocess(ocr: OCRResult, date_hint: str | None = None) -> Report:
    """OCR 结果 → 报告。date_hint:ingest --date 兜底;检查单已解析出日期时兜底不生效。"""
    cells = _cells(ocr)
    date = _find_date(cells, date_hint)

    header_bands = [
        b
        for b in _bands(cells)
        if sum(1 for c in b for role, kws in ROLE_HEADERS.items() if any(kw in c.text.strip() for kw in kws)) >= _HEADER_MIN_ROLES
    ]

    grid: list[list[str]] | None = None
    scores: dict[str, float] = {}
    if ocr.table_structure is not None:
        grid = parse_grid(ocr.table_structure.html)
        scores = _scores_by_text(ocr)

    selected = _select_category(cells, header_bands, grid, scores)
    if selected is None:
        return Report(report_type=UNKNOWN_REPORT_TYPE, report_date=date, status=Status.FAILED, items=[])
    d, pairs, source, n_grid, n_heur = selected
    logger.info(
        "tsr-decision report_type=%s source=%s grid_items=%d heur_items=%d",
        d.report_type, source, n_grid, n_heur,
    )

    items: list[ReportItem] = []
    partial = False
    for it, piece in pairs:
        if piece.raw_text is not None:
            raw_text = piece.raw_text
        else:
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
        report_type=d.report_type,
        report_date=date,
        status=status,
        items=items,
    )


def _title_region_cells(
    cells: list[_Cell],
    header_bands: list[list[_Cell]],
) -> list[_Cell]:
    """标题区:报告类别词预期出现的位置。

    有表头带时取首条表头带顶线之上(标题/抬头行);无表头带(叙述体)时取页面上方
    _TITLE_REGION_FRACTION 高度。分类标题只在标题区内匹配,正文偶发提及不算数。
    """
    if not cells:
        return []
    if header_bands:
        cutoff = min(c.y0 for band in header_bands for c in band)
    else:
        y_lo = min(c.y0 for c in cells)
        y_hi = max(c.y1 for c in cells)
        cutoff = y_lo + _TITLE_REGION_FRACTION * (y_hi - y_lo)
    return [c for c in cells if c.cy <= cutoff]


def _select_category(
    cells: list[_Cell],
    header_bands: list[list[_Cell]],
    grid: list[list[str]] | None = None,
    scores: dict[str, float] | None = None,
) -> tuple[LabDict, list[tuple[DictItem, _RowPiece]], str, int, int] | None:
    """多词典注册表分发(P3 A1 内容优先分类)。

    判定由内容驱动:命中项数 ≥ min_matched_items 的词典「达标」;达标者中命中项数
    多者胜出,并列时标题(标题区内)命中者胜出,再并列取注册表先者(文件名序)。
    仅当没有任何词典达标时,才回退到标题命中的词典——标题不再是「硬覆盖」,正文
    偶发提及无法把报告抢到不相干类别。

    标题区分层(见 _title_region_cells)用于达标者并列裁决;兜底层仍允许全页匹配,
    因为报告未达标时往往正是「标题词被印成项目名」的样本(如 glu_03 的糖化血红蛋白)。

    grid(ADR-0004):表格类报告若带表结构网格,抽取走网格路径(退化/无果自动回退)。
    返回 (词典, 命中项, 路径来源, 网格项数, 启发式项数) 或 None。
    """
    scores = scores or {}
    title_cells = _title_region_cells(cells, header_bands)
    best: tuple[LabDict, list[tuple[DictItem, _RowPiece]], str, int, int] | None = None
    best_key: tuple[bool, int, bool] | None = None
    for d in load_dicts():
        pairs, source, n_grid, n_heur = _extract_pairs(d, cells, header_bands, grid, scores)
        qualified = len(pairs) >= d.min_matched_items
        if not qualified and not any(kw in c.text for c in cells for kw in d.title_keywords):
            continue
        title_region = any(kw in c.text for c in title_cells for kw in d.title_keywords)
        key = (qualified, len(pairs), title_region)
        if best_key is not None and key <= best_key:
            continue
        best, best_key = (d, pairs, source, n_grid, n_heur), key
    return best
