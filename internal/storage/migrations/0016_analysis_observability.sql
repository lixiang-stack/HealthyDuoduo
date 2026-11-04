-- 0016:分析反馈、降级事件与解释层配额(llm-analysis 票 07/09)。
-- 反馈只记录，不自动降级条目；降级事件供每周计算降级率；
-- analysis_llm_calls 一行 = 一次解释层调用尝试（配额按天计数，前两层不消耗）。

-- +goose Up
CREATE TABLE analysis_feedback (
    id          BIGSERIAL PRIMARY KEY,
    analysis_id BIGINT NOT NULL REFERENCES analyses (id) ON DELETE CASCADE,
    entry_id    TEXT CHECK (entry_id IS NULL OR length(entry_id) <= 200),
    reason      TEXT NOT NULL CHECK (length(reason) BETWEEN 1 AND 500),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX analysis_feedback_created_idx ON analysis_feedback (created_at DESC, id DESC);
CREATE INDEX analysis_feedback_analysis_idx ON analysis_feedback (analysis_id);

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

CREATE INDEX analysis_events_created_idx ON analysis_events (created_at DESC, id DESC);
CREATE INDEX analysis_events_analysis_idx ON analysis_events (analysis_id);

CREATE TABLE analysis_llm_calls (
    id          BIGSERIAL PRIMARY KEY,
    analysis_id BIGINT NOT NULL REFERENCES analyses (id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX analysis_llm_calls_created_idx ON analysis_llm_calls (created_at);

-- +goose Down
DROP TABLE analysis_llm_calls;
DROP TABLE analysis_events;
DROP TABLE analysis_feedback;
