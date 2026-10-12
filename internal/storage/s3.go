package storage

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3ObjectStore 为 SeaweedFS S3 网关(bucket raw,键=图像内容 sha256;NF-01 本地不出机)。
type S3ObjectStore struct {
	client *minio.Client
	bucket string
}

// NewS3ObjectStore: endpoint 如 http://localhost:8333。
// scheme 由 endpoint 本身决定(http:// → 非 TLS,https:// → TLS;裸 host 形式不支持)。
func NewS3ObjectStore(endpoint, accessKey, secretKey, bucket string) (*S3ObjectStore, error) {
	secure, endpoint, err := splitEndpoint(endpoint)
	if err != nil {
		return nil, err
	}
	c, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, fmt.Errorf("minio client %s: %w", endpoint, err)
	}
	return &S3ObjectStore{client: c, bucket: bucket}, nil
}

// splitEndpoint 拆出 host:port 与 TLS 标记;仅接受 http(s):// 前缀,裸 host 拒绝
// (配置带 scheme 同时也表达了 TLS 意图,不必也无法用第二个参数各说一半)。
func splitEndpoint(endpoint string) (bool, string, error) {
	switch {
	case strings.HasPrefix(endpoint, "http://"):
		return false, strings.TrimPrefix(endpoint, "http://"), nil
	case strings.HasPrefix(endpoint, "https://"):
		return true, strings.TrimPrefix(endpoint, "https://"), nil
	default:
		return false, "", fmt.Errorf("endpoint %q: expect http:// or https:// scheme", endpoint)
	}
}

// EnsureBucket 桶不存在则创建(bucket 生命周期属启动装配;fail-fast)。
// SeaweedFS 对重复建桶返回 BucketAlreadyOwnedByYou,视为幂等跳过。
func (s *S3ObjectStore) EnsureBucket(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("bucket exists %s: %w", s.bucket, err)
	}
	if ok {
		return nil
	}
	if err = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		if code := minio.ToErrorResponse(err).Code; code != "BucketAlreadyOwnedByYou" {
			return fmt.Errorf("make bucket %s: %w", s.bucket, err)
		}
	}
	return nil
}

// Put 幂等倒入对象(同键覆盖,S3 语义,内容按原样字节保存)。
// bucket 由启动期 EnsureBucket 保证(职责分离:生命周期属启动装配,数据面只放对象)。
func (s *S3ObjectStore) Put(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, s.bucket, key,
		bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

// 编译期接口校验(pipeline.ObjectStore 的实现)。
var _ interface {
	Put(context.Context, string, []byte) error
} = (*S3ObjectStore)(nil)
