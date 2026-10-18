// 图像端点(GET /api/v1/images/{sha256};契约冻结于 schemas/openapi.yaml,viz-and-client 票 07)。
// 后端代理对象存储、局域网直出;对象键即图像内容 sha256(对象存在性即图像存在性:
// 内容寻址,不查 DB——同图重传幂等覆盖,桶内惰性孤儿对象照常可回)。
package main

import (
	"net/http"

	"healthyduoduo/internal/pipeline"
)

// handleGetImage GET /api/v1/images/{sha256}:直出图像字节流。
// Content-Type 由内容嗅探(stdlib DetectContentType;上传不落 content-type,
// 非图像对象回落 application/octet-stream)。非 sha256 形态无法是任何对象,与不存在同样 404。
func handleGetImage(obj pipeline.ObjectStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sha := r.PathValue("sha256")
		if pipeline.ValidateSha256(sha) != nil {
			writeErrorJSON(w, http.StatusNotFound, "image not found")
			return
		}
		data, err := obj.Get(r.Context(), sha)
		if err != nil {
			writeErrorJSON(w, http.StatusInternalServerError, "read image failed")
			return
		}
		if data == nil {
			writeErrorJSON(w, http.StatusNotFound, "image not found")
			return
		}
		w.Header().Set("Content-Type", http.DetectContentType(data))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}
}
