-- 契约变更:report_items 新增 method 列(检测方法/实验方法列印刷原样,如 '化学发光法';
-- schemas/report.schema.json 与 pydantic contract.py 同步为 string|null,见 0002 同源注)。

-- +goose Up
ALTER TABLE report_items ADD COLUMN method TEXT;

-- +goose Down
ALTER TABLE report_items DROP COLUMN method;
