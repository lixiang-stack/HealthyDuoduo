-- 0013:成员模型扩展(llm-analysis 票 01)。
-- 性别 + 出生日期成为建成员的必填信息;是否孩子改为服务端按生日实时计算,
-- 不再保留手选的 is_child 列,也不提供出生日期精度。
-- 破坏性迁移:仅限产品未上线阶段。先解除报告归属并清空 members,再建 NOT NULL 列;
-- 不采用 TRUNCATE members CASCADE —— 那会连带清空 reports,而本迁移要求报告保留、
-- 只把 reports.member_id 置空(无主存量报告仍在时间线中可见)。

-- +goose Up
UPDATE reports SET member_id = NULL;
DELETE FROM members;

ALTER TABLE members
    DROP COLUMN is_child,
    ADD COLUMN sex TEXT NOT NULL
        CONSTRAINT members_sex_check CHECK (sex IN ('male', 'female')),
    ADD COLUMN birth_date DATE NOT NULL;

-- +goose Down
ALTER TABLE members
    DROP COLUMN IF EXISTS birth_date,
    DROP COLUMN IF EXISTS sex,
    ADD COLUMN is_child BOOLEAN NOT NULL DEFAULT false;
