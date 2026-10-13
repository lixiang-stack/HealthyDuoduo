-- M0 契约冻结(viz-and-client 票 02;需求 v1.3「契约与 API 面」+ ADR-0005):
-- ① members(成员最小模型 = 显示名 + 是否孩子;无删除);② reports 挂成员归属
-- (可空:存量 CLI 时代报告无主)与 fail_reason(识别失败原因,票 03/06 由 pipeline 产出);
-- ③ report_items 加核对状态(未核对 --确认|修正--> 已确认|已修正,决策可重复、无撤销);
-- ④ append-only 修正对表(before→after 全集 + 原始行文 + OCR 行号;本期只存不回流)。
-- 注意:unreviewed_low_confidence_count 是响应派生值,不落列(ADR-0005 Consequences)。

-- +goose Up
CREATE TABLE members (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    is_child   BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE reports
    ADD COLUMN member_id  BIGINT REFERENCES members (id),
    ADD COLUMN fail_reason TEXT;

ALTER TABLE report_items
    ADD COLUMN review_status TEXT NOT NULL DEFAULT 'unreviewed'
        CONSTRAINT review_status_check CHECK (review_status IN ('unreviewed', 'confirmed', 'corrected'));

CREATE TABLE correction_pairs (
    id      BIGSERIAL PRIMARY KEY,
    item_id BIGINT NOT NULL REFERENCES report_items (id) ON DELETE CASCADE,
    -- before→after 全集:四个可修正的纸面事实字段各存一份快照(修正只收
    -- value/unit/name/ref_range;flag 由服务端按值×参考范围重算,不入对表)。
    before_name      TEXT,
    before_value     TEXT,
    before_unit      TEXT,
    before_ref_range TEXT,
    after_name       TEXT,
    after_value      TEXT,
    after_unit       TEXT,
    after_ref_range  TEXT,
    raw_text         TEXT NOT NULL,
    ocr_line_numbers INTEGER[] NOT NULL, -- raw_text 对应 OCR 行下标(0-based,与 boxes 同下标)
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE correction_pairs;
ALTER TABLE report_items DROP COLUMN review_status;
ALTER TABLE reports DROP COLUMN fail_reason;
ALTER TABLE reports DROP COLUMN member_id;
DROP TABLE members;
