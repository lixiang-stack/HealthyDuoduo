// Package storage:PG(PostgreSQL + goose 迁移)与 S3(SeaweedFS,minio-go)适配器。
// 依赖规则:本包是外部细节,只被 cmd/hdd 与 pipeline 通过接口消费(整洁架构)。
package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // 启用 database/sql 驱动 "pgx",供 goose 迁移使用
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationsDir 内嵌迁移目录名;go:embed 的模式与 goose.UpContext 的路径共用同一目录。
// //go:embed 指令只能写字面量,无法引用常量:更名目录时须同步改两处(embed 行 + 本常量)。
const migrationsDir = "migrations"

// Open 打开 PG 连接池并确保迁移到最新(goose 库模式,本地首次运行即建四表)。
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := Migrate(ctx, dsn); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// Migrate 用 goose 应用内嵌 SQL 迁移(idempotent,重复运行跳过)。
// goose 以 database/sql 驱动运行,与 pgx 池各自独立;pgx stdlib 注册名为 "pgx"。
func Migrate(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open postgres for migrate: %w", err)
	}
	defer db.Close()
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, db, migrationsDir); err != nil {
		return fmt.Errorf("goose migrate: %w", err)
	}
	return nil
}
