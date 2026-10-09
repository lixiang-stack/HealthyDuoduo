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

// NewS3ObjectStore: endpoint 如 http://localhost:8333(minio-go 需拆出 scheme→secure);凭证可空。
func NewS3ObjectStore(endpoint, accessKey, secretKey, bucket string, secure bool) (*S3ObjectStore, error) {
	secure, endpoint, err := splitEndpoint(endpoint, secure)
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

// splitEndpoint 兼容 http(s)://host:port 写法;裸 host 形式沿用显式 secure 参数。
func splitEndpoint(endpoint string, secure bool) (bool, string, error) {
	for _, prefix := range []string{"http://", "https://"} {
		if strings.HasPrefix(endpoint, prefix) {
			return prefix == "https://", strings.TrimPrefix(endpoint, prefix), nil
		}
	}
	return secure, endpoint, nil
}

// Put 幂等倒入对象(同键覆盖,S3 语义,内容按原样字节保存)。
func (s *S3ObjectStore) Put(ctx context.Context, key string, data []byte) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("bucket exists %s: %w", s.bucket, err)
	}
	if !ok {
		if err = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			// SeaweedFS 对重复建桶返回 BucketAlreadyOwnedByYou,视为幂等跳过
			if code := minio.ToErrorResponse(err).Code; code != "BucketAlreadyOwnedByYou" {
				return fmt.Errorf("make bucket %s: %w", s.bucket, err)
			}
		}
	}
	_, err = s.client.PutObject(ctx, s.bucket, key,
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
