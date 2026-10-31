-- 0012:report_items 标准层字段(medical-standardization 票 01)。
-- 本期只打通契约/存储/API/App 透传:识别端尚未产出的字段为 NULL,
-- 旧行无需回填即可读;实际输出由票 03+ 逐 panel 启用。
-- 不设 CHECK:枚举由识别端 / OpenAPI / TypeScript 契约校验,DB 保留后续扩展空间。

-- +goose Up
ALTER TABLE report_items ADD COLUMN code TEXT;
ALTER TABLE report_items ADD COLUMN loinc TEXT;
ALTER TABLE report_items ADD COLUMN unit_display TEXT;
ALTER TABLE report_items ADD COLUMN unit_status TEXT;
ALTER TABLE report_items ADD COLUMN raw_value TEXT;
ALTER TABLE report_items ADD COLUMN raw_unit TEXT;
ALTER TABLE report_items ADD COLUMN raw_ref_range TEXT;
ALTER TABLE report_items ADD COLUMN ref_kind TEXT;
ALTER TABLE report_items ADD COLUMN result_kind TEXT;
ALTER TABLE report_items ADD COLUMN result_basis TEXT;

-- +goose Down
ALTER TABLE report_items DROP COLUMN IF EXISTS result_basis;
ALTER TABLE report_items DROP COLUMN IF EXISTS result_kind;
ALTER TABLE report_items DROP COLUMN IF EXISTS ref_kind;
ALTER TABLE report_items DROP COLUMN IF EXISTS raw_ref_range;
ALTER TABLE report_items DROP COLUMN IF EXISTS raw_unit;
ALTER TABLE report_items DROP COLUMN IF EXISTS raw_value;
ALTER TABLE report_items DROP COLUMN IF EXISTS unit_status;
ALTER TABLE report_items DROP COLUMN IF EXISTS unit_display;
ALTER TABLE report_items DROP COLUMN IF EXISTS loinc;
ALTER TABLE report_items DROP COLUMN IF EXISTS code;
