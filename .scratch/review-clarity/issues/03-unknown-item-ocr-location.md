# 03: 未知指标项缺少 OCR 原图定位（中性细胞数 4.19）

**What to build:** 让每个指标项都能打开 OCR 行的原图定位，而不是只有低置信项或编辑态才显示 `LineZoom`。用 `samples/expected/report/cbc_01.json` 的「中性细胞数 4.19」（unknown、高置信、`ocr_line_numbers=[169,170,171]`）验证：详情页能直接打开 169–171 行对应的原图区域，看到「NEUT 中性细胞数 4.19」原文，理解它为什么是 unknown。若用户数据中的对应项 `ocr_lines` 为空，则定位行号/行链接数据链路并修复或另立数据票。

**Blocked by:** None（可立即开始；unknown 成因解释由 01/02 决定）。

**Status:** resolved

## 验收清单

- [x] 在 cbc_01 详情页复现：`中性细胞数 4.19` 是 `flag=unknown`、`low_confidence=false`、`ocr_lines` 有 3 行；当前不显示定位的根因是 `ReviewItemCard` 的 `showZoom = (low_confidence || editing) && ocr_lines.length > 0`。
- [x] 每个有 `ocr_lines` 的指标项都有可见入口打开 OCR 原图定位；高置信 unknown 项无需先点「修正」；低置信项保持默认展开/高亮。
- [x] 无 `ocr_lines` 的项不崩、不显示空定位；如数据缺失，记录识别端行号 → 详情 `ocr_lines` 的链路位置，修复或另立数据票，不静默隐藏。
- [x] cbc_01 手工验证：打开定位能看到 169–171 行区域，与「未知 / 4.19 / 原文」同屏对照；若 `ref_range=null`，卡片上能看出「未印参考范围」这一事实（文案可由 01/02 定）。
- [x] 单测覆盖定位可见性判定（低置信 / 高置信 / 无 `ocr_lines` / 编辑态）；`npm run typecheck`、`npm test`、App E2E 回归。
- [x] 不改 flag 计算与核对动作语义；不改 unknown 是否进队列（那是 02）。

## 实现提示

- `ReviewItemCard` 在 `apps/mobile/src/review/components.tsx`：`showZoom` 只看 `item.low_confidence || editing`；`item.ocr_lines` 已由详情 API 提供。
- 可复用 `ReportSectionCard` 的「点按展开原图对照」模式：给每个指标项一个「查看原图定位」开关，高置信默认收起，低置信默认展开。
- 纯逻辑可抽 `shouldShowZoom(item, expanded)` 并单测；E2E 现有 seed 没有 cbc_01，若不便加样张则用单测 + 手工 cbc_01，并说明 E2E 覆盖边界。

## Comments

**实现完成（2026-10-21，分支 member-overview）**

- `shouldShowZoom(item, {open, editing})` 纯逻辑 + 4 组单测（低置信默认展开/高置信默认收起/无 ocr_lines/编辑态）。
- `ReviewItemCard`：每个有 `ocr_lines` 的项都有「查看原图定位/收起原图定位」入口；高置信项无需先点修正；低置信默认展开；无行链接不渲染空视口。
- `ref_range=null` 显示中性事实「参考范围未识别」（不写「未印」，避免把抽取缺陷说成纸面事实）。
- 手工核对 cbc_01（member=aaa 报告 139fde0e0b0c）：详情 API `ocr_lines` 169–171 行框裁切出 `NEUT 中性细胞数 4.19`，无参考范围；与「未知 / 4.19 / 原文」同屏。
- 数据链路：`postprocess._cells` 记录行下标 → 契约 `ocr_line_numbers` → `report_items.ocr_line_numbers` → `toAPIItem` 下标映射行框；`member=aaa` 188/188 项 `ocr_lines` 非空，本次未触发缺行数据票。
- 未在真机/Web 手工点击 cbc_01；同一 `ReviewItemCard` 的定位开关由 App E2E 的 cbc_04 用例覆盖（高置信收起/展开 + 低置信默认展开）。
- 测试：`npm test` 97/97；`npm run typecheck`；`npx expo lint`；App E2E 12/12（新增「高置信未知项无需修改即可展开原图定位」；含低置信默认展开与高置信展开/收起）。
- 未改 flag 计算、核对动作语义、unknown 是否进队列；未新增依赖。
