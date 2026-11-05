-- 0019:补回填总蛋白 / 白蛋白的 analyte key(llm-analysis 低蛋白/白蛋白条目)。
-- 0017/0018 只回填了当时知识条目消费的分析物;词典新增 analyte 后,
-- 存量行需要同步,否则匹配器读不到 total_protein / albumin。

-- +goose Up
UPDATE report_items
SET analyte = 'total_protein'
WHERE analyte IS NULL
  AND (code = '1100204A' OR loinc = '2885-2');

UPDATE report_items
SET analyte = 'albumin'
WHERE analyte IS NULL
  AND (code = '1100104A' OR loinc = '1751-7');

-- +goose Down
UPDATE report_items
SET analyte = NULL
WHERE analyte = 'total_protein'
  AND (code = '1100204A' OR loinc = '2885-2');

UPDATE report_items
SET analyte = NULL
WHERE analyte = 'albumin'
  AND (code = '1100104A' OR loinc = '1751-7');
