# Spec：多图上传识别（一次选多张，逐张归档）

Status: ready-for-agent

维护者 2026-10-24 需求拷问会定稿：多图 = 多份独立报告；整批同一成员；相册多选 + 拍照逐张追加 + 可移除，累计上限 9 张；客户端串行 fan-out，不新增批量接口；单张失败隔离、只重试传输失败项；上传中离开页面继续归档、不做续传。

**依据文档：** 统一语言 `CONTEXT.md`（图像 / 报告 / 成员）· `ADR-0010`（多图上传语义与传输）· App E2E `docs/agents/app-e2e.md` · 现有实现 `apps/mobile/src/app/upload.tsx`、`src/platform/imagePicker.ts`、`src/api/reports.ts`、`src/reports/status.ts`。

## 为什么做

现状 `pickImage` 只取 `assets[0]`，上传契约明确「单图,对应一张检查单」；家长一次有多张检查单时要重复「选图 → 上传 → 等待 → 再传」，交互成本高。需求是**一次选择多张、一次发起上传、逐张得到识别结果**，不是把多页合并成一份报告。

## 做什么

### 批次构建

- 相册：开启 `allowsMultipleSelection` + `selectionLimit`；一次最多 9 张。
- 拍照：一次一张，拍完追加到待上传列表，可继续拍。
- 再次选择：追加；列表逐张可移除；累计上限 9 张，超出部分丢弃并提示。
- 重复选到同一张图不去重：后端按 sha 幂等返回 `duplicate`，App 照常展示「此前已归档」。
- 整批归属当前选中成员；上传期间与完成后锁定成员，需「继续上传」才能切换；传错可在报告详情改归属。

### 上传执行

- App 串行调用现有 `POST /reports`，每张一个 multipart 请求；一张结束再发下一张。
- 单张超时 180s（后端识别 60s × 2 次重试的余量）；超时记为失败。
- 单张失败不阻断后续；失败的图只影响该行。
- 只重试**传输失败**（网络 / HTTP / 超时）的图；识别失败的报告已留痕，重试会命中 `duplicate`，不进入重试队列。

### 结果呈现

- 选中列表即结果列表：上传前显示缩略图 / 文件名 / 「移除」；上传中逐行「识别中… / 等待识别」；完成后逐行状态与动作。
- 顶部汇总：`共 N 张:…`（已归档 / 待核对 / 此前已归档 / 失败）。
- 行状态：`已归档`、`待核对`、`此前已归档`、`识别失败:<原因>`、`上传失败:<原因>`。
- 行动作：需核对 → 「立即核对」；否则 → 「查看报告」。
- 整批完成前不显示行动作与重试 / 继续上传，避免中途离开丢队列。
- 完成后：`重试失败的 M 张`（M 为传输失败数）、`去档案查看`、`继续上传`（清空列表与结果）。
- 单张结果文案沿用 `uploadResultText`；多张用批次汇总文案。

### 明确不做

- 多页合并为一份报告（需要改报告身份与跨页合并，独立领域模型变更）。
- 后端 `POST /reports/batch` / `images[]` 批量接口。
- 客户端批量核对队列、上传中取消 / 停止队列、断点续传 / 持久化队列。
- 客户端按文件内容去重。
- 后端、OpenAPI、识别服务、数据库改动。
- `CONTEXT.md` 改动：「批次」是交互单位，不是领域实体。

## 契约与接口

**无 OpenAPI / 数据库 / 后端改动。** 复用：

- `POST /reports`（multipart：`image` + `member_id`）：单张上传，同步识别；sha 幂等与 409 语义不变。
- `GET /reports/{report_id}`、`GET /images/{sha256}`、`PATCH /reports/{report_id}`：结果行「查看报告」与改归属。
- `uploadReport` 增加 180s `AbortSignal` 超时；非超时错误原样抛。

新增 App 内纯逻辑 `apps/mobile/src/reports/uploadBatch.ts`：

- `MAX_UPLOAD_IMAGES = 9`
- `appendPickedImages(current, picked)` → `{ images, discarded }`
- `uploadImagesSerially(images, indexes, upload, hooks)` → 串行、失败隔离
- `uploadBatchCounts(outcomes)` / `uploadBatchSummaryText(outcomes)`
- `uploadOutcomeLabel(outcome)`

## 怎么实施

1. **纯逻辑**：`uploadBatch.ts` + `uploadBatch.test.ts`（串行顺序、失败不中断、只跑给定下标、上限截断、分类 / 汇总 / 行文案）。
2. **平台**：`imagePicker.ts` 返回 `images[]`；相册多选 + `selectionLimit`；拍照单张；`imagePicker.test.ts` 覆盖多选映射 / 取消 / 拒绝。
3. **传输**：`api/reports.ts` 的 `uploadReport` 加 180s 超时；`reports.test.ts` 覆盖超时中止与非超时透传。
4. **页面**：`upload.tsx` 改为「images + outcomes + phase」状态；串行 runner；逐行渲染、汇总、重试失败、继续上传；相册 / 拍照入口。
5. **App E2E**：`glu_03.jpeg`（success、无低置信）+ `glu_01.png`（partial）加入回放；新增「一次选两张 → 2 行结果 → 时间线 2 份报告」；完成判据 15/15 → 16/16；同步 `docs/agents/app-e2e.md`。
6. **文档**：本 spec + `docs/adr/0010-multi-image-upload-independent-reports.md`。

## 验收

- `npm test`：新增 / 既有单测全绿；重点覆盖上限、串行、失败隔离、重试下标、汇总 / 行文案、超时。
- `npm run typecheck`、`npx expo lint` 全绿。
- `npm run e2e` 16/16 全绿；单图上传与 lifecycle 重传用例保持通过。
- 原生手工验收（Web E2E 盲区）：iOS / Android 相册多选、拍照逐张追加、逐张移除、9 张上限提示、上传中离开页面后时间线可见已归档报告。
- 已知边界：上传中离开页面批次状态丢失；9 张串行最坏等待较长；多页单张检查单不支持。
