# Pipeline E2E(数据链路端到端)

App 层 UI 验收不在这里：屏幕逻辑由 `apps/mobile/src/test/` 的 RNTL 测试覆盖，真机 smoke 见 `apps/mobile/ANDROID.md`。

共享 deploy 栈(本机 compose,项目名 `deploy`:postgres 5432 / recognizer 8000 / seaweedfs 8333,默认账号见 `cmd/hdd/config.go`)上的端到端自证回路。宿主机无 psql:SQL 一律走
`docker exec deploy-postgres-1 psql -U healthyduoduo -d healthyduoduo ...`。

## 按改动类型选回路

| 改动 | 先跑内环(inner loop,快、不动库) | 再做全链路收口(outer loop,动共享栈) |
| --- | --- | --- |
| recognizer 词典/规则 | pytest + golden 重生成 + score(见下) | `refresh` → `verify` → `verify --golden` |
| Go CLI/检索 | `go test ./...` | `refresh` → `verify`(golden 不动) |
| schemas/迁移 | `go test ./...` | `refresh`(goose 自动升版)→ trend/show 冒烟 |

内环(识别侧,在 `<检出>/recognizer` 下):
`uv run pytest`;`uv run python -m recognizer.golden` 重生成后人工核对 raw_text;
`uv run python tools/local_golden.py check / score` 对比改动前后指标(worktree 下带
`HDD_DATA_DIR=<主检出>/data`)。golden 未变绿或 score 退化时不进全链路。

## 全链路标准序列(工具自文档:`recognizer/tools/e2e.py` 的 docstring)

在检出根执行 `uv run --project recognizer python tools/e2e.py <cmd>`,顺次:

1. `snapshot -o <标名>` — 动库前的锚点。完成标准:打印快照路径且字节数 > 0。
2. `refresh --from <被测检出根>` — 该检出的代码成为 served recognizer(go build →
   compose 重建 → 全量 reparse,内置 healthz 等待与点名重试)。完成标准:`refresh ok (N reparses)`。
3. `verify --item 血小板 --from <被测检出根>` — 不变量:单位止于注册规范形/族外原文、
   items / sections 的 ocr_lines 行链接覆盖率与 join/box 一致性(经 hdd serve
   `/api/v1/reports/{id}`;歧义度与几何裕度作度量告警)、叙述型报告(有 sections)零
   items 且 status 与正文段落一致、trend 质量门只滤有据点、delta 只在同单位间。完成
   标准:全 `ok`(warn=源单印刷串列,合法保留原文,如 RDW-CV 的 109/L;ocr-lines
   裕度 >1 的孤立项为斜拍/竖排版式的良性常客,点名供抽查)。
4. `verify --golden --from <被测检出根>` — DB 指标项与可选报告段落 vs
   `<检出>/samples/expected/report` 逐字段一致(items/sections 为红、日期/整单状态偏离为
   warn——ingest `--date` 补录属合法偏离;非影像报告 sections 缺省)。
5. `seed` / `clean` — 种子 ingest 幂等;clean 只删台账登记行且需人工 `yes`。
6. `restore --latest` — 回滚快照(pg_restore --clean,人工 `yes`);机制自证:`restore-check`(临时库)。

## 护栏

- 动共享库之前先 `snapshot`;clean/restore 永远人工确认,不静默重放。
- `samples/` 图与既有真实数据 sha 同源:seed 多为 cached,`clean` 台账与真实行重合——
  把台账当作唯一删除依据。
- recognizer 同时只 serve 一个检出的代码;换检出验证前先 `refresh` 切换。
- main 不开发:worktree 实现、PR 合入,push 由用户决定。

## 已知坑(查环境查不到的)

- `up -d --force-recreate` 后立刻批量 reparse 会撞 uvicorn keep-alive 竞态(随机 EOF);
  `refresh` 已内置等待与重试。绕过工具手工批量时,同样先等 /healthz 再发。
- 词典与单位归一表（`unit_conversions.yaml` / `ocr_forms.yaml`）语义随被测检出:`--from` 已处理;自己写脚本时注意 sys.path 别
  用错检出的 recognizer 包。
- e2e 快照/台账放 `<主检出>/data/`(gitignored);工具自动解析主检出,不受 worktree 影响。
- golden 基准跟随 `--from` 检出取(main 与 feature 分支的 golden 可能不同)。
