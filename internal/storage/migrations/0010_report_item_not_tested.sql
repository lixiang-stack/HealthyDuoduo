-- 0010:report_items.flag 增加 not_tested(结果与参考区间均为裸短横 "-" = 本次未检测)。
-- 该状态不是 normal,也不是 unknown;不进入待核对/趋势/异常计数。

-- +goose Up
ALTER TABLE report_items DROP CONSTRAINT IF EXISTS report_items_flag_check;
ALTER TABLE report_items ADD CONSTRAINT report_items_flag_check
    CHECK (flag IN ('normal', 'high', 'low', 'abnormal', 'unknown', 'not_tested'));

-- +goose Down
ALTER TABLE report_items DROP CONSTRAINT IF EXISTS report_items_flag_check;
ALTER TABLE report_items ADD CONSTRAINT report_items_flag_check
    CHECK (flag IN ('normal', 'high', 'low', 'abnormal', 'unknown'));
