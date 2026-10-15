# 03: 识别端不变量：空 items ⇒ failed

**What to build:** 一张拍糊了或只拍到标题的检查单照片，不会被伪装成"识别成功但什么都没有的报告"静默归档——它一律成为 failed 留痕，家长由此知道要重拍。这是识别端本期唯一改动点。

**Blocked by:** 02（M0 契约冻结）。

**Status:** resolved

- [x] 识别端后处理规则保证：报告级别产出"成功且零指标项"不可能出现，一律判 failed 并给出失败原因
- [x] 数据服务入库校验镜像同规则（双保险，绕过识别端直灌也挡住）
- [x] 识别端 pytest 全绿；若存量 golden 出现"成功且空指标项"样本，按预期行为变更重基并逐条说明，其余 golden 逐字节不动
- [x] 全链路验证：空白样例图走 ingest 后 status=failed，图像与 OCR 留痕保留

## Comments

2026-10-15（agent 实现，从 main@49fd06a 开分支 `empty-items-failed-invariant`，票 02 已合并无需 stack）：

- 三层防线分工，规则语义全部来自票 02 已冻结工件（report.schema.json / openapi.yaml / fixture 词汇），零契约改动：
  1. **识别端**：`recognizer/recognizer/contract.py` `Report` 加 `model_validator`——空 items 且 status ≠ failed 拒绝构造，「成功且零指标项」连模型都建不出来；`run_postprocess` 的空产出 → failed 规则代码已存在（后处理早已保证），本票补的是结构性锁死。契约 schema 未加条件式（02 冻结纪律）。
  2. **Go pipeline（用例层）**：`internal/pipeline/reports.go` 新增 `applyFailInvariant`（空 items ⇒ 一律判 failed 归一化；failed 且空 items ⇒ 产出 `fail_reason="no metric items recognized"`，词条锚定 `schemas/testdata/http/report.summary.failed.json`）。Ingest 与 ReparseStored 两个写入口统一过此函数，`Store.WriteReport` 签名加 `failReason *string`（票 02 迁移注释「fail_reason 由 pipeline 产出」的落点）。fail_reason 随 upsert 全量写：非 failed 一律 NULL，reparse 把历史 failed 空报告算好时旧原因自动清除（测试 `TestReparseRecovers_FailReasonCleared`）。
  3. **Go storage（适配器层）**：`WriteReport` 镜像校验——直灌违规形状（status=success/partial 且零 items）返回错误不写库（真校验而非重复纠正；pipeline 已归一化，此层只为绕过路径兜底）。冒烟 `internal/storage/store_invariant_test.go`（HDD_TEST_DSN 门控 + 一次性 PG 容器，Schema 无需扩表：fail_reason 列在 0004 已就位）。
- **Golden 零重基**：`test_report_matches_golden` 全量通过，samples/ 46 金标无一「成功且零指标项」样本（重基条款未触发），全部逐字节未动。
- **共享栈采纳至 goose 4**（e2e 外环标准序列，snapshot 先行）：`snapshot -o before-03-empty-items-invariant`（主检出 data/e2e-snapshots/）→ `refresh --from` 本 worktree（hdd 批量 reparse 首调用经 storage.Open 0003→0004，`refresh ok (24 reparses)`）→ `verify`/`verify --golden` 全 ok。这也是 handoff 预告的「共享栈升 4 采纳时机」实际落点（票 03 成为第一个写 fail_reason 的代码，升级在此票收口）。
- **空白图验收实测**：PIL 生成纯白 800×1000 JPEG → `hdd ingest` → `sha256=f35644…: unknown - failed (0 items)`；DB 复核 reports 行 `status=failed, fail_reason='no metric items recognized'`、images 行在（seaweedfs 对象在上）、ocr_results 行留痕（188B JSON）。
- 栈收尾状态**有意不回滚**：goose 4 + fail_reason 一致性留给票 05（members 端点 + 本迁移强依赖）；如需回滚 `restore --latest` 需人工 yes（快照在升级前，回滚会同时退 schema 版本）。blank-sheet.jpeg 在 worktree `data/`（gitignored）。
- 环境：一次性容器冒烟用 `hdd-invariant-smoke-pg`:55433（复跑即 destroy 重起）；e2e refresh 需 `--from` 绝对路径（相对 `.` 会因 Popen 无斜杠走 PATH 而误报 ENOENT，工具小坑留给后续票顺手修）。
