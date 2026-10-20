-- 0008:影像单停止指标项抽取后的一次性清理(票 04)。
-- 影像 items 是检验单模型错配数据(产科测量项),无核对价值;直接删除,
-- correction_pairs 经 report_items 的 ON DELETE CASCADE 一并清除。
-- Down 不做数据恢复:reparse 可重新生成报告,但新规则下影像报告仍为零 items。

-- +goose Up
DELETE FROM report_items USING reports
 WHERE report_items.report_sha256 = reports.image_sha256
   AND reports.report_type = '影像';

-- +goose Down
SELECT 1;
