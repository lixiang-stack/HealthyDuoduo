"""图 → OCR 结果(实施计划 8-1):rapidocr 配置 + onnxruntime 推理。

rapidocr 参数不散落代码:引擎按本包 rapidocr.yaml 初始化
(由 wheel 内置默认配置快照生成,`uv run rapidocr config --save_cfg_file` 可重生成);
要调整识别行为直接改该文件,三通道(宿主机/容器 CLI/HTTP)同源生效。
「长边 > 2000px 等比缩放」也是配置内置(load max_side_len=2000 / use_preprocess_img)。
OCRResult pydantic 建模即契约源,镜像 schemas/ocr_result.schema.json。

P3 预处理开关(实施计划 §10,坏例驱动):默认关闭,OCR 输入与 P1 完全一致;
开启后先按序执行 灰度/对比度/纠偏 再进引擎,产出契约不变。
"""

import io
import re
from functools import lru_cache
from pathlib import Path

import numpy as np
from PIL import Image, ImageOps, UnidentifiedImageError
from rapidocr import RapidOCR

from .contract import ModelInfo, OCRResult

type ImageInput = Path | bytes

# rapidocr 升大版本若字段有变,需对照新 wheel 的 config.yaml 重新生成本文件
CONFIG_PATH = Path(__file__).parent / "rapidocr.yaml"

# 可选预处理步骤(P3):按传入顺序依次执行;默认关闭
PREPROCESS_GRAY = "gray"  # 压平为灰度(消除淡印章/淡底色对二值化的干扰)
PREPROCESS_AUTOCONTRAST = "autocontrast"  # 直方图拉伸(暗光拍照/阴影遮挡)
PREPROCESS_DESKEW = "deskew"  # 小角度纠偏(拍摄轻微倾斜)
_PREPROCESS_STEPS = (PREPROCESS_GRAY, PREPROCESS_AUTOCONTRAST, PREPROCESS_DESKEW)

# 直方图拉伸去除两侧极端像素的截断(拍照噪声实测级)
_AUTOCONTRAST_CUTOFF = 1
# 纠偏角度搜索范围与步长:检查单拍摄轻微歪斜的量级;整页 >5° 属重拍问题,不兜底大角度
_DESKEW_MAX_DEG = 5.0
_DESKEW_STEP_DEG = 0.5
# 角度估计的降采样上限(估计与速度的折衷,输出仍用原图旋转)
_DESKEW_EST_MAX_DIM = 1000

_PREPROCESS_SPLIT_RE = re.compile(r"[,;\s]+")

__all__ = ["CONFIG_PATH", "ImageInput", "UnidentifiedImageError", "parse_preprocess", "prepare_image", "run_ocr"]


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


def parse_preprocess(spec: str | None) -> tuple[str, ...]:
    """预处理开关解析:None/空白 → 不处理;逗号/空格分隔,未知步骤名抛 ValueError。"""
    if spec is None or not spec.strip():
        return ()
    steps: list[str] = []
    for token in _PREPROCESS_SPLIT_RE.split(spec.strip()):
        if token not in _PREPROCESS_STEPS:
            raise ValueError(
                f"unknown preprocess step {token!r}; expected any of {','.join(_PREPROCESS_STEPS)}"
            )
        if token not in steps:
            steps.append(token)
    return tuple(steps)


def prepare_image(image: ImageInput, preprocess: str | None = None) -> bytes:
    """读取图像并应用预处理开关,返回进引擎的字节(OCR 与 TSR 共用同一输入)。

    与 run_ocr 的输入口径一致:默认关闭时即原图字节;开启时按序执行预处理。
    """
    steps = parse_preprocess(preprocess)
    data = Path(image).read_bytes() if isinstance(image, Path) else image
    return _apply_preprocess(data, steps) if steps else data


def run_ocr(image: ImageInput, preprocess: str | None = None) -> OCRResult:
    """对一张图像执行 OCR,产出的 OCRResult 即契约实例(NF-06)。

    输入图像路径或字节内容;非图像内容抛 UnidentifiedImageError(rapidocr LoadImage 语义)。
    preprocess:可选预处理开关(步骤名逗号/空格组合;默认关闭,与 P1 输入一致)。
    """
    data = prepare_image(image, preprocess)
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


def _apply_preprocess(data: bytes, steps: tuple[str, ...]) -> bytes:
    """按开关顺序执行预处理,产物回写 PNG bytes 再进引擎(与未开启通道同链路)。"""
    img = Image.open(io.BytesIO(data))
    img.load()
    for step in steps:
        if step == PREPROCESS_GRAY:
            img = _to_gray(img)
        elif step == PREPROCESS_AUTOCONTRAST:
            img = _autocontrast(img)
        elif step == PREPROCESS_DESKEW:
            img = _deskew(img)
    buf = io.BytesIO()
    img.save(buf, format="PNG")
    return buf.getvalue()


def _to_gray(img: Image.Image) -> Image.Image:
    """彩色图压平为灰度后再回三通道引擎输入。"""
    return img.convert("L").convert("RGB")


def _autocontrast(img: Image.Image) -> Image.Image:
    """直方图拉伸:去除两侧极端像素后重映射(暗光照/阴影遮挡的坏例)。"""
    return ImageOps.autocontrast(img, cutoff=_AUTOCONTRAST_CUTOFF)


def _deskew(img: Image.Image) -> Image.Image:
    """小角度纠偏:先在降采样图上估计倾斜角,再以白底回填旋转原图。"""
    angle = _estimate_skew_deg(img)
    if abs(angle) < _DESKEW_STEP_DEG / 2:
        return img
    fill = (255, 255, 255) if img.mode in ("RGB", "RGBA") else 255
    return img.rotate(angle, resample=Image.Resampling.BICUBIC, fillcolor=fill)


def _estimate_skew_deg(img: Image.Image) -> float:
    """倾斜角估计:前景(灰度均值以下)行投影方差最大化的旋转角;
    ±_DESKEW_MAX_DEG 以 _DESKEW_STEP_DEG 步长搜索,降采样上限控制开销。"""
    w, h = img.size
    gray_src = img.convert("L")
    if max(w, h) > _DESKEW_EST_MAX_DIM:
        scale = _DESKEW_EST_MAX_DIM / max(w, h)
        gray_src = gray_src.resize(
            (max(1, int(w * scale)), max(1, int(h * scale))), Image.Resampling.BILINEAR
        )
    gray = np.asarray(gray_src, dtype=np.float32)
    fg = Image.fromarray((gray < gray.mean()).astype(np.uint8) * 255)
    best_deg, best_var = 0.0, -1.0
    for deg in np.arange(-_DESKEW_MAX_DEG, _DESKEW_MAX_DEG + 1e-9, _DESKEW_STEP_DEG):
        rot = np.asarray(fg.rotate(deg, resample=Image.Resampling.BILINEAR))
        var = float(rot.sum(axis=1).astype(np.float64).var())
        if var > best_var:
            best_var, best_deg = var, float(deg)
    return best_deg
