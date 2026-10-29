"""App E2E 确定性回放:按图像 sha256 返回 samples/expected 下的固定识别产物。

只由专用 E2E 栈通过 RECOGNIZER_REPLAY_SAMPLES(逗号分隔样本 id)启用;未列出的
图像仍走真实 OCR,供粗粒度 OCR smoke 使用。产物直接取 golden(OCR + report),
不另存一份 fixture,避免与识别侧 golden 漂移。生产/共享栈不设置该环境变量。

掩码 OCR 语料没有原图,不能按真实图片回放;E2E 用 RECOGNIZER_REPLAY_IMAGES 指向
测试替身图目录(仅 e2e/run.sh 挂载),按样本 id 找替身图,再映射到同一份 golden。
替身图只承载 sha,不参与 OCR 质量判断。
"""

import hashlib
import os
from pathlib import Path

from pydantic import ValidationError

from . import paths
from .contract import OCRResult, Report, ReportResponse

ENV_REPLAY_SAMPLES = "RECOGNIZER_REPLAY_SAMPLES"
ENV_REPLAY_IMAGES = "RECOGNIZER_REPLAY_IMAGES"


class Replay:
    """固定样本的 golden 产物索引:图像 sha256 → {ocr_result, report}。"""

    def __init__(self, sample_ids: list[str], image_dirs: list[Path] | None = None) -> None:
        self._by_sha: dict[str, ReportResponse] = {}
        self._image_dirs = [paths.SAMPLES, *(image_dirs or [])]
        for sample_id in sample_ids:
            image = self._find_image(sample_id)
            ocr_path = paths.EXPECTED_OCR / f"{sample_id}.json"
            report_path = paths.EXPECTED_REPORT / f"{sample_id}.json"
            try:
                ocr = OCRResult.model_validate_json(ocr_path.read_text(encoding="utf-8"))
                report = Report.model_validate_json(report_path.read_text(encoding="utf-8"))
            except (OSError, ValidationError) as exc:
                raise ValueError(
                    f"replay sample {sample_id!r}: invalid golden ({ocr_path}, {report_path}): {exc}"
                ) from exc
            sha = hashlib.sha256(image.read_bytes()).hexdigest()
            self._by_sha[sha] = ReportResponse(ocr_result=ocr, report=report)

    def _find_image(self, sample_id: str) -> Path:
        for directory in self._image_dirs:
            for suffix in paths.IMAGE_SUFFIXES:
                candidate = directory / f"{sample_id}{suffix}"
                if candidate.is_file():
                    return candidate
        raise FileNotFoundError(
            f"replay sample {sample_id!r}: no image under "
            + ", ".join(str(directory) for directory in self._image_dirs)
        )

    def lookup(self, data: bytes) -> ReportResponse | None:
        """命中固定样本 → 预置产物;未命中 → None(调用方继续真实 OCR)。"""
        return self._by_sha.get(hashlib.sha256(data).hexdigest())

    @classmethod
    def from_env(cls) -> "Replay | None":
        raw = os.environ.get(ENV_REPLAY_SAMPLES, "")
        sample_ids = [part.strip() for part in raw.split(",") if part.strip()]
        if not sample_ids:
            return None
        extra_images = os.environ.get(ENV_REPLAY_IMAGES)
        image_dirs = [Path(extra_images)] if extra_images else []
        return cls(sample_ids, image_dirs)
