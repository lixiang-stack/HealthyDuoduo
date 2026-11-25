-- 0021:护理事件持久化（care-events 票 01）。四类事件单表存储，类型专属字段
-- 显式落列；来源固定家长记录，没有记录人 / 设备 / 账号字段。成员删除级联清理。
-- +goose Up
CREATE TABLE care_events (
    id            BIGSERIAL PRIMARY KEY,
    member_id     BIGINT NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    type          TEXT NOT NULL CHECK (type IN ('feed', 'diaper', 'sleep', 'solid')),
    source        TEXT NOT NULL DEFAULT 'parent' CHECK (source = 'parent'),
    started_at    TIMESTAMPTZ NOT NULL,
    ended_at      TIMESTAMPTZ,
    note          TEXT,
    method        TEXT,
    amount_ml     DOUBLE PRECISION,
    duration_min  INTEGER,
    diaper_type   TEXT,
    food          TEXT,
    amount_g      DOUBLE PRECISION
);

ALTER TABLE care_events
    ADD CONSTRAINT care_events_type_fields CHECK (
        (type = 'feed'   OR method IS NULL) AND
        (type <> 'feed'  OR (method IS NOT NULL AND method IN ('breast', 'formula'))) AND
        (type = 'diaper' OR diaper_type IS NULL) AND
        (type <> 'diaper' OR (diaper_type IS NOT NULL AND diaper_type IN ('wet', 'dirty', 'mixed'))) AND
        (type = 'solid'  OR food IS NULL) AND
        (type <> 'solid' OR (food IS NOT NULL AND btrim(food) <> '')) AND
        (type = 'solid'  OR amount_g IS NULL) AND
        (type = 'feed'   OR amount_ml IS NULL) AND
        (duration_min IS NULL OR type IN ('feed', 'sleep')) AND
        (type = 'sleep'  OR ended_at IS NULL)
    ),
    ADD CONSTRAINT care_events_positive_values CHECK (
        (amount_ml IS NULL OR amount_ml > 0) AND
        (amount_g IS NULL OR amount_g > 0) AND
        (duration_min IS NULL OR duration_min > 0)
    ),
    ADD CONSTRAINT care_events_time_order CHECK (
        ended_at IS NULL OR ended_at >= started_at
    ),
    ADD CONSTRAINT care_events_sleep_pair CHECK (
        type <> 'sleep' OR ((ended_at IS NULL) = (duration_min IS NULL))
    ),
    ADD CONSTRAINT care_events_note_trimmed CHECK (
        note IS NULL OR (btrim(note) <> '' AND note = btrim(note))
    ),
    ADD CONSTRAINT care_events_food_trimmed CHECK (
        food IS NULL OR food = btrim(food)
    );

CREATE INDEX care_events_member_started_idx
    ON care_events (member_id, started_at DESC, id DESC);

-- +goose Down
DROP TABLE care_events;
