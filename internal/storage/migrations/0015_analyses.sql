-- 0015:分析快照(llm-analysis 票 04)。
-- 每次分析永久保留成员、Panel、窗口、有效点集合、病历引用与规则版本,
-- 供之后复现当时的结论;快照记录 data_version,读取时与当前数据指纹不同即 stale。

-- +goose Up
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

CREATE INDEX analyses_member_created_idx ON analyses (member_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE analyses;
