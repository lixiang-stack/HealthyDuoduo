package main

// openDeps:按环境配置装配真实适配器(pgx 池 + minio-go 客户端 + HTTP 识别服务)。
// 环境变量全部带本地 deploy/docker-compose.yml 的缺省值,缺省即 compose 端口约定。

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"healthyduoduo/internal/analysis"
	"healthyduoduo/internal/llm"
	"healthyduoduo/internal/pipeline"
	"healthyduoduo/internal/recognizer"
	"healthyduoduo/internal/storage"
)

type config struct {
	databaseURL   string
	recognizerURL string
	s3Endpoint    string
	s3AccessKey   string
	s3SecretKey   string
	s3Bucket      string
	flagPolicy    string

	llmAPIKey       string
	llmBaseURL      string
	llmModel        string
	llmTimeout      time.Duration
	llmInputTokens  int
	llmOutputTokens int
	analysisLimit   int
}

func envConfig() config {
	return config{
		databaseURL:   fromEnv("HDD_DATABASE_URL", "postgres://healthyduoduo:healthyduoduo@localhost:5432/healthyduoduo?sslmode=disable"),
		recognizerURL: fromEnv("HDD_RECOGNIZER_URL", "http://localhost:8000"),
		s3Endpoint:    fromEnv("HDD_S3_ENDPOINT", "http://localhost:8333"),
		s3AccessKey:   fromEnv("HDD_S3_ACCESS_KEY", ""),
		s3SecretKey:   fromEnv("HDD_S3_SECRET_KEY", ""),
		s3Bucket:      fromEnv("HDD_S3_BUCKET", "raw"),
		flagPolicy:    fromEnv("HDD_FLAG_POLICY", "recognizer/recognizer/flag_policy.yaml"),

		llmAPIKey:       fromEnv("HDD_LLM_API_KEY", ""),
		llmBaseURL:      fromEnv("HDD_LLM_BASE_URL", "https://api.deepseek.com"),
		llmModel:        fromEnv("HDD_LLM_MODEL", "deepseek-chat"),
		llmTimeout:      time.Duration(fromEnvInt("HDD_LLM_TIMEOUT_SECONDS", 30)) * time.Second,
		llmInputTokens:  fromEnvInt("HDD_LLM_INPUT_TOKENS", 8000),
		llmOutputTokens: fromEnvInt("HDD_LLM_OUTPUT_TOKENS", 1500),
		analysisLimit:   fromEnvInt("HDD_ANALYSIS_DAILY_LIMIT", analysisDailyLimitDefault),
	}
}

func fromEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func fromEnvInt(key string, def int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return def
	}
	return value
}

// llmClientConfig 环境配置 → LLM 客户端配置;单独成函数便于测试 env → client 的映射。
func llmClientConfig(c config) llm.Config {
	return llm.Config{
		BaseURL:         c.llmBaseURL,
		APIKey:          c.llmAPIKey,
		Model:           c.llmModel,
		Timeout:         c.llmTimeout,
		MaxOutputTokens: c.llmOutputTokens,
	}
}

// newExplainer 只有配置了 API key 才创建解释层编排器;未配置时保持 nil,
// 分析只输出事实层与框架层,不发起任何云端请求。
func newExplainer(c config, quota analysis.QuotaStore, events analysis.ObservabilityStore) *analysis.Explainer {
	if c.llmAPIKey == "" {
		return nil
	}
	return &analysis.Explainer{
		Client:           llm.New(llmClientConfig(c)),
		Matcher:          analysis.NoEntries{},
		Quota:            quota,
		Events:           events,
		DailyLimit:       c.analysisLimit,
		InputTokenLimit:  c.llmInputTokens,
		OutputTokenLimit: c.llmOutputTokens,
	}
}

// openDeps 打开 PG(含 goose 内嵌迁移)与 S3;defer 返回的 cleanup 释放连接池。
func openDeps(c config) (*deps, func(), error) {
	ctx := context.Background()
	pool, err := storage.Open(ctx, c.databaseURL)
	if err != nil {
		return nil, nil, err
	}
	obj, err := storage.NewS3ObjectStore(c.s3Endpoint, c.s3AccessKey, c.s3SecretKey, c.s3Bucket)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	if err := obj.EnsureBucket(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("ensure bucket %s: %w", c.s3Bucket, err)
	}
	policy, err := pipeline.LoadFlagPolicy(c.flagPolicy)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("load flag policy: %w", err)
	}
	st := storage.NewStore(pool)
	st.SetFlagPolicy(policy)
	rec := recognizer.New(c.recognizerURL)
	d := &deps{
		store:         st,
		members:       st,
		reports:       st,
		trends:        st,
		panels:        st,
		growth:        st,
		analyses:      st,
		quota:         st,
		observability: st,
		analysisLimit: c.analysisLimit,
		backfill:      st,
		classifier:    rec,
		rec:           rec,
		obj:           obj,
	}
	d.explainer = newExplainer(c, st, st)
	return d, func() { pool.Close() }, nil
}
