"""图 → OCR 结果(实施计划 8-1):rapidocr 配置 + onnxruntime 推理。

rapidocr 参数不散落代码:引擎按本包 rapidocr.yaml 初始化
(由 wheel 内置默认配置快照生成,`uv run rapidocr config --save_cfg_file` 可重生成);
要调整识别行为直接改该文件,三通道(宿主机/容器 CLI/HTTP)同源生效。
「长边 > 2000px 等比缩放」也是配置内置(load max_side_len=2000 / use_preprocess_img)。
OCRResult pydantic 建模即契约源,镜像 schemas/ocr_result.schema.json。
"""

from functools import lru_cache
from pathlib import Path

from PIL import UnidentifiedImageError
from rapidocr import RapidOCR

from .contract import ModelInfo, OCRResult

type ImageInput = Path | bytes

# rapidocr 升大版本若字段有变,需对照新 wheel 的 config.yaml 重新生成本文件
CONFIG_PATH = Path(__file__).parent / "rapidocr.yaml"

__all__ = ["CONFIG_PATH", "ImageInput", "UnidentifiedImageError", "run_ocr"]


@lru_cache(maxsize=1)
def _engine() -> RapidOCR:
    """进程内共享引擎(模型加载一次;HTTP 常驻与连续 CLI 均受益)。"""
    return RapidOCR(config_path=str(CONFIG_PATH))


@lru_cache(maxsize=1)
def _model_info() -> ModelInfo:
    """OCR 所用模型:取自 rapidocr 配置(Det/Rec: PP-OCRv6 small;Cls: PP-OCRv4 mobile)。"""
    cfg = _engine().cfg
    return ModelInfo(
        det=f"{cfg.Det.ocr_version.value}_{cfg.Det.model_type.value}",
        cls=f"{cfg.Cls.ocr_version.value}_{cfg.Cls.model_type.value}",
        rec=f"{cfg.Rec.ocr_version.value}_{cfg.Rec.model_type.value}",
    )


@lru_cache(maxsize=1)
def _engine_type() -> str:
    """推理引擎名:取自配置(而非硬编码);与 model_info 同一数据来源。"""
    return _engine().cfg.Det.engine_type.value


def run_ocr(image: ImageInput) -> OCRResult:
    """对一张图像执行 OCR,产出的 OCRResult 即契约实例(NF-06)。

    输入图像路径或字节内容;非图像内容抛 UnidentifiedImageError(rapidocr LoadImage 语义)。
    """
    data = Path(image).read_bytes() if isinstance(image, Path) else image
    result = _engine()(data)

    if result.txts is None:
        txts, boxes, scores = [], [], []
    else:
        txts = list(result.txts)
        boxes = [[tuple(float(v) for v in point) for point in box] for box in result.boxes]
        scores = [float(s) for s in result.scores]

    return OCRResult(
        txts=txts,
        boxes=boxes,
        scores=scores,
        elapse=float(result.elapse),
        elapse_list=[float(t) for t in result.elapse_list],
        engine=_engine_type(),
        model_info=_model_info(),
    )
