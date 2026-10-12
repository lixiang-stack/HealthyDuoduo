-- 契约变更:report_items.value 从 DOUBLE PRECISION 改为 TEXT(印刷原样数值字符串,
-- 如 '18.40'、'<2.00' 检出限比较式、'70-80' 范围型结果——schemas/report.schema.json
-- 与 pydantic contract.py 同步为 string|null;ADR 见 docs/adr/0005-item-value-to-string)。

-- +goose Up
ALTER TABLE report_items ALTER COLUMN value TYPE TEXT USING value::text;

-- +goose Down
-- 反向仅回转纯数值行;文本值(比较式等)无法无损回转,置 NULL。
ALTER TABLE report_items ALTER COLUMN value TYPE DOUBLE PRECISION
    USING CASE WHEN value ~ '^-?[0-9]+(\.[0-9]+)?$' THEN value::double precision ELSE NULL END;
