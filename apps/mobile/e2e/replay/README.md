# E2E replay 替身图

`samples/` 中的掩码 OCR 语料条目没有原图（脱敏语料只保留 OCR / report golden），
不能按真实图片回放。本目录为这些样本提供 **E2E 专用替身图**：

- 文件名 = 样本 id（如 `lft_08.png`），每个文件字节唯一，避免 App E2E 的 sha256 全局去重。
- `e2e/compose.yml` 把本目录挂到识别服务的 `/e2e-replay`，并设置
  `RECOGNIZER_REPLAY_IMAGES=/e2e-replay`；`Replay` 命中替身图 sha 后返回
  `samples/expected/{ocr,report}/<id>.json`，不跑 OCR。
- 替身图只提供 sha，**不是报告图像**，不参与识别质量判断，也不替代 `samples/` 真实语料。
- 需要新增无原图样本时：在这里放 `<id>.png`，并把 `<id>` 加入
  `e2e/compose.yml` 的 `RECOGNIZER_REPLAY_SAMPLES`。
