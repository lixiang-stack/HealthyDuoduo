# 01: 成员模型扩展

**What to build:** 建成员时必须填写性别和出生日期；性别提供「男 / 女 / 不确定」三选一；只知道出生年份时允许只填年份并标记精度；修改成员时可改这些字段。产品未上线，迁移时清空现有成员并解除报告归属。

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] 建成员时缺性别或出生日期返回 400，错误信息指明缺哪个字段
- [ ] 性别接受 `male` / `female` / `unspecified`，其他值返回 400
- [ ] `birth_date_precision` 接受 `day` / `year`，默认 `day`
- [ ] 修改成员支持部分更新，必填字段显式 null 返回 400
- [ ] 迁移清空 `members` 并把 `reports.member_id` 置空，迁移说明标注仅限未上线阶段
- [ ] App 表单必填校验、性别三选一、日期精度切换
- [ ] HTTP 测试覆盖正常创建、缺字段、非法枚举、部分更新
- [ ] RNTL 覆盖表单校验与提交
