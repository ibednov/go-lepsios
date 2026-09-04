package s3minio

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/ibednov/go-lepsios/files"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type Adapter struct {
	client    *minio.Client
	bucket    string
	publicURL string
}

func New(endpoint, accessKey, secretKey, bucket, region string, useSSL bool, publicURL string) (*Adapter, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		return nil, mapMinioErr(err, "files.s3-minio.adapter.create-client")
	}

	s := &Adapter{
		client:    client,
		bucket:    bucket,
		publicURL: publicURL,
	}

	exists, err := s.client.BucketExists(context.Background(), bucket)
	if err != nil {
		return nil, mapMinioErr(err, "files.s3-minio.adapter.check-bucket")
	}
	if !exists {
		if err := s.client.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{Region: region}); err != nil {
			return nil, mapMinioErr(err, "files.s3-minio.adapter.create-bucket")
		}
	}

	return s, nil
}

func (s *Adapter) Create(ctx context.Context, in files.CreateInput) error {
	_, err := s.client.PutObject(ctx, s.bucket, in.Path, in.Reader, -1, minio.PutObjectOptions{})
	if err != nil {
		return mapMinioErr(err, "files.s3-minio.adapter.upload-object")
	}
	return nil
}

func (s *Adapter) Get(ctx context.Context, in files.GetInput) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, in.Path, minio.GetObjectOptions{})
	if err != nil {
		return nil, mapMinioErr(err, "files.s3-minio.adapter.get-object")
	}

	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, mapMinioErr(err, "files.s3-minio.adapter.stat-object")
	}

	return obj, nil
}

func (s *Adapter) Delete(ctx context.Context, in files.DeleteInput) error {
	if err := s.client.RemoveObject(ctx, s.bucket, in.Path, minio.RemoveObjectOptions{}); err != nil {
		return mapMinioErr(err, "files.s3-minio.adapter.delete-object")
	}
	return nil
}

func (s *Adapter) PublicURL(path string) string {
	if s.publicURL == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s", trimSlashRight(s.publicURL), s.bucket, path)
}

func trimSlashRight(value string) string {
	for len(value) > 0 && value[len(value)-1] == '/' {
		value = value[:len(value)-1]
	}
	return value
}

func mapMinioErr(err error, source string) error {
	resp := minio.ToErrorResponse(err)

	if resp.Code == "AccessDenied" || resp.StatusCode == 403 {
		return files.WrapError(files.CodeAccessDenied, source, err)
	}

	if resp.Code == "NoSuchKey" || resp.Code == "NoSuchObject" || resp.StatusCode == 404 {
		return files.WrapError(files.CodeNotFound, source, err)
	}

	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		return files.WrapError(files.CodeUnavailable, source, err)
	}

	return files.WrapError(files.CodeInternal, source, err)
}
