"""仓库布局路径常量:tests 与工具(golden/cli)统一从这里取,避免重复拼路径。"""

from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
SAMPLES = REPO_ROOT / "samples"
EXPECTED_OCR = SAMPLES / "expected" / "ocr"
EXPECTED_REPORT = SAMPLES / "expected" / "report"
SCHEMAS = REPO_ROOT / "schemas"
SCHEMA_OCR = SCHEMAS / "ocr_result.schema.json"
SCHEMA_REPORT = SCHEMAS / "report.schema.json"
CONTRACT_TESTDATA = SCHEMAS / "testdata"
