# 05: 存量回填项目组

**Spec:** ../spec.md（§6.4、§7.4、§10.2）

**What to build:** 旧报告通过识别服务按当前指标名重新分类项目组，只更新项目组列，不重写指标项、不丢核对状态 / 修正对；命令可重复执行，支持 dry-run。回填后旧报告分类正确，可在按项目视图复验（03 完成后）。

**Blocked by:** 01（识别端输出项目组与医学依据）、02（报告项目组纵切）

**Status:** resolved

- [x] `hdd backfill-panels` 遍历报告，调用 `/classify`，按 item id 更新项目组与报告级项目组。
- [x] 已核对 / 已修正的报告回填后 `review_status`、修正对、value / unit / ref_range 不变。
- [x] 重复执行幂等；`--dry-run` 不写库，只打印拟更新结果。
- [x] 识别服务不可用 / 单份失败时给出明确错误与成功 / 跳过 / 失败计数；不静默丢报告。
- [ ] 回填后通过 API / 数据库验证旧报告分类正确；项目数 / 异常数与 API 一致；03 完成后在项目视图复验。
- [x] Go 迁移 / 存储 / 回填测试全绿；`go test ./...`。

## Comments

**实现（2026-10-27，分支 `backfill-panels`）**

- `pipeline`：新增 `BackfillStore` / `PanelClassifier` 窄接口与 `BackfillPanels` 用例。按报告扫描 → 调 `/classify` → 比较现有 item panel / 报告 panels → 仅在有差异时写库；`--dry-run` 只计算。`failed` 报告不进项目视图，跳过并计数（`panels` 保持 `[]`，符合 spec §7.3 的存量回退口径）。单份 4xx / 校验失败继续；连接失败 / `/classify` 404/405 / 5xx 视为识别服务不可用，立即停止并提示部署顺序。
- `storage`：`AllReportsForBackfill`（日期降序、dateless 沉底、sha 升序；`--limit` 按报告数）与 `UpdateReportPanels`（单事务只写 `report_items.panel` + `reports.panels`；item id 必须属于该报告，否则回滚）。
- `recognizer`：Go 客户端新增 `POST /classify`；请求携带 `report_type` + 当前 `name/unit`，响应 `panels` / `item_panels`。
- `cmd/hdd`：新增 `hdd backfill-panels [--limit N] [--dry-run]`；逐份输出 `updated / would update / skipped / failed`，末尾打印成功 / 跳过 / 失败计数；不可用时提示 `recognizer -> migrations -> backfill-panels -> app`。
- 校验：除长度/空 code 外，拒绝报告级 `panels` 与 item panel 集合不一致或重复；排序仍以识别端为准（Go 不维护优先级表）。

**验证**

- `go test ./...` 全绿（新增 pipeline / recognizer / cmd 单测；含 dry-run、幂等、单份失败继续、不可用中止、坏响应拒绝、存储更新失败）。
- 真库 `HDD_TEST_DSN`（一次性 PG 17）跑 `TestBackfillStoreSmoke`：排序 / limit、只写 panel 列、review_status / correction_pairs / value / unit / ref_range / status 不变、跨报告 item id 拒绝且回滚、重复执行幂等。
- 真实样本链路（本地识别服务 + 一次性 PG 17）：`hdd ingest samples/lft_02.jpg` 后把 `reports.panels` 置 `{}`、17 个 `report_items.panel` 置 `other`（模拟存量），`--dry-run` 输出 `panels=[liver kidney uric_acid] items=17/17` 且不写库；实跑后 DB 为肝功 12 / 肾功 4 / 尿酸 1，与 `samples/expected/report/lft_02.json` 逐项一致；已修正项的 `review_status`、修正对与 value / unit / ref_range 不变；再跑全部 `skipped (up to date)`。
- 同库 `hdd serve` 读 API：`GET /api/v1/reports` 摘要与 `GET /api/v1/reports/{sha}` 详情均返回 `panels=[liver,kidney,uric_acid]`，详情 17 项 `panel` 与 golden 一致。
- 旧识别服务（无 `/classify`）返回 404 时命令立即失败并打印部署顺序提示；单份 422 仍继续。
- `python3 scripts/check-docs.py`：86 个文件 OK。

**未做 / 待后续**

- 项目列表 / 项目详情的项目数、异常数复验依赖票 03，待 03 完成后补做。
- 真库 storage 测试需每次独立一次性 PG（本测试已自带种子清理，可在同一容器重复运行）。
- 医学复核仍按票 08 待人工。
