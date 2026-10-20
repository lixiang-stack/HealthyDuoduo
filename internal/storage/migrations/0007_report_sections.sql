-- 0007:reports 增报告级叙述段落(可选集合;影像等叙述型报告)。
-- 与识别端 contract.Report.sections 1:1:JSONB 数组,元素
-- {kind, text, ocr_line_numbers};非叙述型报告/存量行为 NULL。
-- 段落是 Report 内的值对象,只读展示;若未来需要段落级核对/检索再迁独立表(ADR-0008)。
-- 版本从 0006 顺延:旧 imaging-content 分支曾占用 0006(六个平铺列),0006 保留为空占位。

-- +goose Up
ALTER TABLE reports ADD COLUMN sections JSONB;

-- +goose Down
ALTER TABLE reports DROP COLUMN sections;
