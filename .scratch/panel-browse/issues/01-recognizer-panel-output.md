# 01: 识别端输出项目组与医学依据

**Spec:** ../spec.md（§5、§6、§10.1）

**What to build:** 识别服务在产出报告类别的同时，为每个指标项产出项目组（`panel`），并把报告级 `panels` 作为指标项项目组的排序聚合。家长侧还不展示；这一票让后续数据、API、App 有可信的项目组来源。词典分组同时带上可追溯的医学依据注释，供临床复核。

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] 每条 golden 报告都有 `panels`；每个指标项都有 `panel`；除 `other` 外不跨报告类别。
- [x] `panels` 按命中项数降序、同数按固定优先级排序；`panels[0]` 是命中项数最多的项目组。
- [x] 样本回归符合 spec §6.5：lft_02 为肝功 / 肾功 / 尿酸；lft_06 为肝功 / 血脂；tft_01 为甲状腺；ua_05 为尿常规；us_01 为影像；coag_01 为凝血；fibrosis_01 / bloodtype_01 / unknown_01 为其他。
- [x] 尿常规的葡萄糖 / 胆红素归尿常规；生化葡萄糖归血糖；影像不进其他；失败报告不进项目列表。
- [x] `/classify` 对混合报告、歧义项、空 items、未知 report_type、父子越界返回正确答案 / 明确拒绝。
- [x] 9 个词典文件都有 `# panel 依据` 块或 `# 依据:` 注释；混合词典 block 覆盖其所有 `panel:` 值；注释不参与解析。
- [x] `cd recognizer && pytest` 全绿；golden diff 人工核对；契约测试锁定枚举与父子约束。

## Comments

**实现（2026-10-27，分支 `panel-tags-impl`）**

- `contract.py`：新增 `Panel` 16 code 枚举、`PANEL_ORDER`、`PANEL_LABELS`、父子允许集合；`ReportItem.panel` / `Report.panels` 必填，`Report` 校验父子约束与聚合排序。
- `postprocess.py`：词典加载解析 `category.panel` / `item.panel`（缺省 `other`）；报告级 `panels` 统一走 `report_panels`；新增 `classify_items`。
- `api.py`：新增内部 `POST /classify`；未知 report_type / 父子越界返回 422。
- 9 个词典：混合词典（chemistry / endocrine / immunology）顶部 `# panel 依据` 块 + 逐项 `panel`；单一词典在 `category.panel` 上方 `# 依据:`；未重排条目。
- 契约 / golden：`schemas/report.schema.json` 增加 `panel` / `panels`；49 份 golden 重生成并人工核对，项目组集合与 spec §6.5 一致（lft_04 额外出现血糖、lft_05 额外出现血脂，均来自真实混合报告）。
- 验证：`cd recognizer && uv run pytest -q`（327 passed / 28 skipped）、`python3 scripts/check-docs.py`、`go test ./internal/contract/...` 全绿。
- 未做：Go 数据服务 / HTTP 契约镜像（票 02）；医学复核仍按票 08 待人工。
