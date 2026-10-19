-- 0006:reports 增报告级叙述段落六列(影像单检查项目 / 检查所见 / 超声提示)。
-- 与识别端 contract.Report 的六个可空字段 1:1:三个 TEXT + 三个 INTEGER[],
-- 存量行 NULL,refresh 全量 reparse 自动回填。文本与行号成对(识别端契约已锁)。

-- +goose Up
ALTER TABLE reports
    ADD COLUMN exam_name TEXT,
    ADD COLUMN exam_name_ocr_line_numbers INTEGER[],
    ADD COLUMN findings TEXT,
    ADD COLUMN findings_ocr_line_numbers INTEGER[],
    ADD COLUMN impression TEXT,
    ADD COLUMN impression_ocr_line_numbers INTEGER[];

-- +goose Down
ALTER TABLE reports
    DROP COLUMN exam_name,
    DROP COLUMN exam_name_ocr_line_numbers,
    DROP COLUMN findings,
    DROP COLUMN findings_ocr_line_numbers,
    DROP COLUMN impression,
    DROP COLUMN impression_ocr_line_numbers;
