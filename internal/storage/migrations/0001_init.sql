-- 0001:检查报告管理 MVP 四表(实施计划 4.4;验收轮修订:身份=图像内容 sha256)。
-- 原图三层留存:images(SeaweedFS 对象,主键=内容 sha256,天然去重)→
-- ocr_results(RapidOCR 全量输出,--force 重跑时历史追加,NF-05)→
-- reports(每图一行,主键=图像 sha256)→ report_items(指标项)。

-- +goose Up
CREATE TABLE images (
    sha256            TEXT PRIMARY KEY,
    original_filename TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ocr_results (
    id           BIGSERIAL PRIMARY KEY, -- 识别历史内部序号(NF-05 追加留存)
    image_sha256 TEXT NOT NULL REFERENCES images (sha256),
    engine_output JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE reports (
    image_sha256  TEXT PRIMARY KEY REFERENCES images (sha256), -- 每图一行(实施计划 4.4)
    ocr_result_id BIGINT NOT NULL REFERENCES ocr_results (id),
    report_type   TEXT NOT NULL,
    report_date   DATE, -- 识别失败且未补录时为 NULL(实施计划 4.2)
    status        TEXT NOT NULL CHECK (status IN ('success', 'partial', 'failed')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE report_items (
    id             BIGSERIAL PRIMARY KEY,
    report_sha256  TEXT NOT NULL REFERENCES reports (image_sha256) ON DELETE CASCADE,
    name           TEXT NOT NULL,
    value          DOUBLE PRECISION,
    unit           TEXT,
    ref_range      TEXT,
    flag           TEXT NOT NULL CHECK (flag IN ('normal', 'high', 'low', 'unknown')),
    low_confidence BOOLEAN NOT NULL,
    raw_text       TEXT NOT NULL
);

-- +goose Down
DROP TABLE report_items;
DROP TABLE reports;
DROP TABLE ocr_results;
DROP TABLE images;
