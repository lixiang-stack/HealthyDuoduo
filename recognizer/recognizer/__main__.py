"""调试 CLI:图 → stdout 输出 OCR JSON(实施计划 8-2)。

用法:python -m recognizer <image>
与 /ocr 端点共用 run_ocr 实现,保证宿主机、容器、HTTP 三通道输出一致;
容器内同款命令(验收 8-3:容器 exec 与宿主机 uv run 输出 txts 一致)。
"""

import sys
from pathlib import Path

from PIL import UnidentifiedImageError
from pydantic import ValidationError

from .ocr import run_ocr


def main(argv: list[str] | None = None) -> int:
    args = sys.argv[1:] if argv is None else argv
    if len(args) != 1:
        print("usage: python -m recognizer <image>", file=sys.stderr)
        return 2
    try:
        result = run_ocr(Path(args[0]))
    except (OSError, UnidentifiedImageError) as exc:
        print(f"cannot read image {args[0]}: {exc}", file=sys.stderr)
        return 1
    except ValidationError as exc:
        print(f"OCR result violates the contract: {exc}", file=sys.stderr)
        return 1
    print(result.model_dump_json())
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
