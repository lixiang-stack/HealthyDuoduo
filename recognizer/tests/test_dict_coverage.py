"""词典覆盖审计(review 修复的防回归门禁)。

背景:cbc_dict.yaml 起初由人工构造规范名/别名,未逐一对照真实样本 OCR,
出现过规范名与单据印法颠倒(红细胞平均体积 vs 平均红细胞体积)、
无证据别名等错误。本测试把「每条 OCR 别名必须能在 samples/expected/ocr 的
真实文本中出现(容忍星号/序号前缀与名称熔断数字尾)」固化为门禁:
新增一条 OCR 别名却未落样本证据时,测试失败,要求先补样本或核实印法再进词典。
标准同义名/缩写(synonyms)不走语料门,由 test_synonyms_have_source 要求
每条都有 source;canonical(name)是面向报告的规范名,允许与单据印法不同
(它是维护者的选择,不在此门禁内;但每一组 terms 必须整体有样本证据,防止孤儿词条)。
"""

import json
import re

import pytest
import yaml

from recognizer import paths
from recognizer.contract import PANEL_ALLOWED_BY_REPORT_TYPE, Panel
from recognizer.postprocess import DICT_DIR, DICT_GLOB, _name_forms

# 词典注册表:每个 *_dict.yaml 一个报告类别(测试在收集期读取文件列表)
_DICTS = sorted(DICT_DIR.glob(DICT_GLOB))

# 与 postprocess._match_name 官方语义一致的前缀形态:
# ①原样 ②去行首星号 ③去行首序号 ④去名称熔断数字尾 ⑤序号+熔断组合剥离
_CORPUS: dict[str, set[str]] | None = None


def _corpus() -> dict[str, set[str]]:
    global _CORPUS
    if _CORPUS is None:
        _CORPUS = {}
        for p in paths.EXPECTED_OCR.glob("*.json"):
            ocr = json.loads(p.read_text(encoding="utf-8"))
            for t in ocr["txts"]:
                _CORPUS.setdefault(t.strip(), set()).add(p.stem)
    return _CORPUS


def _witnessed(term: str) -> set[str]:
    """与 postprocess._match_name 官方语义一致的名称形态(经 _name_forms 派生:
    原样/去序号/去星号/两种组合),再叠加「名称熔断数值」尾缀剥离。"""
    seen: set[str] = set()
    for raw, sids in _corpus().items():
        variants = {raw}
        for f in _name_forms(raw):
            variants.add(f)
            variants.add(re.sub(r"\d+(?:\.\d+)?$", "", f).strip())
        if term in variants:
            seen |= sids
    return seen


def _ocr_terms(it: dict) -> list[str]:
    """需要语料见证的别名:旧 aliases + 新 ocr_aliases(synonyms 只要求来源)。"""
    return [*it.get("aliases", []), *(a["term"] for a in it.get("ocr_aliases") or [])]


def _synonyms(it: dict) -> list[str]:
    return [s["term"] for s in it.get("synonyms") or []]


def _all_terms(it: dict) -> list[str]:
    return [it["name"], *_synonyms(it), *_ocr_terms(it)]


def _narrative(dict_path) -> bool:
    """叙述体词典(超声):别名内嵌于叙述行,门禁按子串见证,而非整行等值。"""
    d = yaml.safe_load(dict_path.read_text(encoding="utf-8"))
    return str(d.get("category", {}).get("mode", "table")) == "narrative"


def _sub_witnessed(term: str, text: str) -> bool:
    """叙述体子串见证;ASCII 别名前后不得紧邻字母,避免短缩写被更长英文串误命中(如 HC 命中 HCT/MCHC/HCG)。

    数字紧邻仍算证据(HL36mm / AC174m 是叙述体量值印法);中文/标点/空格不设限。
    """
    if not term.isascii():
        return term in text
    return re.search(rf"(?<![A-Za-z]){re.escape(term)}(?![A-Za-z])", text) is not None


def _witnessed_sub(term: str) -> set[str]:
    """门禁路径与 _witnessed 相同(序号/熔断容忍的整行变体),叙述体改为子串匹配。"""
    seen: set[str] = set()
    for raw, sids in _corpus().items():
        variants = {
            raw,
            re.sub(r"^[*＊※✱]+", "", raw),
            re.sub(r"^\d+[.、．]?[\s]*", "", raw),
        }
        if any(_sub_witnessed(term, v) for v in variants):
            seen |= sids
    return seen


@pytest.mark.parametrize("dict_path", _DICTS, ids=lambda p: p.stem)
def test_every_alias_has_corpus_evidence(dict_path) -> None:
    d = yaml.safe_load(dict_path.read_text(encoding="utf-8"))
    problems = []
    for it in d.get("items") or []:
        for a in set(_ocr_terms(it)):
            ids = _witnessed_sub(a) if _narrative(dict_path) else _witnessed(a)
            if not ids:
                problems.append(f"{it['name']}: alias {a!r} 无任何样本证据")
    assert not problems, "词典别名与真实样本脱钩:\n" + "\n".join(problems)


@pytest.mark.parametrize("dict_path", _DICTS, ids=lambda p: p.stem)
def test_synonyms_have_source(dict_path) -> None:
    """标准同义名/缩写必须有来源;OCR 变体才走语料证据门。"""
    d = yaml.safe_load(dict_path.read_text(encoding="utf-8"))
    problems = []
    for it in d.get("items") or []:
        for s in it.get("synonyms") or []:
            if not s.get("source"):
                problems.append(f"{it['name']}: synonym {s.get('term')!r} 无 source")
    assert not problems, "标准同义名缺来源:\n" + "\n".join(problems)


@pytest.mark.skipif(not _DICTS, reason="no *_dict.yaml in recognizer package")
@pytest.mark.parametrize("dict_path", _DICTS, ids=lambda p: p.stem)
def test_panel_declarations_are_legal(dict_path) -> None:
    """category.panel 必填;每个 item 解析后的 panel 属于父类允许集合(spec §5.1)。"""
    d = yaml.safe_load(dict_path.read_text(encoding="utf-8"))
    report_type = d["report_type"]
    category_panel = d["category"].get("panel")
    assert category_panel in {p.value for p in Panel}, f"{dict_path}: invalid category.panel {category_panel!r}"
    allowed = PANEL_ALLOWED_BY_REPORT_TYPE.get(report_type, frozenset({Panel.OTHER}))
    problems = []
    for it in d.get("items") or []:
        panel = Panel(it.get("panel", category_panel))
        if panel not in allowed:
            problems.append(f"{it['name']}: {panel.value} 不属于 {report_type} 的允许集合")
    assert not problems, "词典 panel 越界:\n" + "\n".join(problems)


@pytest.mark.skipif(not _DICTS, reason="no *_dict.yaml in recognizer package")
@pytest.mark.parametrize("dict_path", _DICTS, ids=lambda p: p.stem)
def test_panel_basis_comments(dict_path) -> None:
    """9 个词典都有依据注释;混合词典顶部 block 覆盖其全部 item.panel(spec §5.5.5)。"""
    text = dict_path.read_text(encoding="utf-8")
    d = yaml.safe_load(text)
    lines = text.splitlines()
    item_panels = {it["panel"] for it in d.get("items") or [] if "panel" in it}
    if item_panels:
        top: list[str] = []
        for line in lines:
            if line.startswith("#"):
                top.append(line)
            elif line.strip():
                break
        block = "\n".join(top)
        assert "# panel 依据" in block, f"{dict_path}: missing top '# panel 依据' block"
        for panel in sorted(item_panels):
            assert re.search(rf"\b{re.escape(panel)}\b", block), (
                f"{dict_path}: top block does not cover item panel {panel}"
            )
    else:
        panel_idx = next(
            i for i, line in enumerate(lines) if line.strip().startswith("panel: ")
        )
        basis = [i for i, line in enumerate(lines[:panel_idx]) if line.strip().startswith("# 依据:")]
        assert basis, f"{dict_path}: missing '# 依据:' above category.panel"


@pytest.mark.skipif(not _DICTS, reason="no *_dict.yaml in recognizer package")
@pytest.mark.parametrize("dict_path", _DICTS, ids=lambda p: p.stem)
def test_every_item_has_witnessed_reference(dict_path) -> None:
    """每一词典组(规范名 + 别名)至少有一条真实样本证据,防孤儿词条。"""
    d = yaml.safe_load(dict_path.read_text(encoding="utf-8"))
    problems = []
    for it in d.get("items") or []:
        terms = set(_all_terms(it))
        if not any(_witnessed_sub(t) if _narrative(dict_path) else _witnessed(t) for t in terms):
            problems.append(f"{it['name']}: 规范名与全部别名均无样本证据")
    assert not problems, "孤儿词条:\n" + "\n".join(problems)
