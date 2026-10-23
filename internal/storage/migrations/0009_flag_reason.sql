-- 0009:report_items 增 flag_reason(unknown 的原因,服务端计算;normal/high/low/abnormal 为 NULL),
-- 并把 flag CHECK 扩到 abnormal(定性无方向异常;high/low 仍只用于数值方向异常)。

-- +goose Up
ALTER TABLE report_items ADD COLUMN flag_reason TEXT;
ALTER TABLE report_items DROP CONSTRAINT IF EXISTS report_items_flag_check;
ALTER TABLE report_items ADD CONSTRAINT report_items_flag_check
    CHECK (flag IN ('normal', 'high', 'low', 'abnormal', 'unknown'));
ALTER TABLE report_items ADD CONSTRAINT report_items_flag_reason_check
    CHECK (flag_reason IS NULL OR flag_reason IN (
        'ocr_miss', 'ocr_merge', 'extractor_pairing', 'no_reference',
        'qualitative_unsupported', 'unverified', 'narrative',
        'informational', 'indeterminate', 'scalar_ref', 'other'
    ));

-- +goose Down
ALTER TABLE report_items DROP CONSTRAINT IF EXISTS report_items_flag_reason_check;
ALTER TABLE report_items DROP CONSTRAINT IF EXISTS report_items_flag_check;
ALTER TABLE report_items ADD CONSTRAINT report_items_flag_check
    CHECK (flag IN ('normal', 'high', 'low', 'unknown'));
ALTER TABLE report_items DROP COLUMN flag_reason;
