-- 0018:analyte key 统一为 LOINC component 全称的小写 snake_case。
-- 0017 回填的缩写 key 在存量行上就地改名;总胆红素在此补回填。
-- Down 只回退本迁移引入的 key;0017 的回填语义不变。

-- +goose Up
UPDATE report_items
SET analyte = CASE analyte
    WHEN 'alt' THEN 'alanine_aminotransferase'
    WHEN 'ast' THEN 'aspartate_aminotransferase'
    WHEN 'alp' THEN 'alkaline_phosphatase'
    WHEN 'mcv' THEN 'mean_corpuscular_volume'
    WHEN 'mch' THEN 'mean_corpuscular_hemoglobin'
    WHEN 'mchc' THEN 'mean_corpuscular_hemoglobin_concentration'
    WHEN 'rdw_cv' THEN 'erythrocyte_distribution_width_cv'
    ELSE analyte
END
WHERE analyte IN ('alt', 'ast', 'alp', 'mcv', 'mch', 'mchc', 'rdw_cv');

UPDATE report_items
SET analyte = 'bilirubin_total'
WHERE analyte IS NULL
  AND (code = '1700504A' OR loinc = '1975-2');

-- +goose Down
UPDATE report_items
SET analyte = CASE analyte
    WHEN 'alanine_aminotransferase' THEN 'alt'
    WHEN 'aspartate_aminotransferase' THEN 'ast'
    WHEN 'alkaline_phosphatase' THEN 'alp'
    WHEN 'mean_corpuscular_volume' THEN 'mcv'
    WHEN 'mean_corpuscular_hemoglobin' THEN 'mch'
    WHEN 'mean_corpuscular_hemoglobin_concentration' THEN 'mchc'
    WHEN 'erythrocyte_distribution_width_cv' THEN 'rdw_cv'
    ELSE analyte
END
WHERE analyte IN (
    'alanine_aminotransferase',
    'aspartate_aminotransferase',
    'alkaline_phosphatase',
    'mean_corpuscular_volume',
    'mean_corpuscular_hemoglobin',
    'mean_corpuscular_hemoglobin_concentration',
    'erythrocyte_distribution_width_cv'
);

UPDATE report_items
SET analyte = NULL
WHERE analyte = 'bilirubin_total';
