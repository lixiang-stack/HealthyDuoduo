-- 0019:未上线阶段的 schema 基线。产品未发布,不再保留 0001–0018 的过程迁移;
-- 本文件是当前完整 schema 的唯一来源。goose 记录 version 19:已跑过 0018 的本地库
-- 直接跳过,新库也落在 19,后续迁移从 0020 开始。
--
-- 数据模型要点:
--   身份 = 图像内容 sha256(images 主键;reports 每图一行)
--   原图 / OCR 历史 / 报告三层留存;OCR 重跑追加,不覆盖历史
--   report_items 装最终生效值;修正历史进 append-only correction_pairs(ADR-0005)
--   项目组按指标项分类,报告级 panels 只做聚合(ADR-0012)
--   成员只存称呼 / 性别 / 出生日期;是否孩子由出生日期实时计算(ADR-0015)
--   分析快照永久保留;反馈 / 降级事件 / 解释层调用次数用于观测(ADR-0015 之后)

-- +goose Up
CREATE TABLE members (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sex        TEXT NOT NULL CONSTRAINT members_sex_check CHECK (sex IN ('male', 'female')),
    birth_date DATE NOT NULL
);

CREATE TABLE images (
    sha256            TEXT PRIMARY KEY,
    original_filename TEXT NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE ocr_results (
    id           BIGSERIAL PRIMARY KEY, -- 识别历史内部序号;重跑追加
    image_sha256 TEXT NOT NULL REFERENCES images (sha256),
    engine_output JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE reports (
    image_sha256  TEXT PRIMARY KEY REFERENCES images (sha256), -- 每图一行
    ocr_result_id BIGINT NOT NULL REFERENCES ocr_results (id),
    report_type   TEXT NOT NULL,
    report_date   DATE, -- 识别失败且未补录时为 NULL
    status        TEXT NOT NULL CHECK (status IN ('success', 'partial', 'failed')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    member_id     BIGINT REFERENCES members (id),
    fail_reason   TEXT,
    sections      JSONB, -- 叙述型报告段落,可空(ADR-0008/0009)
    panels        TEXT[] NOT NULL DEFAULT '{}' -- 报告级项目组聚合,panels[0] 为主标签
);

CREATE TABLE report_items (
    id               BIGSERIAL PRIMARY KEY,
    report_sha256    TEXT NOT NULL REFERENCES reports (image_sha256) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    value            TEXT, -- 印刷原样(如 '<2.00'、'70-80')
    unit             TEXT,
    ref_range        TEXT,
    flag             TEXT NOT NULL CONSTRAINT report_items_flag_check
        CHECK (flag IN ('normal', 'high', 'low', 'abnormal', 'unknown', 'not_tested')),
    low_confidence   BOOLEAN NOT NULL,
    raw_text         TEXT NOT NULL,
    method           TEXT,
    review_status    TEXT NOT NULL DEFAULT 'unreviewed'
        CONSTRAINT review_status_check CHECK (review_status IN ('unreviewed', 'confirmed', 'corrected')),
    ocr_line_numbers INTEGER[], -- 0-based,与 ocr_results.engine_output 的 txts/boxes 同下标
    flag_reason      TEXT
        CONSTRAINT report_items_flag_reason_check CHECK (flag_reason IS NULL OR flag_reason IN (
            'ocr_miss', 'ocr_merge', 'extractor_pairing', 'no_reference',
            'qualitative_unsupported', 'unverified', 'narrative',
            'informational', 'indeterminate', 'scalar_ref', 'other'
        )),
    panel            TEXT NOT NULL DEFAULT 'other',
    code             TEXT,
    loinc            TEXT,
    unit_display     TEXT,
    unit_status      TEXT,
    raw_value        TEXT,
    raw_unit         TEXT,
    raw_ref_range    TEXT,
    ref_kind         TEXT,
    result_kind      TEXT,
    result_basis     TEXT,
    analyte          TEXT -- 分析物语义 key(识别端词典;修正名称时清空)
);

CREATE TABLE correction_pairs (
    id               BIGSERIAL PRIMARY KEY,
    item_id          BIGINT NOT NULL REFERENCES report_items (id) ON DELETE CASCADE,
    before_name      TEXT,
    before_value     TEXT,
    before_unit      TEXT,
    before_ref_range TEXT,
    after_name       TEXT,
    after_value      TEXT,
    after_unit       TEXT,
    after_ref_range  TEXT,
    raw_text         TEXT NOT NULL,
    ocr_line_numbers INTEGER[] NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE growth_records (
    id          BIGSERIAL PRIMARY KEY,
    member_id   BIGINT NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    measured_on DATE NOT NULL,
    height_cm   DOUBLE PRECISION CHECK (height_cm > 0),
    weight_kg   DOUBLE PRECISION CHECK (weight_kg > 0),
    source      TEXT NOT NULL CHECK (source IN ('measured', 'report')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT growth_records_measurement_present CHECK (height_cm IS NOT NULL OR weight_kg IS NOT NULL)
);

CREATE TABLE analyses (
    id             BIGSERIAL PRIMARY KEY,
    member_id      BIGINT NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    panel          TEXT NOT NULL,
    entry          TEXT NOT NULL CHECK (entry IN ('panel', 'indicator', 'clinical_note')),
    indicator_name TEXT,
    window_months  INTEGER NOT NULL CHECK (window_months IN (6, 12, 24)),
    status         TEXT NOT NULL CHECK (status IN ('success', 'insufficient_evidence')),
    layers         JSONB NOT NULL,
    snapshot       JSONB NOT NULL,
    sources        JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE analysis_feedback (
    id          BIGSERIAL PRIMARY KEY,
    analysis_id BIGINT NOT NULL REFERENCES analyses (id) ON DELETE CASCADE,
    entry_id    TEXT CHECK (entry_id IS NULL OR length(entry_id) <= 200),
    reason      TEXT NOT NULL CHECK (length(reason) BETWEEN 1 AND 500),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE analysis_events (
    id          BIGSERIAL PRIMARY KEY,
    analysis_id BIGINT NOT NULL REFERENCES analyses (id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN (
                    'llm_timeout', 'llm_invalid_json', 'bridge_text_rejected',
                    'quota_exhausted', 'trust_gate_failed', 'entry_miss',
                    'llm_error', 'input_too_large'
                )),
    detail      TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE analysis_llm_calls (
    id          BIGSERIAL PRIMARY KEY,
    analysis_id BIGINT NOT NULL REFERENCES analyses (id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX reports_member_date_idx ON reports (member_id, report_date DESC);
CREATE INDEX report_items_report_panel_idx ON report_items (report_sha256, panel);
CREATE INDEX growth_records_member_measured_idx
    ON growth_records (member_id, measured_on DESC, id DESC);
CREATE INDEX analyses_member_created_idx ON analyses (member_id, created_at DESC, id DESC);
CREATE INDEX analysis_feedback_created_idx ON analysis_feedback (created_at DESC, id DESC);
CREATE INDEX analysis_feedback_analysis_idx ON analysis_feedback (analysis_id);
CREATE INDEX analysis_events_created_idx ON analysis_events (created_at DESC, id DESC);
CREATE INDEX analysis_events_analysis_idx ON analysis_events (analysis_id);
CREATE INDEX analysis_llm_calls_created_idx ON analysis_llm_calls (created_at);

-- +goose Down
DROP TABLE analysis_llm_calls;
DROP TABLE analysis_events;
DROP TABLE analysis_feedback;
DROP TABLE analyses;
DROP TABLE growth_records;
DROP TABLE correction_pairs;
DROP TABLE report_items;
DROP TABLE reports;
DROP TABLE ocr_results;
DROP TABLE images;
DROP TABLE members;
