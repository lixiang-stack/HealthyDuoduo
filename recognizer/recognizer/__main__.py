"""调试 CLI:图 → stdout 输出 OCR JSON(实施计划 8-2)。

用法:python -m recognizer <image> [--preprocess gray,autocontrast,deskew]
与 /ocr 端点共用 run_ocr 实现,保证宿主机、容器、HTTP 三通道输出一致;
容器内同款命令(验收 8-3:容器 exec 与宿主机 uv run 输出 txts 一致)。
preprocess:P3 预处理开关(坏例驱动,默认关闭)。
"""

import sys
from pathlib import Path

from PIL import UnidentifiedImageError
from pydantic import ValidationError

from .ocr import parse_preprocess, run_ocr


def main(argv: list[str] | None = None) -> int:
    args = sys.argv[1:] if argv is None else argv
    image: str | None = None
    preprocess: str | None = None
    rest = list(args)
    while rest:
        a = rest.pop(0)
        if a in ("--preprocess", "-preprocess"):
            if not rest:
                print("--preprocess needs a value", file=sys.stderr)
                return 2
            preprocess = rest.pop(0)
        elif image is None:
            image = a
        else:
            image = None  # 多余位置参数:统一走 usage 错误
            break
    if image is None:
        print("usage: python -m recognizer <image> [--preprocess gray,autocontrast,deskew]", file=sys.stderr)
        return 2
    try:
        parse_preprocess(preprocess)  # 步骤名合法性提前报错
    except ValueError as exc:
        print(f"invalid --preprocess: {exc}", file=sys.stderr)
        return 2
    try:
        result = run_ocr(Path(image), preprocess)
    except (OSError, UnidentifiedImageError) as exc:
        print(f"cannot read image {image}: {exc}", file=sys.stderr)
        return 1
    except ValidationError as exc:
        print(f"OCR result violates the contract: {exc}", file=sys.stderr)
        return 1
    print(result.model_dump_json())
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
