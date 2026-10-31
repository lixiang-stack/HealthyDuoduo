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
报告级段落(叙述型报告如影像单,词典 category.section_anchors):按行首锚词切
检查项目/检查所见/提示,整段原文 + OCR 行号留痕,不解析内容(见 _extract_sections)。

低置信:任一组成行 score < LOW_SCORE_THRESHOLD → 该项 low_confidence 标记保留;
partial 触发(review 2026-10-12 收敛 + 低置信占比补丁):指标 value 缺失、日期缺失,
或 **low_confidence 占比 ≥ 50%**(过半指标不可靠 → 整单降级,供人工甄别)。
单位解析损耗(如 109/L、7/6.0)经单位族规范印形表(unit_families.yaml)归一:
仅当该项词典 units 已注册 canonical 才替换 unit 字段(项上下文裁决),未注册
印形保留原文;归一只改展示一致性(hdd trend sameUnit/delta),原文留痕在
raw_text,不改变置信标记(决策 #7)。
未解析日期且无兜底 → report_date=null+partial;类别失败或无产出 → failed。
叙述型报告(影像;min_section_hits>0)只按正文段落派生 status:findings/impression
至少一段 → success,否则 failed;不产出指标项,也不进入 partial(票 04)。

样本收窄边界(review):规则与词典由已入库真实样本驱动;新样本若引入
新版式/新指标名,预期行为是「未命中的行被跳过、指标缺失 → partial/failed」,
不会崩溃。扩展点按优先级:
1. 词典数据文件 *_dict.yaml(新类别 = 注册表加文件);
2. 改动后重跑 `python -m recognizer.golden` 并人工核对 raw_text;
3. 新版式(表头关键词/列距/熔断形态不匹配)才动本文件解析规则,
   且须先以真实样本落 samples/expected/ocr(golden)后再改。
"""

import re
import unicodedata
from dataclasses import dataclass, field
from functools import lru_cache
from pathlib import Path

import yaml

from .contract import (
    Flag,
    FlagReason,
    OCRResult,
    Panel,
    Report,
    ReportItem,
    ReportSection,
    Status,
    allowed_panels,
    report_panels,
)
from .dash_recovery import recover_dash_fields

# 词典注册表:每个报告类别一个数据文件(决策 #8:只有规范名/别名/已知单位集,
# 不内置参考值;参考范围一律从检查单抽取),按文件名序注册。
DICT_DIR = Path(__file__).parent
DICT_GLOB = "*_dict.yaml"

# 单位族规范印形表(数据文件注册表内,与类别 *_dict.yaml 平行;语义见文件头)
UNIT_FAMILIES_FILE = DICT_DIR / "unit_families.yaml"

# 定性比较策略(与 Go 数据服务共读;只含比较规则,不含 OCR 别名/单位)
FLAG_POLICY_FILE = DICT_DIR / "flag_policy.yaml"

# 决策 #7:任一组成行 score < 阈值 → 项 low_confidence(固定常量);报告级触发见
# LOW_CONF_PARTIAL_RATIO。
LOW_SCORE_THRESHOLD = 0.8

# 低置信占比 ≥ 此值(过半指标不可靠)→ 整单 partial,供人工甄别(review 新增,
# 2026-10-12);值缺失/日期缺失仍是另一 partial 触发源。
LOW_CONF_PARTIAL_RATIO = 0.5

# 类别不识别时的报告类别(review 决定用英文占位,区别于「临检」等正名)
UNKNOWN_REPORT_TYPE = "unknown"

# 报告级段落字段(识别端契约 ReportSection 与叙述型词典 category.section_anchors 的键)。
# 正文锚 = 报告叙述主体;exam_name 只用于段落抽取,不参与类别判定(票 04)。
BODY_SECTION_KINDS = ("findings", "impression")
SECTION_FIELDS = ("exam_name", *BODY_SECTION_KINDS)

# 日期标签,按优先级降序取首个命中行里的日期(打印/申请最低;检验最高)
_DATE_LABELS = (
    "检验时间", "检验日期", "检测时间", "检测日期", "检查时间", "检查日期",
    "采样时间", "采集时间", "标本采集时间",
    "核收时间", "报告时间", "报告日期", "打印时间", "申请时间",
)
_DATE_RE = re.compile(r"(\d{4})[-/.年](\d{1,2})[-/.月]?(\d{1,2})")
# 独立日期行(行首锚定):分隔符与 _DATE_RE 同口径(破折号/斜杠/点/年月日);
# 月日合法性由 _date_match 复核。
_DATE_LINE_RE = re.compile(r"^(\d{4})[-/.年](\d{1,2})[-/.月]?(\d{1,2})\b")

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
    # 纯符号定性结果(- / + / ± / 全角形态)先于数值提示判定,供 value/ref 两列使用。
    if re.fullmatch(r"[-+±＋－—−]+", t):
        return t
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

# 竖排回退候选「数值 token 居格首」的前缀容差:容许空白/箭头等 ≤2 个前缀字符
# (页脚散文格的数字在格中段,超此即拒)。
_VERTICAL_VALUE_PREFIX_MAX = 2

# 相邻 OCR 行框常有小幅上下重叠(us_02 侧脑室名称行与 9.1mm 值行重叠 4px/17px);
# 竖排回退允许最多 1/4 格高的重叠,既能取到正下方值行,又不把同一行邻格当下一行。
_VERTICAL_ROW_OVERLAP_RATIO = 0.25

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
    panel: Panel
    # 标准层:名称/代码/单位/结果类型/来源;缺省时保持旧词典行为。
    code: str | None = None
    loinc: str | None = None
    local_code: str | None = None
    source_status: str = "pending"
    review_status: str = "pending"
    result_kind: str = "quantitative"
    canonical_unit: str | None = None
    unit_display: str | None = None
    accepted_units: tuple[dict[str, object], ...] = ()
    ocr_unit_forms: tuple[dict[str, object], ...] = ()
    synonyms: tuple[str, ...] = ()
    ocr_aliases: tuple[str, ...] = ()
    result_basis: str | None = None


@dataclass
class LabDict:
    """一个报告类别的词典(数据文件 *_dict.yaml 的加载产物);单位字面量与
    别名索引在加载时一次派生。mode=table(默认,表头带/行带版式)或
    narrative(叙述体:别名 + 后随数值的行内配对,见 _extract_narrative)。"""

    report_type: str
    title_keywords: tuple[str, ...]
    min_matched_items: int
    items: tuple[DictItem, ...]
    alias_map: dict[str, DictItem] = field(default_factory=dict)
    unit_literals: tuple[str, ...] = ()
    mode: str = "table"
    # 报告级段落锚(字段 → 行首锚词);非叙述体类别为空。
    section_anchors: dict[str, tuple[str, ...]] = field(default_factory=dict)
    # 正文锚命中阈值(叙述型类别≥1;0 = 不按段落分类)。
    min_section_hits: int = 0


@lru_cache(maxsize=1)
def load_dicts() -> tuple[LabDict, ...]:
    """加载词典注册表(DICT_DIR 下全部 *_dict.yaml,文件名序);改词典文件后重启进程即生效。"""
    return tuple(_build_dict(p) for p in sorted(DICT_DIR.glob(DICT_GLOB)))


def _aliases_of(it: dict) -> tuple[str, ...]:
    """别名 = 旧 aliases + 标准 synonyms + OCR 变体;去重保序,末尾补规范名。"""
    aliases = list(it.get("aliases", []))
    aliases += [s["term"] for s in it.get("synonyms") or []]
    aliases += [a["term"] for a in it.get("ocr_aliases") or []]
    return tuple(dict.fromkeys(aliases)) + (it["name"],)


def _canonical_units_of(it: dict) -> list[str]:
    """词典项的单位注册集:新 schema 用 canonical + display,旧 schema 用 units。"""
    if it.get("canonical_unit") or it.get("unit_display"):
        return [u for u in (it.get("canonical_unit"), it.get("unit_display")) if u]
    return list(it.get("units", []))


def _literals_of(it: DictItem) -> set[str]:
    """一项可参与匹配的单位字面量:canonical/display + 真实替代 + OCR 损耗形。"""
    out = {u for u in it.units if u and u != "1"}
    for u in (it.canonical_unit, it.unit_display):
        if u and u != "1":
            out.add(u)
    for au in it.accepted_units:
        for key in ("unit", "ucum"):
            if au.get(key):
                out.add(str(au[key]))
    for form in it.ocr_unit_forms:
        if form.get("form"):
            out.add(str(form["form"]))
    return out


def _build_dict(path: Path) -> LabDict:
    raw = yaml.safe_load(path.read_text(encoding="utf-8"))
    category = raw["category"]
    # 解析规则(item.panel ?? category.panel ?? other):混合词典逐项覆盖,
    # 单一项目组词典只写类别默认(spec §5.5.1)。
    default_panel = Panel(category.get("panel", Panel.OTHER))
    items = tuple(
        DictItem(
            name=it["name"],
            aliases=_aliases_of(it),
            units=frozenset(u.lower() for u in _canonical_units_of(it)),
            panel=Panel(it.get("panel", default_panel)),
            code=it.get("code"),
            loinc=it.get("loinc"),
            local_code=it.get("local_code"),
            source_status=str(it.get("source_status", "pending")),
            review_status=str(it.get("review_status", "pending")),
            result_kind=str(it.get("result_kind", "quantitative")),
            canonical_unit=it.get("canonical_unit"),
            unit_display=it.get("unit_display"),
            accepted_units=tuple(it.get("accepted_units") or ()),
            ocr_unit_forms=tuple(it.get("ocr_unit_forms") or ()),
            synonyms=tuple(s["term"] for s in it.get("synonyms") or ()),
            ocr_aliases=tuple(a["term"] for a in it.get("ocr_aliases") or ()),
            result_basis=it.get("result_basis"),
        )
        for it in raw.get("items") or []
    )
    alias_map: dict[str, DictItem] = {}
    for it in items:
        for a in it.aliases:
            alias_map.setdefault(a, it)
    # 全部词典单位的字面量(含 * 前缀变体),longest-first 供前缀剥离
    units: set[str] = set()
    for it in items:
        units |= _literals_of(it)
    literals: list[str] = []
    for u in sorted(units):
        literals.append(u)
        if not u.startswith("*"):
            literals.append("*" + u)
    raw_anchors = raw["category"].get("section_anchors") or {}
    section_anchors = {
        field: tuple(words)
        for field, words in raw_anchors.items()
        if field in SECTION_FIELDS and words
    }
    return LabDict(
        report_type=raw["report_type"],
        title_keywords=tuple(raw["category"]["title_keywords"]),
        min_matched_items=int(raw["category"].get("min_matched_items", 0)),
        items=items,
        alias_map=alias_map,
        unit_literals=tuple(sorted(set(literals), key=len, reverse=True)),
        mode=str(raw["category"].get("mode", "table")),
        section_anchors=section_anchors,
        min_section_hits=int(raw["category"].get("min_section_hits", 0)),
    )


@dataclass
class _Cell:
    text: str
    score: float
    x0: float
    y0: float
    x1: float
    y1: float
    line: int  # OCR 行下标(txts/boxes 同下标;_cells 填充,供 item→行链接)

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
    """行文本 + box 多边形 → 以包络矩形表达的格(带行下标)。"""
    out = []
    for i, (text, box, score) in enumerate(zip(ocr.txts, ocr.boxes, ocr.scores)):
        xs = [p[0] for p in box]
        ys = [p[1] for p in box]
        out.append(_Cell(text=text, score=score, x0=min(xs), y0=min(ys), x1=max(xs), y1=max(ys), line=i))
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
    # 纯符号定性结果(- / + / ±)不是单位;留给 _qualitative_value 进 value/ref 槽。
    if re.fullmatch(r"[-+±＋－—−]+", s):
        return None, None, None
    # 纯中文非数字文本(如「结果」「审核者：」)不产生任何 fragment,防止伪单位;
    # 短 ASCII 字母串(如 fL、pg)是合法单位形
    if not s or (
        not _HINT_RE.search(s)
        and not _UNIT_WORD_RE.fullmatch(s)
        and _unit_match(s, d) is None
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
    # 词典单位字面前缀(含 * 前缀形态);单位按命中的原文切片保留大小写。
    # 前缀命中后剩余片段若只是标点(如 "mmol/L." 的 "."),仍以前缀为准。
    rest = s
    unit_from_prefix = _unit_match(rest, d)
    if unit_from_prefix is not None:
        rest = rest[len(unit_from_prefix) :].strip()
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
            unit = _unit_or_none(tail, d)
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
                    unit = _unit_or_none(tail, d)
            elif _NUMBER_RE.fullmatch(rest.strip()):
                value = rest.strip()
            elif m := re.fullmatch(rf"({_NUM})%", rest.strip()):
                # 数值百分号熔一格('22.8%'、'31%'):value + unit,不当作纯单位。
                value, unit = m.group(1), "%"
            elif rest.strip() and _ASCII_UNIT_RE.fullmatch(rest.strip()):
                # 单位字面:ASCII 数字/字母/%/^~// 形态(g/L、fL、pg、f1、109/L、1012/L);
                # 纯中文词(审核者：、结果、赵志佳)或含中文的垃圾不成立
                unit = _unit_or_none(rest, d)
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
                        elif unit is None and _ASCII_UNIT_RE.fullmatch(tok) and not _NUMBER_RE.fullmatch(tok):
                            unit = _unit_or_none(tok, d)
    return value, ref, unit_from_prefix or unit


_TIER_LABEL_RE = re.compile(r"^[\u4e00-\u9fff]{1,4}$")


def _parse_tier_ref(text: str, d: LabDict) -> tuple[str, str | None, str] | None:
    """参考区间列的分级解读行(如 "<12ng/ml缺乏" / ">20ng/ml正常")→ (范围, 单位, 标签)。

    只认「范围 + 可选单位 + 1~4 个纯中文标签」的行;孕周前缀(1-2周:65-650)、
    普通区间 + 单位(≤21.0μmol/L)不当作分级。
    """
    s = text.strip()
    m = re.match(
        rf"^([<>≤≥]?\s*{_NUM}(?:\s*[-–—~]{{1,2}}\s*{_NUM})?)\s*(.*)$",
        s,
    )
    if m is None:
        return None
    range_text = re.sub(r"\s+", "", m.group(1))
    rest = m.group(2).strip()
    unit = _unit_match(rest, d)
    if unit is not None:
        rest = rest[len(unit) :].strip()
    if not rest or _TIER_LABEL_RE.fullmatch(rest) is None:
        return None
    return range_text, unit, rest


def _tier_ref_text(
    tier: tuple[str, str | None, str],
    item: DictItem | None,
) -> str:
    range_text, unit, label = tier
    return f"{range_text}{_normalize_unit(unit, item) or ''}{label}"


def _same_column(a: "_Cell", b: "_Cell") -> bool:
    return min(a.x1, b.x1) - max(a.x0, b.x0) > 0


def _attach_tier_refs(
    rows: list["_RowPiece"],
    ref_cells: list["_Cell"],
    d: LabDict,
) -> None:
    """把参考列下方的分级解读行挂回该指标行,用「正常」档范围做比较基准。

    真实版式(vitanemia_01/02):首行印 "<12ng/ml缺乏",下方空行依次印
    "12-20ng/ml不足" / ">20ng/ml正常"。只取首行会把「<12」当整个参考区间,
    把 5.70 判 normal、39.60 判 high;改为以正常档 ">20ng/ml正常" 为参考后,
    5.70→low、39.60→normal。无正常档时分档文本前置「分级:」,比较层自然回到
    unknown(不猜);原文行仍全部留在 raw_text。
    """
    if not ref_cells:
        return
    ref_ids = {id(c) for c in ref_cells}
    used = {id(c) for row in rows for c in row.raw_cells}
    for index, row in enumerate(rows):
        ref_cell = next((c for c in row.raw_cells if id(c) in ref_ids), None)
        if ref_cell is None:
            continue
        first = _parse_tier_ref(ref_cell.text, d)
        if first is None:
            continue
        next_cy = rows[index + 1].cy if index + 1 < len(rows) else float("inf")
        row_height = max(ref_cell.y1 - ref_cell.y0, 1.0)
        max_cy = min(next_cy, ref_cell.cy + 6 * row_height)
        tiers = [first]
        extra: list[_Cell] = []
        for cell in ref_cells:
            if id(cell) in used or cell is ref_cell:
                continue
            if not (ref_cell.cy < cell.cy < max_cy):
                continue
            if not _same_column(cell, ref_cell):
                continue
            tier = _parse_tier_ref(cell.text, d)
            if tier is None:
                continue
            tiers.append(tier)
            extra.append(cell)
        normal = next(
            (tier for tier in tiers if "正常" in tier[2] or tier[2].lower() == "normal"),
            None,
        )
        row.ref = (
            _tier_ref_text(normal, row.item)
            if normal is not None
            else "分级：" + "；".join(_tier_ref_text(tier, row.item) for tier in tiers)
        )
        row.raw_cells.extend(extra)
        used.update(id(cell) for cell in extra)


@dataclass(frozen=True)
class _FlagProfile:
    normal: frozenset[str] = frozenset()
    abnormal: frozenset[str] = frozenset()
    informational: frozenset[str] = frozenset()
    normal_values: frozenset[str] = frozenset()
    abnormal_values: frozenset[str] = frozenset()
    informational_all: bool = False


@dataclass(frozen=True)
class _FlagPolicy:
    categories: dict[str, str]
    profiles: dict[str, _FlagProfile]
    items: dict[str, str]
    no_reference_items: frozenset[str]
    no_reference_methods: tuple[str, ...]


def _policy_text(text: str) -> str:
    s = unicodedata.normalize("NFKC", text).strip()
    s = re.sub(r"\s+", "", s)
    return s.replace("–", "-").replace("—", "-").replace("−", "-")


@lru_cache(maxsize=1)
def load_flag_policy() -> _FlagPolicy:
    """定性比较策略(flag_policy.yaml);与 Go 数据服务共读同一份。"""
    raw = yaml.safe_load(FLAG_POLICY_FILE.read_text(encoding="utf-8"))
    categories: dict[str, str] = {}
    for category, aliases in raw["normalizers"].items():
        for alias in aliases:
            categories[_policy_text(str(alias))] = category
    profiles: dict[str, _FlagProfile] = {}
    for name, p in raw["profiles"].items():
        profiles[name] = _FlagProfile(
            normal=frozenset(p.get("normal", [])),
            abnormal=frozenset(p.get("abnormal", [])),
            informational=frozenset(p.get("informational", [])),
            normal_values=frozenset(_policy_text(v) for v in p.get("normal_values", [])),
            abnormal_values=frozenset(_policy_text(v) for v in p.get("abnormal_values", [])),
            informational_all=bool(p.get("informational_all", False)),
        )
    items = {str(k): str(v) for k, v in raw.get("items", {}).items()}
    no_reference = frozenset(raw.get("no_reference_items", []))
    no_reference_methods = tuple(raw.get("no_reference_method_keywords", []))
    return _FlagPolicy(categories, profiles, items, no_reference, no_reference_methods)


@dataclass(frozen=True)
class _NumForm:
    kind: str  # scalar | interval | bound
    a: float = 0.0
    b: float = 0.0
    op: str = ""


def _num_form(text: str, ref: bool = False) -> _NumForm | None:
    s = text.strip()
    if ref:
        s = _SEX_PREFIX_RE.sub("", s)
    m = _RANGE_RE.fullmatch(s)
    if m:
        return _NumForm("interval", float(m.group(1)), float(m.group(2)))
    m = _CMP_RE.match(s)
    if m and (ref or not s[m.end():].strip()):
        return _NumForm("bound", float(m.group(2)), op=m.group(1))
    if _NUMBER_RE.fullmatch(s):
        return _NumForm("scalar", float(s))
    return None


def _value_interval(form: _NumForm) -> tuple[float, float] | None:
    if form.kind == "interval":
        return (form.a, form.b) if form.a <= form.b else None
    if form.kind == "bound":
        if form.op in "<≤":
            return (0.0, form.a) if form.a >= 0 else None
        return (form.a, float("inf"))
    return (form.a, form.a)


def _compare_intervals(vlo: float, vhi: float, rlo: float, rhi: float) -> tuple[Flag, FlagReason | None]:
    if vhi < rlo:
        return Flag.LOW, None
    if vlo > rhi:
        return Flag.HIGH, None
    if vlo >= rlo and vhi <= rhi:
        return Flag.NORMAL, None
    return Flag.UNKNOWN, FlagReason.INDETERMINATE


def _compare_numeric(vform: _NumForm, rform: _NumForm) -> tuple[Flag, FlagReason | None]:
    if rform.kind == "scalar":
        # 标量 0 / 0.0 视为精确零点;其他标量语义不明,不猜。
        if rform.a != 0:
            return Flag.UNKNOWN, FlagReason.SCALAR_REF
        rform = _NumForm("interval", 0.0, 0.0)
    if rform.kind == "interval":
        if rform.a > rform.b:
            return Flag.UNKNOWN, FlagReason.OTHER
        if vform.kind == "scalar":
            v = vform.a
            if v < rform.a:
                return Flag.LOW, None
            if v > rform.b:
                return Flag.HIGH, None
            return Flag.NORMAL, None
        vints = _value_interval(vform)
        if vints is None:
            return Flag.UNKNOWN, FlagReason.OTHER
        return _compare_intervals(*vints, rform.a, rform.b)
    if vform.kind == "bound":
        v_upper = vform.op in "<≤"
        r_upper = rform.op in "<≤"
        if v_upper and r_upper:
            if vform.a < rform.a or (
                vform.a == rform.a and not (vform.op == "≤" and rform.op == "<")
            ):
                return Flag.NORMAL, None
            return Flag.UNKNOWN, FlagReason.INDETERMINATE
        if not v_upper and not r_upper:
            if vform.a > rform.a or (
                vform.a == rform.a and not (vform.op == "≥" and rform.op == ">")
            ):
                return Flag.NORMAL, None
            return Flag.UNKNOWN, FlagReason.INDETERMINATE
    vints = _value_interval(vform)
    if vints is None:
        return Flag.UNKNOWN, FlagReason.OTHER
    vlo, vhi = vints
    if rform.op in "<≤":
        if vhi < rform.a:
            return Flag.NORMAL, None
        if vlo >= rform.a:
            return Flag.HIGH, None
        return Flag.UNKNOWN, FlagReason.INDETERMINATE
    if vlo > rform.a:
        return Flag.NORMAL, None
    if vhi <= rform.a:
        return Flag.LOW, None
    return Flag.UNKNOWN, FlagReason.INDETERMINATE


def _compare_qualitative(
    name: str | None, value: str, ref: str | None, policy: _FlagPolicy
) -> tuple[Flag, FlagReason | None]:
    nv = _policy_text(value)
    nr = _policy_text(ref) if ref else None
    # 结果、参考都是裸短横 → 本次未检测(不是正常);已做项目的阴性参考仍走下方正常规则。
    if nv == "-" and nr == "-":
        return Flag.NOT_TESTED, None
    vcat = policy.categories.get(nv)
    rcat = policy.categories.get(nr) if nr else None
    # 值、参考都是阴性文本(含「阴性(-)」等,但不是裸短横)→ 正常。
    if vcat == "negative" and rcat == "negative":
        return Flag.NORMAL, None
    profile = policy.profiles.get(policy.items.get(name or "", ""))
    if profile is None and rcat == "negative":
        profile = policy.profiles.get("negative_normal")
    if profile is None and rcat == "positive":
        profile = policy.profiles.get("positive_normal")
    if profile is None:
        return Flag.UNKNOWN, FlagReason.QUALITATIVE_UNSUPPORTED
    if profile.informational_all:
        return Flag.UNKNOWN, FlagReason.INFORMATIONAL
    if nv in profile.normal_values or vcat in profile.normal:
        return Flag.NORMAL, None
    if nv in profile.abnormal_values or vcat in profile.abnormal:
        return Flag.ABNORMAL, None
    if vcat in profile.informational:
        return Flag.UNKNOWN, FlagReason.INFORMATIONAL
    return Flag.UNKNOWN, FlagReason.QUALITATIVE_UNSUPPORTED


def _compare(name: str | None, value: str | None, ref: str | None) -> tuple[Flag, FlagReason | None]:
    """value(印刷原样) × ref_range → (flag, reason)。

    - high/low:数值有方向异常;abnormal:定性无方向异常;normal:可比较且正常;
    - unknown 带 reason;reason 由抽取层进一步细化为 OCR/配对/无范围。
    """
    if value is None or not value.strip():
        return Flag.UNKNOWN, FlagReason.OTHER
    policy = load_flag_policy()
    if ref is None and name in policy.no_reference_items:
        return Flag.UNKNOWN, FlagReason.NO_REFERENCE
    vform = _num_form(value)
    rform = _num_form(ref, ref=True) if ref is not None else None
    if vform is not None and rform is not None:
        return _compare_numeric(vform, rform)
    if vform is not None:
        # 数值值 + 非数值/不可解析参考范围:不猜。
        return Flag.UNKNOWN, FlagReason.OTHER
    if rform is not None:
        return Flag.UNKNOWN, FlagReason.QUALITATIVE_UNSUPPORTED
    return _compare_qualitative(name, value, ref, policy)


def _flag(value: str | None, ref: str | None, name: str | None = None) -> Flag:
    """兼容旧调用:只返回 flag;需要 reason 用 _compare。"""
    return _compare(name, value, ref)[0]


def _extraction_reason(piece: "_RowPiece", reason: FlagReason | None, raw_text: str) -> FlagReason:
    """把比较层的通用 reason 细化为卡片可解释的抽取原因。

    只在 flag=unknown 时调用;比较层能判定的语义原因(标量/定性/信息项/
    indeterminate)原样保留,剩余缺失值/范围按 raw_text 线索分到 OCR/配对。
    """
    if reason not in (None, FlagReason.OTHER):
        return reason
    if piece.ref:
        m = _RANGE_RE.fullmatch(piece.ref)
        if m and float(m.group(1)) > float(m.group(2)) and piece.ref in raw_text:
            return FlagReason.OCR_MERGE
    if piece.value is None:
        # 值不在 value 位,但 raw_text 有数值候选 → 配对;否则 OCR 没输出。
        return FlagReason.EXTRACTOR_PAIRING if _numeric_candidate(raw_text, piece) else FlagReason.OCR_MISS
    if piece.ref is None:
        if piece.method and any(k in piece.method for k in load_flag_policy().no_reference_methods):
            return FlagReason.NO_REFERENCE
        return FlagReason.EXTRACTOR_PAIRING if _RANGE_RE.search(raw_text) else FlagReason.OCR_MISS
    return FlagReason.OTHER


def _numeric_candidate(raw_text: str, piece: "_RowPiece") -> bool:
    """raw_text 里是否有不属于 name/ref/unit 的数值候选。"""
    excluded = _policy_text(piece.item.name if piece.item else "") + _policy_text(piece.ref or "")
    unit = (piece.unit or "").strip()
    # 纯单位里的数字(10^9/L、109/L)不算值;但「22.8%」「31%」是值误落 unit,算候选。
    if not re.fullmatch(rf"{_NUM}%", unit):
        excluded += _policy_text(unit)
    # 行首序号(「10 酮体」)和方法括号(「(9-HR)」)不是值候选。
    text = _policy_text(raw_text)
    text = re.sub(r"^\d{1,2}", "", text)
    text = re.sub(r"\([^)]*\)", "", text)
    for m in re.finditer(_NUM, text):
        if m.group(0) not in excluded:
            return True
    return False


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


def _clean_unit_text(text: str) -> str:
    """单位格尾部标点(OCR 常见 "mmol/L." / "g/l." / "pg/mL.")不是单位的一部分。

    只对像单位的文本去尾点;"0." 这类乱珠保留原文,避免无依据改写。
    """
    cleaned = text.strip()
    trimmed = cleaned.rstrip(".。").strip()
    if not trimmed:
        return ""
    if any(ch.isalpha() or ch in "%^/" or "\u4e00" <= ch <= "\u9fff" for ch in trimmed):
        return trimmed
    return cleaned


def _unit_match(text: str, d: LabDict) -> str | None:
    """单位格/尾段命中的最长合法单位前缀(词典注册字面 + 该项可归一损耗形)。

    只返回原文切片,大小写与损耗形保留;规范化仍由 _normalize_unit 按项裁决。
    """
    s = text.strip()
    if not s:
        return None
    lower = s.lower()
    registered = {u.lower() for u in d.unit_literals}
    candidates = set(d.unit_literals)
    for form, canons in load_unit_families().items():
        if any(canon.lower() in registered for canon in canons):
            candidates.add(form)
    for lit in sorted(candidates, key=len, reverse=True):
        if lower.startswith(lit.lower()):
            return s[: len(lit)]
    return None


def _unit_or_none(text: str, d: LabDict) -> str | None:
    """尾段/纯单位格的单位值:先找注册印形/损耗形;否则仅接受 ASCII 单位形态。

    含中文的尾段(如 "ng/ml缺乏" 的 "缺乏")不是单位;尾随标点先清理。
    """
    matched = _unit_match(text, d)
    if matched is not None:
        return _clean_unit_text(matched) or None
    cleaned = _clean_unit_text(text)
    if cleaned and _ASCII_UNIT_RE.fullmatch(cleaned):
        return cleaned
    return None


def _normalize_unit(unit: str | None, item: DictItem | None) -> str | None:
    """印刷单位 → 家族规范印形(unit_families.yaml;展示一致性归一)。

    仅当该项词典 units 已注册 canonical 才替换(如 f1 对 fL 注册项 → fL、
    对 10^9 家族注册项不生效);未注册印形或无词典项 → 原文保留,不猜。
    原文留痕在 raw_text;报告字段重合成 golden 后由人工核对。
    """
    if unit is None:
        return None
    unit = _clean_unit_text(unit)
    cands = load_unit_families().get(unit.lower())
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
    # 标签与日期分格(webwxgetmsgimg_23):标签行(如「报告时间：」)右侧同带最近的
    # 含日期格兜底。门限按图内容宽度/格高缩放——旧值 300px/10px 按小截图校准,
    # 大图字号同比放大(实测 3316px 照片盒间距 322px);max 单调放宽不引入更严判定。
    width = max((c.x1 for c in cells), default=0.0)
    for label in _DATE_LABELS:
        for c in cells:
            if label not in c.text:
                continue
            gap_limit = max(300.0, 0.2 * width)
            dy_limit = max(10.0, 0.25 * c.h)
            cands = [
                d2
                for d2 in cells
                if d2 is not c and d2.cx > c.cx and abs(d2.cy - c.cy) <= dy_limit and (d2.cx - c.cx) <= gap_limit
            ]
            if not cands:
                continue
            d = _date_match(min(cands, key=lambda d2: d2.cx).text)
            if d:
                return d
    if date_hint:
        return date_hint
    for c in cells:  # 独立日期行(标签与日期被 OCR 拆开时的兜底,如 cbc_05)
        if _DATE_LINE_RE.match(c.text.strip()):
            d = _date_match(c.text)
            if d:
                return d
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
                        if role == "value" or role == "ref":
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
                    elif role == "ref" and row.ref is None:
                        row.absorb(c, None, qual, None)
                        used.add(id(c))
                    elif role == "method" and row.method is None:
                        row.absorb(c, None, None, None, qual)
                        used.add(id(c))
                    continue
                v, r, u = _decompose(c.text, d)
                # 参考列的单标量 0(如 NRBC 的 "0")在 _decompose 里被当成 value;
                # 按列角色改回 ref,交给 ComputeFlag 的标量 0 规则比较。非 0 标量
                # 语义不明,维持原行为(不进 ref 位,避免把 OCR 乱码当参考区间)。
                ref_value = r
                if role == "ref" and ref_value is None and v is not None and u is None and float(v) == 0:
                    ref_value = v
                    v = None
                # 值列的纯比较式(<2.00,结果低于检出限)不是参考区间:拆不出数值时其 r
                # 不进 ref 位,避免挤掉参考列真格(如 webwxgetmsgimg_1 抗dsDNA抗体
                # "<2.00"值列 挤掉 "0-30"参考列)。复合表头列(「结果判定参考范围」)
                # 的值格其 r 本是完全区间,仍照旧吸收;熔断式(38.201<8,带首部数值)不受限。
                ref_ok = ref_value is not None and not (role == "value" and v is None and id(c) not in cell_ref_kw)
                if (v is not None and row.value is None) or (ref_ok and row.ref is None) or (
                    u is not None and row.unit is None
                ):
                    row.absorb(c, v, ref_value if ref_ok else None, u)
                    used.add(id(c))
                elif role == "value" and v is None and r is not None and row.value is None:
                    # 值列的纯比较式/范围式(<2.00、1-2)= 检出限或范围型结果,value 契约是
                    # 印刷原样字符串,可直接承载;不占 ref 位,参考列真格正常配对。
                    row.absorb(c, r, None, None)
                    used.add(id(c))
            # 4.5 分级参考区间:参考列下方同行内空行的解读档挂回指标行。
            _attach_tier_refs(rows, assigned.get(("ref", blk), []), d)

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
    # 阶段三:横排表格里的项目名右侧同行值(转正后的产筛风险计算表:
    # 唐氏综合征 1:1913 / 开放性神经管缺陷 2.5MOM / 母龄风险 33.25岁)。
    for name in [k for k, v3 in best.items() if v3[2] is None]:
        piece_cell = best[name][1].raw_cells[0]
        vcell, value = _right_value(cells, piece_cell, d)
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
    ③非日期形;④非纯范围串;⑤数值 token 居格首(容许 ≤2 前缀字符;含数值的散文格如
    页脚「实验室地址:…155号」不得按数字抽值——实测会把门牌号写成 value);
    ⑥行框允许 ≤1/4 格高的上下重叠,相邻 OCR 行不因几像素重叠被跳过。
    按 (dy,|dcx|) 取最近;找不到 → (None, None),保持 value=None 的诚实留痕。
    """
    overlap = (name_cell.y1 - name_cell.y0) * _VERTICAL_ROW_OVERLAP_RATIO
    best: tuple[float, float, _Cell, str] | None = None
    for c in cells:
        if c is name_cell or c.y0 < name_cell.y1 - overlap:
            continue
        if not (name_cell.x0 <= c.cx <= name_cell.x1):
            continue
        if _DATE_RE.search(c.text) or _match_name(c.text.strip(), d)[0] is not None:
            continue
        if _RANGE_RE.fullmatch(c.text.strip()):
            continue
        m = re.search(r"(<|≤|>|≥)?\s*(\d+(?:\.\d+)?(?::\d+)?)", c.text)
        if m is None or m.start() > _VERTICAL_VALUE_PREFIX_MAX:
            continue
        val = (m.group(1) or "") + m.group(2)
        key = (c.y0 - name_cell.y1, abs(c.cx - name_cell.cx))
        if best is None or key < best[:2]:
            best = (*key, c, val)
    if best is None:
        return None, None
    return best[2], best[3]


_RIGHT_VALUE_RE = re.compile(
    r"(\d+(?:\.\d+)?(?::\d+)?)(?:MOM|岁)?", re.IGNORECASE
)


def _right_value(
    cells: list[_Cell], name_cell: _Cell, d: LabDict
) -> tuple[_Cell | None, str | None]:
    """横排表格回退:名称格**右侧同一行**最近的值格(产筛风险计算表)。

    只接受纯数值/风险比值/带 MOM 或 岁 后缀的值,排除日期、纯参考区间与词典名;
    行差不超过两格较高者。按 (x0, |dy|) 取最近(风险值列在截断值列左侧);
    找不到 → (None, None)。
    """
    best: tuple[float, float, _Cell, str] | None = None
    for c in cells:
        if c is name_cell or c.x0 < name_cell.x1:
            continue
        text = c.text.strip()
        if _DATE_RE.search(text) or _match_name(text, d)[0] is not None:
            continue
        if _RANGE_RE.fullmatch(text):
            continue
        m = _RIGHT_VALUE_RE.fullmatch(text)
        if m is None:
            continue
        dy = abs(c.cy - name_cell.cy)
        if dy > max(name_cell.h, c.h):
            continue
        key = (c.x0, dy)
        if best is None or key < best[:2]:
            best = (*key, c, m.group(1))
    if best is None:
        return None, None
    return best[2], best[3]


def _extract_sections(
    txts: list[str], anchors: dict[str, tuple[str, ...]]
) -> list[ReportSection]:
    """按行首锚词切报告级段落,整段原文 + OCR 行号留痕(不解析内容)。

    规则(影像单三段式):
    - 锚 = 去空白后行首命中锚词;段落 = 锚词后的同行尾缀 + 后续行,至下一锚行或页尾;
    - 「注：」不设终止锚,自然并入其前段;
    - exam_name 例外:只取锚行尾缀;尾缀为空时取紧随其后的一行(不吞患者信息/日期/图像);
    - 同类锚多次出现按 OCR 行序保留多个 section;空段不产出 section。
    """
    present = [section for section in SECTION_FIELDS if anchors.get(section)]
    if not present:
        return []
    words = sorted(
        ((word, section) for section in present for word in anchors[section]),
        key=lambda pair: len(pair[0]),
        reverse=True,
    )
    hits: list[tuple[int, str, str]] = []
    for idx, text in enumerate(txts):
        stripped = text.strip()
        for word, section in words:
            if stripped.startswith(word):
                tail = re.sub(r"^[\s:：]+", "", stripped[len(word):]).strip()
                hits.append((idx, section, tail))
                break

    sections: list[ReportSection] = []
    for pos, (start, section, tail) in enumerate(hits):
        end = hits[pos + 1][0] if pos + 1 < len(hits) else len(txts)
        if section == "exam_name":
            if tail:
                lines = [(start, tail)]
            elif start + 1 < end:
                lines = [(start + 1, txts[start + 1].strip())]
            else:
                lines = []
        else:
            lines = ([(start, tail)] if tail else []) + [
                (idx, txts[idx].strip()) for idx in range(start + 1, end)
            ]
        text = "\n".join(line for _, line in lines if line)
        if text:
            sections.append(
                ReportSection(
                    kind=section,
                    text=text,
                    ocr_line_numbers=[idx for idx, line in lines if line],
                )
            )
    return sections


def run_postprocess(
    ocr: OCRResult, date_hint: str | None = None, image: bytes | None = None
) -> Report:
    """OCR 结果 → 报告。date_hint:ingest --date 兜底;检查单已解析出日期时兜底不生效。

    image 为同一次识别的原图字节(可选):RapidOCR 漏掉孤立 '-' 时,用像素确认
    结果/参考两格确实是短横线后补回 "-";/reparse 只有存库 OCR,无法恢复。
    """
    cells = _cells(ocr)
    date = _find_date(cells, date_hint)

    header_bands = _header_bands(cells)

    selected = _select_category(cells, header_bands, ocr.txts)
    if selected is None:
        return Report(
            report_type=UNKNOWN_REPORT_TYPE,
            report_date=date,
            status=Status.FAILED,
            panels=[Panel.OTHER],
            items=[],
        )
    d, pairs = selected
    sections = _extract_sections(ocr.txts, d.section_anchors)

    items: list[ReportItem] = []
    for it, piece in pairs:
        raw_cells = sorted(piece.raw_cells, key=lambda c: c.cx)
        raw_text = "  ".join(c.text.strip() for c in raw_cells) or (piece.ref or "")
        flag, flag_reason = _compare(it.name, piece.value, piece.ref)
        if flag is Flag.UNKNOWN:
            flag_reason = _extraction_reason(piece, flag_reason, raw_text)
        else:
            flag_reason = None
        low_conf = piece.low_score
        items.append(
            ReportItem(
                name=it.name,
                panel=it.panel,
                value=piece.value,
                unit=_normalize_unit(piece.unit, piece.item),
                ref_range=piece.ref,
                method=piece.method,
                flag=flag,
                flag_reason=flag_reason,
                low_confidence=low_conf,
                raw_text=raw_text,
                ocr_line_numbers=[c.line for c in raw_cells],
            )
        )

    if image is not None and items:
        for index, (value, ref_range) in recover_dash_fields(image, ocr, items).items():
            item = items[index]
            flag, reason = _compare(item.name, value, ref_range)
            items[index] = item.model_copy(
                update={
                    "value": value,
                    "ref_range": ref_range,
                    "flag": flag,
                    "flag_reason": reason if flag is Flag.UNKNOWN else None,
                }
            )
    partial = any(item.value is None for item in items)

    if d.min_section_hits > 0:
        # 按正文段落分类的报告(影像)只反映叙述正文:至少一段 findings/impression →
        # success,否则 failed;日期未印、低置信都不再降级 partial(票 04)。
        has_body = any(s.kind in BODY_SECTION_KINDS for s in sections)
        status = Status.SUCCESS if has_body else Status.FAILED
    elif not items:
        status = Status.FAILED
    else:
        performed = [it for it in items if it.flag is not Flag.NOT_TESTED]
        low_conf_majority = bool(performed) and (
            sum(1 for it in performed if it.low_confidence) / len(performed) >= LOW_CONF_PARTIAL_RATIO
        )
        if partial or date is None or low_conf_majority:
            status = Status.PARTIAL
        else:
            status = Status.SUCCESS
    panels = report_panels(d.report_type, status, (item.panel for item in items))
    return Report(
        report_type=d.report_type,
        report_date=date,
        status=status,
        panels=panels,
        items=items,
        sections=sections or None,
    )


def classify_items(
    report_type: str, names: list[str]
) -> tuple[list[Panel], list[Panel]]:
    """按 report_type + 当前指标名重算项目组(/classify 与存量回填共用;spec §6.4)。

    只查同一 report_type 下的词典(歧义项靠父类区分,如临检葡萄糖 → 尿常规),
    未命中 → other。空 items:影像 → [imaging],其余 → [other]。
    """
    dicts = load_dicts()
    if report_type not in {d.report_type for d in dicts} | {UNKNOWN_REPORT_TYPE}:
        raise ValueError(f"unknown report_type: {report_type}")
    allowed = allowed_panels(report_type)
    item_panels: list[Panel] = []
    for name in names:
        panel = Panel.OTHER
        for d in dicts:
            if d.report_type != report_type:
                continue
            item, _ = _match_name(name, d)
            if item is not None:
                panel = item.panel
                break
        if panel not in allowed:
            raise ValueError(
                f"item {name!r} panel {panel.value!r} is not allowed "
                f"for report_type {report_type!r}"
            )
        item_panels.append(panel)
    return report_panels(report_type, Status.SUCCESS, item_panels), item_panels


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
    txts: list[str],
) -> tuple[LabDict, list[tuple[DictItem, _RowPiece]]] | None:
    """多词典注册表分发(P3 A1 内容优先分类)。

    判定由内容驱动:指标项命中 ≥ min_matched_items,或正文锚(findings/impression)
    命中 ≥ min_section_hits 的词典「达标」。达标者按 (正文锚命中数, 指标项命中数,
    标题区命中) 排序,再并列取注册表先者(文件名序)。仅当没有任何词典达标时,才回退
    到标题命中的词典——标题不再是「硬覆盖」,正文偶发提及无法把报告抢到不相干类别。
    exam_name 锚只用于段落抽取,不参与分类(票 04)。

    标题区分层(见 _title_region_cells)用于达标者并列裁决;兜底层仍允许全页匹配,
    因为报告未达标时往往正是「标题词被印成项目名」的样本(如 glu_03 的糖化血红蛋白)。
    """
    title_cells = _title_region_cells(cells, header_bands)
    best: tuple[LabDict, list[tuple[DictItem, _RowPiece]]] | None = None
    best_key: tuple[bool, int, int, bool] | None = None
    for d in load_dicts():
        if d.mode == "narrative":
            pairs = _extract_narrative(cells, d)
        else:
            pairs = _extract_items(cells, header_bands, d)
        body_anchors = {
            kind: words
            for kind, words in d.section_anchors.items()
            if kind in BODY_SECTION_KINDS
        }
        section_hits = len(_extract_sections(txts, body_anchors))
        qualified = (
            d.min_matched_items > 0 and len(pairs) >= d.min_matched_items
        ) or (
            d.min_section_hits > 0 and section_hits >= d.min_section_hits
        )
        if not qualified and not any(kw in c.text for c in cells for kw in d.title_keywords):
            continue
        title_region = any(kw in c.text for c in title_cells for kw in d.title_keywords)
        key = (qualified, section_hits, len(pairs), title_region)
        if best_key is not None and key <= best_key:
            continue
        best, best_key = (d, pairs), key
    return best
