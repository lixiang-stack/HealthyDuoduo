-- 0005:report_items 增 ocr_line_numbers(识别端直吐的组成 OCR 行下标,0-based,
-- 与 ocr_results.engine_output 的 txts/boxes 同下标;顺序与 raw_text 组成顺序一致)。
-- 详情/核对的行链接由此精确映射,不再做 raw_text 文本反推。
-- 可空:存量行在 reparse 回填前为 NULL(链接退化为空数组,不猜)。

-- +goose Up
ALTER TABLE report_items
    ADD COLUMN ocr_line_numbers INTEGER[];

-- +goose Down
ALTER TABLE report_items
    DROP COLUMN ocr_line_numbers;
