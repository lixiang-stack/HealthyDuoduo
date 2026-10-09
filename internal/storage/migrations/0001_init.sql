-- 0001:检查报告管理 MVP 四表(实施计划 4.4)。
-- 原图三层留存:images(SeaweedFS 对象,sha256 唯一去重)→ ocr_results(RapidOCR 全量输出,
-- --force 重跑时历史追加,NF-05)→ reports(一图一份结构化报告)→ report_items(指标项)。

-- +goose Up
CREATE TABLE images (
    id                BIGSERIAL PRIMARY KEY,
    sha256            TEXT NOT NULL UNIQUE,
    object_key        TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ocr_results (
    id            BIGSERIAL PRIMARY KEY,
    image_id      BIGINT NOT NULL REFERENCES images (id),
    engine_output JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE reports (
    id             BIGSERIAL PRIMARY KEY,
    image_id       BIGINT NOT NULL REFERENCES images (id),
    ocr_result_id  BIGINT NOT NULL REFERENCES ocr_results (id),
    report_type    TEXT NOT NULL,
    report_date    DATE, -- 识别失败且未补录时为 NULL(实施计划 4.2)
    status         TEXT NOT NULL CHECK (status IN ('success', 'partial', 'failed')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE report_items (
    id             BIGSERIAL PRIMARY KEY,
    report_id      BIGINT NOT NULL REFERENCES reports (id) ON DELETE CASCADE,
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
