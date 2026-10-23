# 04: 参考范围缺失文案改为中性事实

**What to build:** 把 `ReviewItemCard` 在 `ref_range=null` 时显示的 `参考范围未识别` 改为 `无参考范围`，并同步注释。`参考范围未识别` 断言的是“识别失败”，但 `cbc_01` 的中性细胞数 `4.19` 纸面本来就没有参考范围；`无参考范围` 只陈述数据事实，不推断是纸面未印还是 OCR/抽取漏掉。

**Blocked by:** None。

**Status:** resolved

## 验收清单

- [x] `apps/mobile/src/review/components.tsx` 的 `MISSING_REF_RANGE_LABEL` 改为 `无参考范围`。
- [x] 紧邻注释改为“只陈述数据事实，不推断是纸面未印还是抽取/OCR 漏掉”，不要保留“参考范围未识别”的旧说法。
- [x] 不改 `ref_range` 有值时的 `参考 ...` 展示、不改条带/flag/核对动作/编辑器。
- [x] 搜索并更新任何断言旧文案的测试或 E2E；当前仓库没有该字符串的测试断言，新增/复用 `cbc_04` 的血清淀粉样蛋白A（`ref_range=null`）用例做可见文案断言，若该用例尚未存在则手工核对详情页。
- [x] `npm test`、`npm run typecheck`、`npx expo lint` 全绿；不新增依赖。
- [x] 不改服务端/契约/数据库；不引入新的状态或字段。

## 实现提示

- 常量在 `apps/mobile/src/review/components.tsx:30-32`；展示位置在 `ReviewItemCard` 的 `item.ref_range ? '参考 ...' : MISSING_REF_RANGE_LABEL` 分支。
- 这是旧 03（原图定位）的跟进小修；旧 03 的 `LineZoom` / `shouldShowZoom` 不要动。
- 文案选择理由：`无参考范围` 是数据事实；`未识别` 会把“纸面没有”误报成“识别失败”，也会在家长侧制造不必要的重拍/排查暗示。

## Comments

**实现（2026-10-23，分支 `review-clarity-v2-followup`）**

- `MISSING_REF_RANGE_LABEL` 改为 `无参考范围`；注释改为“只陈述数据事实，不推断是纸面未印还是抽取/OCR 漏掉”。
- 旧文案断言检索：生产代码仅 `apps/mobile/src/review/components.tsx` 一处；测试/E2E 无旧文案断言。
- 复用 `cbc_04` 详情 E2E，断言 SAA（`ref_range=null`）卡片显示 `无参考范围`。
- 验证：`npm test` 103 通过；`npm run typecheck` 通过；`npx expo lint` 通过；App E2E 14 条通过。
- 未改服务端/契约/数据库/其他展示分支。
