package main

// openDeps:按环境配置装配真实适配器(pgx 池 + minio-go 客户端 + HTTP 识别服务)。
// 环境变量全部带本地 deploy/docker-compose.yml 的缺省值,缺省即 compose 端口约定。

import (
	"context"
	"os"

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
}

func envConfig() config {
	return config{
		databaseURL:   fromEnv("HDD_DATABASE_URL", "postgres://healthyduoduo:healthyduoduo@localhost:5432/healthyduoduo?sslmode=disable"),
		recognizerURL: fromEnv("HDD_RECOGNIZER_URL", "http://localhost:8000"),
		s3Endpoint:    fromEnv("HDD_S3_ENDPOINT", "http://localhost:8333"),
		s3AccessKey:   fromEnv("HDD_S3_ACCESS_KEY", ""),
		s3SecretKey:   fromEnv("HDD_S3_SECRET_KEY", ""),
		s3Bucket:      fromEnv("HDD_S3_BUCKET", "raw"),
	}
}

func fromEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// openDeps 打开 PG(含 goose 内嵌迁移)与 S3;defer 返回的 cleanup 释放连接池。
func openDeps(c config) (*deps, func(), error) {
	pool, err := storage.Open(context.Background(), c.databaseURL)
	if err != nil {
		return nil, nil, err
	}
	obj, err := storage.NewS3ObjectStore(c.s3Endpoint, c.s3AccessKey, c.s3SecretKey, c.s3Bucket, false)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	d := &deps{
		store: storage.NewStore(pool),
		rec:   recognizer.New(c.recognizerURL),
		obj:   obj,
	}
	return d, func() { pool.Close() }, nil
}
