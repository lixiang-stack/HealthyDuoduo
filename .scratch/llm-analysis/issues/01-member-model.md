# 01: 成员模型扩展

**What to build:** 建成员时必须填写性别和出生日期；性别提供「男 / 女 / 不确定」三选一；只知道出生年份时允许只填年份并标记精度；修改成员时可改这些字段。产品未上线，迁移时清空现有成员并解除报告归属。

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] 建成员时缺性别或出生日期返回 400，错误信息指明缺哪个字段
- [x] 性别接受 `male` / `female` / `unspecified`，其他值返回 400
- [x] `birth_date_precision` 接受 `day` / `year`，默认 `day`
- [x] 修改成员支持部分更新，必填字段显式 null 返回 400
- [x] 迁移清空 `members` 并把 `reports.member_id` 置空，迁移说明标注仅限未上线阶段
- [x] App 表单必填校验、性别三选一、日期精度切换
- [x] HTTP 测试覆盖正常创建、缺字段、非法枚举、部分更新
- [x] RNTL 覆盖表单校验与提交

## Comments

**实现（2026-11-02，分支 `llm-analysis-spec`）**

- 迁移 `0013_member_sex_birth_date.sql`：先解除 `reports.member_id` 归属，再清空 `members`；新增 `sex` / `birth_date`（NOT NULL）与 `birth_date_precision`（默认 `day`）。报告与识别历史保留。
- 领域校验集中在 `internal/pipeline/members.go`；建成员与 PATCH 共用同一枚举 / 日期校验，存储边界再次镜像。
- OpenAPI / Go 镜像 / App 表单 / RNTL 全链路同步；`CONTEXT.md` 成员定义与 `ADR-0015` 已更新。
