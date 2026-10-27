-- 0011:报告项目组(panel-browse 票 02)。
-- report_items.panel 存指标项级项目组 code(识别端输出;存量默认 other,回填后纠正);
-- reports.panels 存有序项目组数组(panels[0] 为主标签;存量默认空,App 回退 report_type)。
-- 不加 CHECK:枚举与父子关系由识别端 / Go 契约测试校验,新增 code 不强制迁移。

-- +goose Up
ALTER TABLE report_items ADD COLUMN panel text NOT NULL DEFAULT 'other';
ALTER TABLE reports ADD COLUMN panels text[] NOT NULL DEFAULT '{}';

CREATE INDEX report_items_report_panel_idx ON report_items (report_sha256, panel);
CREATE INDEX reports_member_date_idx ON reports (member_id, report_date DESC);

-- +goose Down
DROP INDEX IF EXISTS reports_member_date_idx;
DROP INDEX IF EXISTS report_items_report_panel_idx;

ALTER TABLE reports DROP COLUMN IF EXISTS panels;
ALTER TABLE report_items DROP COLUMN IF EXISTS panel;
