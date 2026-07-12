package minio

import (
	"context"
	"fmt"
	miniogo "github.com/minio/minio-go/v7"
	"io"
	"strings"
	"time"
)

type AvatarStore struct {
	client             *miniogo.Client
	bucket, publicBase string
}

func NewAvatarStore(c *miniogo.Client, bucket, publicBase string) *AvatarStore {
	return &AvatarStore{client: c, bucket: bucket, publicBase: strings.TrimRight(publicBase, "/")}
}
func (s *AvatarStore) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	_ = contentType
	u, err := s.client.PresignedPutObject(ctx, s.bucket, key, ttl)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
func (s *AvatarStore) Put(ctx context.Context, key string, r io.Reader, size int64, ct string) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, miniogo.PutObjectOptions{ContentType: ct})
	return err
}
func (s *AvatarStore) Get(ctx context.Context, key string) (io.ReadCloser, int64, string, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, miniogo.GetObjectOptions{})
	if err != nil {
		return nil, 0, "", err
	}
	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, 0, "", err
	}
	return obj, info.Size, info.ContentType, nil
}
func (s *AvatarStore) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, miniogo.RemoveObjectOptions{})
}
func (s *AvatarStore) PublicURL(key string) string {
	if s.publicBase != "" {
		return s.publicBase + "/" + key
	}
	scheme := "http"
	if s.client.IsOnline() { /* endpoint is already configured */
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, s.client.EndpointURL().Host, s.bucket, key)
}
