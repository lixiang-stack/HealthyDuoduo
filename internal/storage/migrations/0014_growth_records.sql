-- 0014:生长记录(llm-analysis 票 02)。
-- 身高体重是会变化的测量值,不放进 members;每次测量一行,分析取最近一次并标注日期。
-- BMI 不落列:用测量时点的身高体重实时计算,避免缓存与身高体重不一致。

-- +goose Up
CREATE TABLE growth_records (
    id          BIGSERIAL PRIMARY KEY,
    member_id   BIGINT NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    measured_on DATE NOT NULL,
    height_cm   DOUBLE PRECISION CHECK (height_cm > 0),
    weight_kg   DOUBLE PRECISION CHECK (weight_kg > 0),
    source      TEXT NOT NULL CHECK (source IN ('measured', 'report', 'manual')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT growth_records_measurement_present CHECK (height_cm IS NOT NULL OR weight_kg IS NOT NULL)
);

CREATE INDEX growth_records_member_measured_idx
    ON growth_records (member_id, measured_on DESC, id DESC);

-- +goose Down
DROP TABLE growth_records;
