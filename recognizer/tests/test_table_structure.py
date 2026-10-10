"""表结构识别前端(ADR-0004)单测:两子模型「非退化优先、格数多者」择优与退化回退。

用假子模型替换真引擎(不加载模型、不联网):覆盖 择优/退化/异常。
"""

from types import SimpleNamespace

from recognizer import table_structure as ts
from recognizer.contract import ModelInfo, OCRResult

_GOOD = "<table><tr><td>a</td><td>b</td></tr><tr><td>c</td><td>d</td></tr></table>"
_BAD = "<table><tr><td>a</td></tr></table>"  # 1 列 → 退化


def _ocr() -> OCRResult:
    return OCRResult(
        txts=[], boxes=[], scores=[], elapse=0.0, elapse_list=[],
        engine="onnxruntime", model_info=ModelInfo(det="d", cls="c", rec="r"),
    )


class _Engine:
    def __init__(self, html: str | None = None, exc: bool = False) -> None:
        self.html = html
        self.exc = exc
        self.calls = 0

    def __call__(self, image, ocr_result=None):
        self.calls += 1
        if self.exc:
            raise RuntimeError("boom")
        return SimpleNamespace(pred_html=self.html, pred_htmls=[self.html] if self.html else None, elapse=0.2)


def _patch(monkeypatch, wired: _Engine, lineless: _Engine) -> None:
    monkeypatch.setattr(ts, "_engines", lambda: (wired, lineless))


def test_picks_non_degenerate(monkeypatch) -> None:
    _patch(monkeypatch, _Engine(_BAD), _Engine(_GOOD))
    out = ts.run_table_structure("x", _ocr())
    assert out is not None and out.model == "lineless"


def test_more_cells_wins_when_both_non_degenerate(monkeypatch) -> None:
    _patch(
        monkeypatch,
        _Engine("<table><tr><td>a</td><td>b</td></tr></table>"),
        _Engine("<table><tr><td>a</td><td>b</td></tr><tr><td>c</td><td>d</td></tr></table>"),
    )
    out = ts.run_table_structure("x", _ocr())
    assert out is not None and out.model == "lineless"


def test_both_degenerate_returns_none(monkeypatch) -> None:
    _patch(monkeypatch, _Engine(_BAD), _Engine(_BAD))
    assert ts.run_table_structure("x", _ocr()) is None


def test_engine_exception_treated_as_no_output(monkeypatch) -> None:
    _patch(monkeypatch, _Engine(exc=True), _Engine(_GOOD))
    out = ts.run_table_structure("x", _ocr())
    assert out is not None and out.model == "lineless"


def test_both_fail_returns_none(monkeypatch) -> None:
    _patch(monkeypatch, _Engine(exc=True), _Engine(None))
    assert ts.run_table_structure("x", _ocr()) is None


def test_produced_event_logged(monkeypatch, caplog) -> None:
    """可观测:产出时记 tsr-produced(含胜出子模型)。"""
    _patch(monkeypatch, _Engine(_GOOD), _Engine(None))
    with caplog.at_level("INFO", logger="recognizer.table_structure"):
        out = ts.run_table_structure("x", _ocr())
    assert out is not None and out.model == "wired"
    assert any("tsr-produced model=wired" in r.getMessage() for r in caplog.records)


def test_degenerate_event_logged(monkeypatch, caplog) -> None:
    """可观测:两子模型皆退化时记 tsr-degenerate(判断启发式能否下线的信号)。"""
    _patch(monkeypatch, _Engine(_BAD), _Engine(_BAD))
    with caplog.at_level("INFO", logger="recognizer.table_structure"):
        assert ts.run_table_structure("x", _ocr()) is None
    assert any("tsr-degenerate" in r.getMessage() for r in caplog.records)
