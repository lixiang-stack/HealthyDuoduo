# 识别服务镜像。
# P1:rapidocr + onnxruntime(onnxruntime 显式依赖;三个默认模型随 rapidocr wheel
# 打包进 site-packages/rapidocr/models/,构建后离线可用,NF-02)。
# P3+(ADR-0004):wired/lineless 表结构模型构建期预下载进镜像(运行时离线可用,NF-01)。
# uv sync --frozen 按 uv.lock 精确复现依赖;--no-dev 不带测试工具;
# 引擎参数随 recognizer/rapidocr.yaml 进镜像,调参改该文件后重新 build 即生效。
FROM python:3.12-slim
COPY --from=ghcr.io/astral-sh/uv:0.9 /uv /uvx /usr/local/bin/

# opencv-python 运行库(slim 无 X11/GL 依赖,rapidocr 要求 opencv_python 而非 headless 变体):
# cv2.so 实测引用 libGL.so.1 / libglib-2.0.so.0(libxcb1 由 libglx0→libx11 链自动带入)。
RUN apt-get update \
    && apt-get install -y --no-install-recommends libgl1 libglib2.0-0 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY recognizer/ ./
RUN uv sync --frozen --no-dev

ENV PATH="/app/.venv/bin:$PATH"
# 预下载表结构子模型(构建期联网一次),使运行时可离线(裸机/容器均不外发原图、不联网)
RUN python -c "from wired_table_rec.main import WiredTableInput, WiredTableRecognition; \
from lineless_table_rec.main import LinelessTableInput, LinelessTableRecognition; \
WiredTableRecognition(WiredTableInput()); LinelessTableRecognition(LinelessTableInput())"

EXPOSE 8000
CMD ["uvicorn", "recognizer.api:app", "--host", "0.0.0.0", "--port", "8000"]
