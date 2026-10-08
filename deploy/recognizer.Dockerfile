# 识别服务镜像。
# P0:依赖 fastapi/uvicorn/pydantic(仅 /healthz 占位);
# P1 起追加 rapidocr + onnxruntime(模型随 wheel 打包、离线可用,NF-02)。
FROM python:3.13-slim
COPY --from=ghcr.io/astral-sh/uv:0.9 /uv /uvx /usr/local/bin/

WORKDIR /app
COPY recognizer/ ./
RUN uv sync --frozen --no-dev

ENV PATH="/app/.venv/bin:$PATH"
EXPOSE 8000
CMD ["uvicorn", "recognizer.api:app", "--host", "0.0.0.0", "--port", "8000"]
