# 0001 · OCR 进程内调用，HTTP 服务化推迟至 P4

> **状态：已废弃**——被 [0002 · 双语架构](./0002-python-recognizer-go-data-service.md) 取代：维护者更熟 Go 且长线即服务化，识别服务自 P1 起就以 Docker + HTTP 交付。

最初的架构草案将 OCR 画作独立服务、由数据处理服务跨服务调用。决定：P1 交付 OCR 核心模块（`run_ocr`）+ CLI + Docker 镜像；P2 数据管道在同一 Python 进程内 import 调用 `run_ocr`；OCR 与数据处理的 HTTP 服务化一并在 P4 落地，届时 HTTP 只是套在 `run_ocr` 外的薄适配层。

理由：进程内调用让 CLI 与测试无前置条件（不起服务、不探活、无端口/超时问题）；「模型只加载一次」的收益用 pytest session 级 fixture 即可获得；单仓单环境下无隔离成本。否掉的替代方案「P1 即起 HTTP 服务」会让 CLI 单测不再自足、CI 多一层服务起停编排。
