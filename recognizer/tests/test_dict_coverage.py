"""词典覆盖审计(review 修复的防回归门禁)。

背景:cbc_dict.yaml 起初由人工构造规范名/别名,未逐一对照真实样本 OCR,
出现过规范名与单据印法颠倒(红细胞平均体积 vs 平均红细胞体积)、
无证据别名等错误。本测试把「每条别名必须能在 samples/expected/ocr 的
真实文本中出现(容忍星号/序号前缀与名称熔断数字尾)」固化为门禁:
新增一条词典别名却未落样本证据时,测试失败,要求先补样本或核实印法再进词典。
canonical(name)是面向报告的规范名,允许与单据印法不同(它是维护者的选择,
不在此门禁内;但每一组 aliases∪{name} 必须整体有样本证据,防止孤儿词条)。
"""

import json
import re

import pytest
import yaml

from recognizer import paths
from recognizer.postprocess import DICT_DIR, DICT_GLOB

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
    seen: set[str] = set()
    for raw, sids in _corpus().items():
        variants = {
            raw,
            re.sub(r"^[*＊※✱]+", "", raw),
            re.sub(r"^\d+[.、．]?[\s]*", "", raw),
            re.sub(r"\d+(?:\.\d+)?$", "", raw),
            re.sub(r"\d+(?:\.\d+)?$", "", re.sub(r"^\d+[.、．]?[\s]*", "", raw)),
        }
        if term in variants:
            seen |= sids
    return seen


def _narrative(dict_path) -> bool:
    """叙述体词典(超声):别名内嵌于叙述行,门禁按子串见证,而非整行等值。"""
    d = yaml.safe_load(dict_path.read_text(encoding="utf-8"))
    return str(d.get("category", {}).get("mode", "table")) == "narrative"


def _witnessed_sub(term: str) -> set[str]:
    """门禁路径与 _witnessed 相同(序号/熔断容忍的整行变体),叙述体改为子串匹配。"""
    seen: set[str] = set()
    for raw, sids in _corpus().items():
        if term in raw or any(term in v for v in {
            re.sub(r"^[*＊※✱]+", "", raw),
            re.sub(r"^\d+[.、．]?[\s]*", "", raw),
        }):
            seen |= sids
    return seen


@pytest.mark.parametrize("dict_path", _DICTS, ids=lambda p: p.stem)
def test_every_alias_has_corpus_evidence(dict_path) -> None:
    d = yaml.safe_load(dict_path.read_text(encoding="utf-8"))
    problems = []
    for it in d["items"]:
        for a in set(it.get("aliases", [])):
            ids = _witnessed_sub(a) if _narrative(dict_path) else _witnessed(a)
            if not ids:
                problems.append(f"{it['name']}: alias {a!r} 无任何样本证据")
    assert not problems, "词典别名与真实样本脱钩:\n" + "\n".join(problems)


@pytest.mark.skipif(not _DICTS, reason="no *_dict.yaml in recognizer package")
@pytest.mark.parametrize("dict_path", _DICTS, ids=lambda p: p.stem)
def test_every_item_has_witnessed_reference(dict_path) -> None:
    """每一词典组(规范名 + 别名)至少有一条真实样本证据,防孤儿词条。"""
    d = yaml.safe_load(dict_path.read_text(encoding="utf-8"))
    problems = []
    for it in d["items"]:
        terms = {it["name"], *it.get("aliases", [])}
        if not any(_witnessed_sub(t) if _narrative(dict_path) else _witnessed(t) for t in terms):
            problems.append(f"{it['name']}: 规范名与全部别名均无样本证据")
    assert not problems, "孤儿词条:\n" + "\n".join(problems)
