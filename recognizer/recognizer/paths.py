"""仓库布局路径常量:tests 与工具(golden/cli)统一从这里取,避免重复拼路径。"""

import os
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
SAMPLES = REPO_ROOT / "samples"
EXPECTED_OCR = SAMPLES / "expected" / "ocr"
EXPECTED_REPORT = SAMPLES / "expected" / "report"
# 未脱敏真实数据(NF-01):仅本机迭代,永不提交。默认 <repo>/data;
# worktree 或多检出场景用 HDD_DATA_DIR 指向主检出的 data。
DATA = Path(os.environ.get("HDD_DATA_DIR") or REPO_ROOT / "data")
LOCAL_EXPECTED_OCR = DATA / "expected" / "ocr"
LOCAL_EXPECTED_REPORT = DATA / "expected" / "report"
LOCAL_EXPECTED_VL = DATA / "expected" / "vl"
SCHEMAS = REPO_ROOT / "schemas"
SCHEMA_OCR = SCHEMAS / "ocr_result.schema.json"
SCHEMA_REPORT = SCHEMAS / "report.schema.json"
CONTRACT_TESTDATA = SCHEMAS / "testdata"
