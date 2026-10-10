"""表结构识别前端(ADR-0004):OCR 行框 → 单元格网格(HTML 表)。

采用 RapidAI TableStructureRec 的两个 ONNX 子模型:`wired_table_rec`(有线表)、
`lineless_table_rec`(无线表);**同时运行、择优使用**(非退化优先:行/列 ≥ 2;
并列取格数多者)。不依赖 `table_cls` 单模型路由——原型实测它误判率高,是 lft_05/
tft_02 走错子模型而退化的根因。

两子模型皆退化或抛错 → 返回 None,由 postprocess 回退现有启发式抽取(保证不劣化)。
复用调用方已算好的 OCR 行(txts/boxes/scores),不重复 OCR(NF-02)。

模型在首次实例化时按需从 ModelScope 下载;离线/镜像需自带模型并固定路径(NF-01)。
"""

from functools import lru_cache
from pathlib import Path

import numpy as np

from .contract import OCRResult, TableStructure
from .postprocess import parse_grid

type ImageInput = Path | bytes

# 行/列都 ≥ 此值才算「非退化」(与原型 clean 指标口径一致)
MIN_GRID_EDGE = 2

__all__ = ["MIN_GRID_EDGE", "ImageInput", "run_table_structure"]


@lru_cache(maxsize=1)
def _engines() -> tuple[object, object]:
    """进程内共享两子模型(模型加载/下载一次);延迟 import 避免无 TSR 场景加载。"""
    from lineless_table_rec.main import LinelessTableInput, LinelessTableRecognition
    from wired_table_rec.main import WiredTableInput, WiredTableRecognition

    return WiredTableRecognition(WiredTableInput()), LinelessTableRecognition(LinelessTableInput())


def _run_one(model: str, engine: object, image: ImageInput, ocr_result: list) -> tuple[bool, int, str, str, float] | None:
    """跑单个子模型:返回 (非退化, 格数, html, model, elapse);抛错/无输出 → None。"""
    try:
        res = engine(image, ocr_result=ocr_result)  # type: ignore[operator]
    except Exception:  # noqa: BLE001 —— 子模型对个别版式会抛任意内部错(如 polygons 为空),一律按无输出处理
        return None
    htmls = getattr(res, "pred_htmls", None)
    html = htmls[0] if htmls else getattr(res, "pred_html", None)
    if not html:
        return None
    rows = parse_grid(html)
    n_rows = len(rows)
    n_cols = max((len(r) for r in rows), default=0)
    n_cells = sum(len(r) for r in rows)
    non_degenerate = n_rows >= MIN_GRID_EDGE and n_cols >= MIN_GRID_EDGE
    return non_degenerate, n_cells, html, model, float(getattr(res, "elapse", 0.0))


def run_table_structure(image: ImageInput, ocr: OCRResult) -> TableStructure | None:
    """对一张图像跑 TSR,产出最优网格;两者皆退化/异常 → None(回退启发式)。

    image:图像路径或字节(应与 ocr 出自同一输入);ocr:已算好的 OCR 行(复用,不重复 OCR)。
    """
    wired, lineless = _engines()
    boxes = np.array(ocr.boxes, dtype=np.float32)
    ocr_result = [(box, t, s) for box, t, s in zip(boxes, ocr.txts, ocr.scores)]

    best: tuple[bool, int, str, str, float] | None = None
    for model, engine in (("wired", wired), ("lineless", lineless)):
        cand = _run_one(model, engine, image, ocr_result)
        if cand is None:
            continue
        if best is None or (cand[0], cand[1]) > (best[0], best[1]):
            best = cand
    if best is None or not best[0]:
        return None
    _, _, html, model, elapse = best
    return TableStructure(html=html, model=model, elapse=elapse)
