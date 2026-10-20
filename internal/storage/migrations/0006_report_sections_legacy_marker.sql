-- 0006:兼容占位(已废弃的 imaging-content 分支曾用 0006 加六个平铺段落列)。
-- 本分支改走 reports.sections JSONB(0007/ADR-0008);该文件保持版本链连续,
-- 不修改 schema。新库执行空操作;已跑过旧 0006 的库直接跳过,由 0007 补列。
-- 旧六列不在此删除(避免对既有实验库做破坏性清理);新代码不读取它们。

-- +goose Up
SELECT 1;

-- +goose Down
SELECT 1;
