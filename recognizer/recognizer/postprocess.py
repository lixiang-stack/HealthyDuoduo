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

低置信:任一组成行 score < LOW_SCORE_THRESHOLD → 该项 low_confidence 标记保留;
partial 触发(review 2026-10-12 收敛 + 低置信占比补丁):指标 value 缺失、日期缺失,
或 **low_confidence 占比 ≥ 50%**(过半指标不可靠 → 整单降级,供人工甄别)。
单位解析损耗(如 109/L、7/6.0)经单位族规范印形表(unit_families.yaml)归一:
仅当该项词典 units 已注册 canonical 才替换 unit 字段(项上下文裁决),未注册
印形保留原文;归一只改展示一致性(hdd trend sameUnit/delta),原文留痕在
raw_text,不改变置信标记(决策 #7)。
未解析日期且无兜底 → report_date=null+partial;类别失败或无产出 → failed。

样本收窄边界(review):规则与词典由已入库真实样本驱动;新样本若引入
新版式/新指标名,预期行为是「未命中的行被跳过、指标缺失 → partial/failed」,
不会崩溃。扩展点按优先级:
1. 词典数据文件 *_dict.yaml(新类别 = 注册表加文件);
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

# 词典注册表:每个报告类别一个数据文件(决策 #8:只有规范名/别名/已知单位集,
# 不内置参考值;参考范围一律从检查单抽取),按文件名序注册。
DICT_DIR = Path(__file__).parent
DICT_GLOB = "*_dict.yaml"

# 单位族规范印形表(数据文件注册表内,与类别 *_dict.yaml 平行;语义见文件头)
UNIT_FAMILIES_FILE = DICT_DIR / "unit_families.yaml"

# 决策 #7:任一组成行 score < 阈值 → 项 low_confidence(固定常量);报告级触发见
# LOW_CONF_PARTIAL_RATIO。
LOW_SCORE_THRESHOLD = 0.8

# 低置信占比 ≥ 此值(过半指标不可靠)→ 整单 partial,供人工甄别(review 新增,
# 2026-10-12);值缺失/日期缺失仍是另一 partial 触发源。
LOW_CONF_PARTIAL_RATIO = 0.5

# 类别不识别时的报告类别(review 决定用英文占位,区别于「临检」等正名)
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

# 项目名印形里的装饰:互认标记(9-HR)/(8—HR)与科室/院级/标本限定(H)/(院级)/(静脉)、
# 「测定/定量/检测」方法后缀(如「总胆红素测定(9-HR)」→「总胆红素」),以及名称熔断的尾随
# 英文缩写(如「天门冬氨酸氨基转移酶测定(9-HR)AST」→「天门冬氨酸氨基转移酶」)。
_PAREN_TAIL_RE = re.compile(r"[（(][^（()）]*[)）]\s*$")
_METHOD_SUFFIX_RE = re.compile(r"(定量)?(测定|检测)$")
_ABBR_TAIL_RE = re.compile(r"[A-Za-z][A-Za-z0-9/_\-.]*$")
_CJK_RE = re.compile(r"[\u4e00-\u9fff]")

# 表头关键词 → 列角色(P3 扩类别后新增印形:中文名称(tft_03)、No项(lft_01 的
# 序号与项目名合并);测定结果(tft_01)/参考值单位(tft_03) 含既有关键词子串,不需单列)
ROLE_HEADERS: dict[str, tuple[str, ...]] = {
    "name": ("项目名称", "检验项目", "中文名称", "No项", "编号项目", "检验项", "参数"),
    "value": ("结果", "测定值"),
    "unit": ("单位",),
    "ref": ("参考值", "参考区间", "参考范围"),
    "aux": ("序号", "代码", "代号", "缩写", "简称"),
    "method": ("检测方法", "实验方法", "方法"),
}


# 定性结果格(值列):去箭头、去括号组后为短的纯词(含中文/字母,阴性(-)/阳性(+)/
# 核均质型/B型/RhD(+)/淡黄/Normal 之类);无数字/单位/范围/比较符,非列角色关键词。
_ROLE_KW_ALL_RE = re.compile("|".join(kw for kws in ROLE_HEADERS.values() for kw in kws))
_QUAL_VALUE_RE = re.compile(r"^[\u4e00-\u9fffA-Za-z0-9＋+（()）\-—～~、/ ]{1,20}$")


def _qualitative_value(text: str) -> str | None:
    """定性结果格 → value 字符串(印刷原样,仅去箭头);非定性格 → None。

    判定:去箭头与括号组后为短纯词(不含数字/单位/范围/比较符),且不是任何列
    角色关键词(表头词如「结果」「单位」不得进 value)。实测:ana_01/_8 定性
    阳性/阴性、_13 尿常规 黄色/清亮/Normal、_3 血型 B型/RhD(+) 等。
    """
    t = _ARROW_RE.sub("", text.strip()).strip()
    if not t or _ROLE_KW_ALL_RE.fullmatch(t):
        return None
    core = re.sub(r"[（(][^（()）]*[)）]", "", t)
    core = core.strip()
    if not core or _HINT_RE.search(core):
        return None
    if core == t and _UNIT_WORD_RE.fullmatch(core):
        return None  # 无括号组的短纯字母 = 单位形(fL),非定性词;如 RhD(+) 剥组后不此排除
    if not _QUAL_VALUE_RE.fullmatch(t):
        return None
    return t


_METHOD_TEXT_RE = re.compile(r"^([^（()）]{1,30}([（(][^（()）]*[)）])?)$")


def _method_value(text: str) -> str | None:
    """检测方法/实验方法列的数据格 → method 字符串(印刷原样,仅去方向箭头)。

    守卫:去箭头后非空、≤30 字(含尾注括号如 (贝克曼/罗氏)),非列角色关键词、
    非日期形;注释保留:方法名仅面向 method 列格,勿另作他用。
    """
    t = _ARROW_RE.sub("", text.strip()).strip()
    if not t or _ROLE_KW_ALL_RE.fullmatch(t) or _DATE_RE.search(t):
        return None
    return t if _METHOD_TEXT_RE.fullmatch(t) else None


# 行带容差系数 × 该带当前最大行高;0.7 经 5 张已入库样本实测校准:
# 过小(≤0.55)会拆散左右半栏垂直错位 ≤ 行高的同一视觉行,过大(≥0.8)会把紧凑排版的相邻行并带。
_BAND_TOLERANCE_FACTOR = 0.7

# 碎片配对 dy 上限 = 该半栏名称行距中位数 × 系数;防止跨行碎片污染(咬合到错误行)。
_PIECE_CAP_FACTOR = 0.6
_PIECE_CAP_MIN_PX = 6.0
_PIECE_CAP_DEFAULT_PX = 25.0

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
    narrative(影像叙述体:别名 + 后随数值的行内配对,见 _extract_narrative)。"""

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


def _role_keyword_count(band: list[_Cell]) -> int:
    """带内 (格, 列角色关键词) 命中数;一处阈值判定表头带的共用口径。"""
    return sum(
        1
        for c in band
        for kws in ROLE_HEADERS.values()
        if any(kw in c.text.strip() for kw in kws)
    )


def _header_bands(cells: list[_Cell]) -> list[list[_Cell]]:
    """表头带:只聚「含列角色关键词」的格。

    纯数据/标题格不参与聚带,故同一表头行内竖直错位的格(如「序号 项目名称」
    比其它列标签低半行,OCR box 偏高)仍能按相邻间距聚成一带;相邻小表间距远大于
    带内相邻关键词格间距,不会跨表合并。角色命中数 ≥ _HEADER_MIN_ROLES 才成立。
    """
    kw_cells = [c for c in cells if _role_keyword_count([c])]
    kw_cells.sort(key=lambda c: (c.cy, c.cx))
    bands: list[list[_Cell]] = []
    for c in kw_cells:
        if bands and c.cy - bands[-1][-1].cy <= _BAND_TOLERANCE_FACTOR * max(x.h for x in bands[-1]):
            bands[-1].append(c)
        else:
            bands.append([c])
    for b in bands:
        b.sort(key=lambda c: c.cx)
    return [b for b in bands if _role_keyword_count(b) >= _HEADER_MIN_ROLES]


def _decompose(text: str, d: LabDict) -> tuple[str | None, str | None, str | None]:
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
    # 方向箭头(↑↓▲▼)是 flag 语义,数值配对当分隔符:替换为空格而非删除,
    # 防止值与范围数字粘连(实测 webwxgetmsgimg_19 '18.30↓20.00~50.00' →
    # '18.30 20.00~50.00',RANGE 分支可正常拆出 value+ref)。
    s = _ARROW_RE.sub(" ", s).strip()
    # 范围分隔符两侧的空白坍缩('2.00 - 7.00' → '2.00-7.00'),不影响单位词(单位不含波浪/横杠分隔形)
    s = re.sub(r"\s*([~～\-–—])\s*", r"\1", s)
    value: str | None = None
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
            value = head
        if tail:
            unit = tail
    else:
        # 数字熔尾段单位:值+单位无空白一格('39.3fL'、'3.78D');尾段须是词典登记的
        # 单位字面(如 fL/IU)才算单位,否则尾段丢弃只保 value(乱珠如 'D')。
        # '109/L'/'1012/L' 首段数字熔 '/' 不属尾字面,落到单位分支保留原文单位。
        m2 = re.fullmatch(rf"({_NUM})([A-Za-z][A-Za-z0-9/^.%]*)", rest)
        if m2 and m2.group(2).lower() in {l.lower() for l in d.unit_literals}:
            value, unit = m2.group(1), m2.group(2)
        elif m2:
            value = m2.group(1)
        else:
            # 比较(值熔断比较式):38.201<8 mg/L
            m = _CMP_RE.search(rest)
            if m:
                head, tail = rest[: m.start()].strip(), rest[m.end() :].strip()
                hv = _NUMBER_TAIL_RE.search(head)
                if hv:
                    value = hv.group(1)
                ref = m.group(0)
                if tail:
                    unit = tail
            elif _NUMBER_RE.fullmatch(rest.strip()):
                value = rest.strip()
            elif rest.strip() and _ASCII_UNIT_RE.fullmatch(rest.strip()):
                # 单位字面:ASCII 数字/字母/%/^~// 形态(g/L、fL、pg、f1、109/L、1012/L);
                # 纯中文词(审核者：、结果、赵志佳)或含中文的垃圾不成立
                unit = rest.strip()
            else:
                # 白分隔熔断格:值+箭头+单位一格(webwxgetmsgimg_12 '2.19 ↑ IU/L'、
                # webwxgetmsgimg_20 '37.1 %'、'85.3 fL')。首段数值 → value;
                # 中段若为范围 → ref;尾段 ASCII 单位形 → unit。order: NUM [RANGE] [UNIT]
                toks = rest.split()
                if len(toks) >= 2 and _NUMBER_RE.fullmatch(toks[0]):
                    value = toks[0]
                    for tok in toks[1:]:
                        if ref is None and _RANGE_RE.fullmatch(tok):
                            ref = tok
                        elif _ASCII_UNIT_RE.fullmatch(tok) and unit is None and not _NUMBER_RE.fullmatch(tok):
                            unit = tok
    return value, ref, unit or unit_from_prefix


def _num(s: str | None) -> float | None:
    """字符串 → 数值(flag 比较用);value 契约是印刷原样字符串,含比较式/范围串不可比→ None(不猜)。"""
    if s is None:
        return None
    try:
        return float(s)
    except (TypeError, ValueError):
        return None


def _flag(value: str | None, ref: str | None) -> Flag:
    """value(印刷原样字符串) vs 参考范围(4.2):normal/high/low/unknown。

    支持 a-b(含 --、~、性别前缀 男:0-15)与 <v / ≤v / >v / ≥v;
    value 不可数值化(如 '<2.00' 检出限比较式、'70-80' 范围型结果)或参考范围
    上下界写反(OCR 乱珠)或不可解析 → unknown,不猜。
    """
    vnum = _num(value)
    if vnum is None or ref is None:
        return Flag.UNKNOWN
    r = _SEX_PREFIX_RE.sub("", ref.strip())
    m = _RANGE_RE.fullmatch(r)
    if m:
        lo, hi = float(m.group(1)), float(m.group(2))
        if lo > hi:
            return Flag.UNKNOWN
        if lo <= vnum <= hi:
            return Flag.NORMAL
        return Flag.HIGH if vnum > hi else Flag.LOW
    m = _CMP_RE.match(r)
    if m:
        op, v = m.group(1), float(m.group(2))
        if op in "<≤":
            return Flag.NORMAL if vnum < v else Flag.HIGH
        return Flag.NORMAL if vnum > v else Flag.LOW
    return Flag.UNKNOWN


@lru_cache(maxsize=1)
def load_unit_families() -> dict[str, tuple[str, ...]]:
    """单位族规范印形表:单位字面(小写)→ 候选规范印形列表;载入一次缓存。

    数据文件 unit_families.yaml(证据规则见文件头);与 *_dict.yaml 类别注册表
    相互独立——单位家族是跨类别的字形级事实(印形/损耗),类别词典只负责
    「该项合法单位集」的注册与裁决。
    """
    raw = yaml.safe_load(UNIT_FAMILIES_FILE.read_text(encoding="utf-8"))
    out: dict[str, list[str]] = {}
    for fam in raw["families"]:
        canon = str(fam["canonical"])
        for form in fam["forms"]:
            out.setdefault(str(form).strip().lower(), []).append(canon)
    return {k: tuple(v) for k, v in out.items()}


def _normalize_unit(unit: str | None, item: DictItem | None) -> str | None:
    """印刷单位 → 家族规范印形(unit_families.yaml;展示一致性归一)。

    仅当该项词典 units 已注册 canonical 才替换(如 f1 对 fL 注册项 → fL、
    对 10^9 家族注册项不生效);未注册印形或无词典项 → 原文保留,不猜。
    原文留痕在 raw_text;报告字段重合成 golden 后由人工核对。
    """
    if unit is None:
        return None
    cands = load_unit_families().get(unit.strip().lower())
    if cands and item is not None:
        for canon in cands:
            if canon.lower() in item.units:
                return canon
    return unit


def _undecorate(text: str) -> str:
    """剥离项目名印形装饰:反复去掉尾部括号组与「测定/定量/检测」方法后缀。

    如「总胆红素测定(9-HR)」→「总胆红素」、「总胆红素(8一HR)(TBIL)」→「总胆红素」、
    「血红蛋白(8-HR)」→「血红蛋白」。装饰只影响印形,不改规范名匹配语义。
    """
    prev = None
    while text and text != prev:
        prev = text
        text = _PAREN_TAIL_RE.sub("", text).strip()
        text = _METHOD_SUFFIX_RE.sub("", text).strip()
        if _CJK_RE.search(text):  # 名称熔断的尾随英文缩写(纯缩写如 eGFR 不剥)
            text = _ABBR_TAIL_RE.sub("", text).strip()
    return text


def _name_forms(text: str) -> list[str]:
    """名称的候选规范化形态:原样 / 去序号 / 去星号 / 去序号+星号(两种顺序),
    再叠加各形态去装饰(_undecorate)后的结果。

    单据印形里序号与星号可任意组合(如 `1 *丙氨酸氨基转移酶` 需去序号+星号,
    而 `3*Y谷氨酰转肽酶` 的别名本身含 `*`,只该去序号),故全部形态都试;
    互认标记/方法后缀(如 `血红蛋白(8-HR)`、`总胆红素测定`)同样只试不替换。
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
    for c in cands:
        u = _undecorate(c)
        if u and u not in forms:
            forms.append(u)
    return forms


def _match_name(text: str, d: LabDict) -> tuple[DictItem | None, str | None]:
    """词典名匹配:支持序号/星号前缀剥离(多形态)与「名称熔断数值」尾缀。

    返回 (词典项, 熔断出的数值字符串);匹配不上 → (None, None)。
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
                    return d.alias_map[alias], m.group(0)
    return None, None


@dataclass
class _RowPiece:
    """一行指标项的抽取产物;value/ref/unit 为结构化字段。"""

    cy: float = 0.0
    item: DictItem | None = None
    fused_value: str | None = None
    value: str | None = None
    ref: str | None = None
    unit: str | None = None
    method: str | None = None
    low_score: bool = False
    raw_cells: list[_Cell] = field(default_factory=list)

    def absorb(self, cell: _Cell, v: str | None, r: str | None, u: str | None, m: str | None = None) -> None:
        self.value = self.value if self.value is not None else v
        self.ref = self.ref if self.ref is not None else r
        self.unit = self.unit if self.unit is not None else u
        self.method = self.method if self.method is not None else m
        self.low_score |= cell.score < LOW_SCORE_THRESHOLD
        self.raw_cells.append(cell)


def _date_match(text: str) -> str | None:
    """日期文本 → YYYY-MM-DD;月/日形态非法(>12/>31)丢弃,不猜。"""
    m = _DATE_RE.search(text)
    if m:
        y, mo, day = int(m.group(1)), int(m.group(2)), int(m.group(3))
        if 1 <= mo <= 12 and 1 <= day <= 31:
            return f"{y:04d}-{mo:02d}-{day:02d}"
    return None


def _find_date(cells: list[_Cell], date_hint: str | None) -> str | None:
    """检查单日期:按标签优先级在行文本里搜日期;解析失败 → 用 date_hint 兜底。"""
    for label in _DATE_LABELS:
        for c in cells:
            if label in c.text:
                d = _date_match(c.text)
                if d:
                    return d
    # 标签与日期分格(webwxgetmsgimg_23):标签行(如「报告时间：」)右侧同带最近
    # 的含日期格兜底。标签与值竖直错位 ≤10px,水平近距扫描 300px。
    for label in _DATE_LABELS:
        for c in cells:
            if label not in c.text:
                continue
            cands = [
                d2
                for d2 in cells
                if d2 is not c and d2.cx > c.cx and abs(d2.cy - c.cy) <= 10.0 and (d2.cx - c.cx) <= 300.0
            ]
            if not cands:
                continue
            d = _date_match(min(cands, key=lambda d2: d2.cx).text)
            if d:
                return d
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
    (如 glu_01 逐项独立小表)互不串位。列中心取全部表头带的全局合集(P2 语义);
    每段按表头列的角色序列周期切块(左右并排同构小表,如双栏版式),
    块内名称/代号/碎片按 y 配对,不跨块;单块版式即整表配对(段内缺某角色的列时
    借全局中心补齐,如 lft_05 第二张表无名称列)。"""
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
    ny: dict[int, float] = {}  # 格 → 列基准校准后的 y(见 col_shift)

    def col_shift(role: str, idx: int) -> float:
        """列竖直基准:该列相对「名称」列表头的偏移。同一视觉行各列 OCR box 高度
        不一致会造成系统偏移(如某院区版式名称列整体低半行),用表头行作基准把
        各列 y 对齐到名称列。仅在「单表头带 + 单名称列」的版式启用(多块并排的名称列
        已按块对齐,偏移基准不可靠);否则为 0,退化为原行为。"""
        names_col = all_roles.get("name")
        role_col = all_roles.get(role)
        if len(header_bands) != 1 or not names_col or len(names_col) != 1 or not role_col or idx >= len(role_col):
            return 0.0
        return role_col[idx][1].cy - names_col[0][1].cy

    def band_columns(band: list[_Cell]) -> list[tuple[float, str, int, bool, int]]:
        """本带表头列 (cx, 角色, 角色内序号, 分组 id)。

        分组:表头列的角色序列若以最小周期重复(左右并排的同构小表,如
        「项目名称/检验项目/结果/…」×2 的双栏版式),按周期分块;否则退回按
        「角色内序号」分组(与左右半栏语义一致;单名称列时一律 0,整表配对)。缺某
        角色的带借全局中心补齐(如 lft_05 第二张表无名称列)。"""
        cols: list[tuple[float, str, bool]] = []
        for c in band:
            t = c.text.strip()
            for role, kws in ROLE_HEADERS.items():
                if any(kw in t for kw in kws):
                    # 复合列标记:role==value 的表头文本里还含参考范围类关键词
                    # (如「结果判定参考范围」「单位 结果提示参考范围」)→ 该列本身
                    # 承载 value+ref,value 角色格的 r 也可能就是真参考区间。
                    ref_kw = role == "value" and any(kw in t for kw in ROLE_HEADERS["ref"])
                    cols.append((c.cx, role, ref_kw))
                    break
        cols.sort()
        roles = [r for _, r, _ in cols]
        n = len(roles)
        period = next((p for p in range(1, n) if n % p == 0 and all(roles[i] == roles[i + p] for i in range(n - p))), 0)
        multi_half = roles.count("name") >= 2
        out: list[tuple[float, str, int, bool, int]] = []
        seen: dict[str, int] = {}
        for i, (cx, role, ref_kw) in enumerate(cols):
            idx = seen.get(role, 0)
            seen[role] = idx + 1
            out.append((cx, role, idx, ref_kw, i // period if period else (idx if multi_half else 0)))
        for role, lst in all_roles.items():
            if role not in seen:
                out.extend((cx, role, 0, False, 0) for cx, _ in lst)
        return out

    items: list[tuple[DictItem, _RowPiece]] = []
    for si, band in enumerate(header_bands):
        seg = seg_cells[si]
        if not seg:
            continue
        bcols = band_columns(band)

        # 1. 逐格归 (角色, 块)
        # 列归属 = 最近表头列中心(中点分区,不设固定偏离上限)。名称列左对齐,长文本格的
        # 中心随文本长度向右漂移(实测免疫单「抗Ribosomal P抗体」中心偏离名称列头 398px),
        # 固定容差会把这类格整格丢弃、整行漏抽(故选整行/整单缩水)。页面级垃圾格(标题/
        # 落款)照常归到最近列:与词典不命中且与数据行 dy 超限,不产出伪指标项(样本语料
        # golden 与全量真实数据回归锁定)。
        assigned: dict[tuple[str, int], list[_Cell]] = {}
        cell_ref_kw: set[int] = set()  # 复合 value 表头列(文本含参考范围)的格 id → 其 r 可进 ref 位
        for c in seg:
            best_role, best_idx, best_blk, best_d = None, 0, 0, float("inf")
            best_ref_kw = False
            for cx, role, idx, ref_kw, blk in bcols:
                dx = abs(c.cx - cx)
                if dx < best_d:
                    best_role, best_idx, best_blk, best_d, best_ref_kw = role, idx, blk, dx, ref_kw
            if best_role is not None:
                assigned.setdefault((best_role, best_blk), []).append(c)
                if best_ref_kw and best_role == "value":
                    cell_ref_kw.add(id(c))
                ny[id(c)] = c.cy - col_shift(best_role, best_idx)

        blocks = sorted({blk for (role, blk) in assigned if role in ("name", "aux", "value", "unit", "ref")})
        for blk in blocks:
            # 2. 名称候选(名称列/代号列兜底;结果列的名称熔断格也纳入,如 cbc_04 血清淀粉样蛋白A17.05)
            names: list[tuple[float, DictItem, str | None, _Cell]] = []
            for role in ("name", "aux", "value"):
                for c in assigned.get((role, blk), []):
                    it, fv = _match_name(c.text, d)
                    if it is not None:
                        names.append((ny.get(id(c), c.cy), it, fv, c))
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
            # 4. 碎片配对:先「结果」列做**互最近直配**——名称格常印在数据行上方,若结果格
            #    与某行互为最近且偏移 > 0.25×行距,直接落该行并用其 y 作为该行数据行 y
            #    (避免最近 y 把上一行的参考配到下一行,如 lft_05 顶部);其余碎片按 |dy|
            #    贪心配对(上限 0.6×行距)。
            if len(rows) >= 2:
                pitches = sorted(b - a for a, b in zip([r.cy for r in rows], [r.cy for r in rows[1:]]))
                pitch = pitches[len(pitches) // 2]
                cap = max(_PIECE_CAP_MIN_PX, _PIECE_CAP_FACTOR * pitch)
            else:
                pitch = _PIECE_CAP_DEFAULT_PX
                cap = _PIECE_CAP_DEFAULT_PX
            used: set[int] = set()
            value_cells = [c for c in assigned.get(("value", blk), []) if any(_decompose(c.text, d))]
            if value_cells and rows:
                row_of = {id(c): min(rows, key=lambda r: abs(ny.get(id(c), c.cy) - r.cy)) for c in value_cells}
                cell_of = {id(r): min(value_cells, key=lambda c: abs(ny.get(id(c), c.cy) - r.cy)) for r in rows}
                for c in value_cells:
                    row = row_of[id(c)]
                    if cell_of.get(id(row)) is c and (ny.get(id(c), c.cy) - row.cy) > 0.25 * pitch:
                        v, _, _ = _decompose(c.text, d)
                        if v is not None and row.value is None:
                            row.absorb(c, v, None, None)
                            row.cy = ny.get(id(c), c.cy)
                            used.add(id(c))
            pieces: list[tuple[_Cell, str, _RowPiece, float, str | None]] = []
            for role in ("value", "method", "unit", "ref"):
                for c in assigned.get((role, blk), []):
                    if id(c) in used:
                        continue
                    v, r, u = _decompose(c.text, d)
                    qual = None
                    if v is None and r is None and u is None:
                        if role == "value":
                            qual = _qualitative_value(c.text)
                        elif role == "method":
                            qual = _method_value(c.text)
                        if qual is None:
                            continue
                    for row in rows:
                        dy = abs(ny.get(id(c), c.cy) - row.cy)
                        if dy <= cap:
                            pieces.append((c, role, row, dy, qual))
            # 吸收顺序:value 角色碎片优先(value 列才是 value 槽的权属;其他角色碎片
            # 熔出的 v 不能抢在 value 列之前,如 glu_01 糖化 '6.50'(value 列)必须
            # 先于乱珠单位格 '1 %' 熔出的 '1'落地),同角色再按 (dy,cx) 贪心。
            pieces.sort(key=lambda t: (t[1] != "value", t[3], t[0].cx))
            for c, role, row, _, qual in pieces:
                if c in row.raw_cells or id(c) in used:
                    continue
                if qual:
                    # 定性结果格(阳性(+)/阴性(一)/核均质型/B型/RhD(+)/淡黄/Normal…)
                    # 印刷原样进 value 位(string 契约);flag 维持 unknown。
                    # 检测方法格 → method 位(检测方法/实验方法列)。
                    if role == "value" and row.value is None:
                        row.absorb(c, qual, None, None)
                        used.add(id(c))
                    elif role == "method" and row.method is None:
                        row.absorb(c, None, None, None, qual)
                        used.add(id(c))
                    continue
                v, r, u = _decompose(c.text, d)
                # 值列的纯比较式(<2.00,结果低于检出限)不是参考区间:拆不出数值时其 r
                # 不进 ref 位,避免挤掉参考列真格(如 webwxgetmsgimg_1 抗dsDNA抗体
                # "<2.00"值列 挤掉 "0-30"参考列)。复合表头列(「结果判定参考范围」)
                # 的值格其 r 本是完全区间,仍照旧吸收;熔断式(38.201<8,带首部数值)不受限。
                ref_ok = r is not None and not (role == "value" and v is None and id(c) not in cell_ref_kw)
                if (v is not None and row.value is None) or (ref_ok and row.ref is None) or (
                    u is not None and row.unit is None
                ):
                    row.absorb(c, v, r if ref_ok else None, u)
                    used.add(id(c))
                elif role == "value" and v is None and r is not None and row.value is None:
                    # 值列的纯比较式/范围式(<2.00、1-2)= 检出限或范围型结果,value 契约是
                    # 印刷原样字符串,可直接承载;不占 ref 位,参考列真格正常配对。
                    row.absorb(c, r, None, None)
                    used.add(id(c))
            # 5. 出项
            for row in rows:
                if row.value is None and row.fused_value is not None:
                    row.value = row.fused_value
                if row.value is None and row.ref is None and row.unit is None and row.fused_value is None:
                    # 零碎片行:竖排回退——名称格正下方同列最近数值格(产筛/糖耐量等
                    # 竖列头版式,webwxgetmsgimg_17/_18);找到则吸收,否则该行仅名称在案。
                    vcell, vval = _vertical_value(cells, row.raw_cells[0], d)
                    if vval is not None:
                        row.absorb(vcell, vval, None, None)
                items.append((row.item, row))
    # 同 canonical 名去重:一图一标准名只留一行——同名重复命中多为页脚/直方图图例
    # 的再命中(webwxgetmsgimg_19:真 血小板 145.00 行 + 图例 'PLT' 幽灵行)。
    # 保留碎片(吸收格)多/有 value 的行;并列时注册表序(先者)。
    dedup: dict[str, tuple[DictItem, _RowPiece]] = {}
    for it, row in items:
        prev = dedup.get(it.name)
        if prev is not None:
            # 保留吸收格多(value 视同两格权重)者;并列留先者。
            score_new = len(row.raw_cells) + (5 if row.value is not None else 0)
            score_old = len(prev[1].raw_cells) + (5 if prev[1].value is not None else 0)
            if score_new > score_old:
                dedup[it.name] = (it, row)
        else:
            dedup[it.name] = (it, row)
    return list(dedup.values())


def _extract_narrative(cells: list[_Cell], d: LabDict) -> list[tuple[DictItem, _RowPiece]]:
    """影像叙述体抽取(P3;category.mode=narrative):无表头带,别名 + 桥接符 +
    后随数值的行内配对(如 「（NT）2.6mm」「胎心率159次/分」)。

    值紧密性:别名后仅允许桥接符(冒号/括号/约/'值'/空白/句点等)再出现数字,
    「NT筛查(省免)、11-14周」之类远距数字不误认。同一 canonical 多次命中时
    取首个有数值者(标题提及在前、测值在后的版式);raw_text 为整行。

    竖排回退(产筛特有版式,webwxgetmsgimg_17):别名格(value 为 None 时)在**正下方
    同列**(x 重叠 ≥ 名格宽 50%)找最近的数值格,其数值 token 作为 value;该版式
    项目名在列头、风险值/校正mom值在列内向下排布,行内配对天然不适用。
    """
    bridge_re = re.compile(r"^[：:()（）约≈。，、.\s值]*")
    best: dict[str, tuple[DictItem, _RowPiece, str | None]] = {}
    for c in cells:
        t = _ARROW_RE.sub("", c.text.strip()).strip()
        if not t:
            continue
        for alias in sorted(d.alias_map, key=len, reverse=True):
            idx = t.find(alias)
            if idx < 0:
                continue
            it = d.alias_map[alias]
            # 两阶段:阶段一只做行内配对(值紧邻别名);阶段二(见下循环)对仍无 value 的
            # 命中做竖排回退。若行内配对阶段就做竖排,「标题提及在前、测值在后」的版式
            # (us_01:NT 标题格 + 后段 (NT)2.6mm)会把标题格先于真值格竖排抢值。
            head = bridge_re.match(t[idx + len(alias):])
            value: str | None = None
            unit: str | None = None
            if head is not None:
                rest = t[idx + len(alias) + head.end():]
                m = re.match(rf"({_NUM})(.*)", rest)
                if m:
                    value = m.group(1)
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
    # 阶段二:行内无数值的命中 → 竖排回退(正下方同列最近数值格)
    for name in [k for k, v3 in best.items() if v3[2] is None]:
        piece_cell = best[name][1].raw_cells[0]
        vcell, value = _vertical_value(cells, piece_cell, d)
        if value is not None:
            it, piece, _ = best[name]
            best[name] = (it,
                          _RowPiece(cy=piece.cy, item=it, value=value, unit=None,
                                    raw_cells=[piece_cell, vcell],
                                    low_score=piece.low_score),
                          value)
    return [(it, piece) for it, piece, _ in (best[k] for k in sorted(best))]


def _vertical_value(
    cells: list[_Cell], name_cell: _Cell, d: LabDict
) -> tuple[_Cell | None, str | None]:
    """竖排回退(narrative/零碎片表行):名称格**正下方同列**的最近数值格。

    产筛竖排表与糖耐量转置表(列头=项目名/缩写,值在列内向下,webwxgetmsgimg_17/_18):
    候选门:①cx 落在名格 x 区间内(列归属);②未其他词典命中(下一行「序号+名」格不算);
    ③非日期形;④非纯范围串;含数值 token(比较前缀可带) → 数值字符串;按 (dy,|dcx|) 取最近。
    找不到 → (None, None),保持 value=None 的诚实留痕。
    """
    best: tuple[float, float, _Cell, str] | None = None
    for c in cells:
        if c is name_cell or c.y0 < name_cell.y1:
            continue
        if not (name_cell.x0 <= c.cx <= name_cell.x1):
            continue
        if _DATE_RE.search(c.text) or _match_name(c.text.strip(), d)[0] is not None:
            continue
        if _RANGE_RE.fullmatch(c.text.strip()):
            continue
        m = re.search(r"(<|≤|>|≥)?\s*(\d+(?:\.\d+)?(?::\d+)?)", c.text)
        if m is None:
            continue
        val = (m.group(1) or "") + m.group(2)
        key = (c.y0 - name_cell.y1, abs(c.cx - name_cell.cx))
        if best is None or key < best[:2]:
            best = (*key, c, val)
    if best is None:
        return None, None
    return best[2], best[3]


def run_postprocess(ocr: OCRResult, date_hint: str | None = None) -> Report:
    """OCR 结果 → 报告。date_hint:ingest --date 兜底;检查单已解析出日期时兜底不生效。"""
    cells = _cells(ocr)
    date = _find_date(cells, date_hint)

    header_bands = _header_bands(cells)

    selected = _select_category(cells, header_bands)
    if selected is None:
        return Report(report_type=UNKNOWN_REPORT_TYPE, report_date=date, status=Status.FAILED, items=[])
    d, pairs = selected

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
                unit=_normalize_unit(piece.unit, piece.item),
                ref_range=piece.ref,
                method=piece.method,
                flag=flag,
                low_confidence=low_conf,
                raw_text=raw_text,
            )
        )
        if piece.value is None:
            partial = True

    if not items:
        status = Status.FAILED
    else:
        low_conf_majority = (
            sum(1 for it in items if it.low_confidence) / len(items) >= LOW_CONF_PARTIAL_RATIO
        )
        if partial or date is None or low_conf_majority:
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
) -> tuple[LabDict, list[tuple[DictItem, _RowPiece]]] | None:
    """多词典注册表分发(P3 A1 内容优先分类)。

    判定由内容驱动:命中项数 ≥ min_matched_items 的词典「达标」;达标者中命中项数
    多者胜出,并列时标题(标题区内)命中者胜出,再并列取注册表先者(文件名序)。
    仅当没有任何词典达标时,才回退到标题命中的词典——标题不再是「硬覆盖」,正文
    偶发提及无法把报告抢到不相干类别。

    标题区分层(见 _title_region_cells)用于达标者并列裁决;兜底层仍允许全页匹配,
    因为报告未达标时往往正是「标题词被印成项目名」的样本(如 glu_03 的糖化血红蛋白)。
    """
    title_cells = _title_region_cells(cells, header_bands)
    best: tuple[LabDict, list[tuple[DictItem, _RowPiece]]] | None = None
    best_key: tuple[bool, int, bool] | None = None
    for d in load_dicts():
        if d.mode == "narrative":
            pairs = _extract_narrative(cells, d)
        else:
            pairs = _extract_items(cells, header_bands, d)
        qualified = len(pairs) >= d.min_matched_items
        if not qualified and not any(kw in c.text for c in cells for kw in d.title_keywords):
            continue
        title_region = any(kw in c.text for c in title_cells for kw in d.title_keywords)
        key = (qualified, len(pairs), title_region)
        if best_key is not None and key <= best_key:
            continue
        best, best_key = (d, pairs), key
    return best
