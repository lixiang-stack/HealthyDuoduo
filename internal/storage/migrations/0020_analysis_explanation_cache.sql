-- 0020:解释层输入指纹缓存。相同证据包 / 条目版本 / prompt / 规则 / 模型直接复用,
-- 不重复调用 LLM,也不消耗每日配额。缓存按成员隔离并随成员删除级联,
-- 避免删除成员后仍保留其证据衍生的解释文本。
-- +goose Up
CREATE TABLE analysis_explanation_cache (
    member_id   BIGINT NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    cache_key   TEXT NOT NULL,
    explanation JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (member_id, cache_key)
);

-- +goose Down
DROP TABLE analysis_explanation_cache;
