-- 0017:report_items.analyte 分析物语义 key(识别端词典 analyte;框架层按此匹配)。
-- 存量行按 code / loinc 一次性回填;新报告由识别端写入。修正指标名时清空,
-- 避免沿用旧身份(分析层对空 analyte 不输出框架标签)。

-- +goose Up
ALTER TABLE report_items ADD COLUMN analyte TEXT;

UPDATE report_items
SET analyte = CASE
    WHEN code = '1500104A' OR loinc = '1742-6' THEN 'alt'
    WHEN code = '1500204A' OR loinc = '1920-8' THEN 'ast'
    WHEN code = '1500404A' OR loinc = '6768-6' THEN 'alp'
    WHEN code = '0100401A' OR loinc = '718-7' THEN 'hemoglobin'
    WHEN code = '0100601A' OR loinc = '787-2' THEN 'mcv'
    WHEN code = '0100701A' OR loinc = '785-6' THEN 'mch'
    WHEN code = '0100801A' OR loinc = '786-4' THEN 'mchc'
    WHEN loinc = '788-0' THEN 'rdw_cv'
    ELSE NULL
END
WHERE analyte IS NULL;

-- +goose Down
ALTER TABLE report_items DROP COLUMN IF EXISTS analyte;
