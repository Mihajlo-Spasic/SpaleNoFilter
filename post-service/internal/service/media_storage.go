package service

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/instagram-clone/post-service/internal/config"
	"github.com/instagram-clone/post-service/internal/models"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MediaStorage interface {
	Upload(file multipart.File, header *multipart.FileHeader, userID uint64) (*models.PostMedia, error)
	Delete(mediaKey string) error
}

type minioStorage struct {
	client *minio.Client
	bucket string
	cfg    *config.Config
}

func NewMinioStorage(cfg *config.Config) (MediaStorage, error) {
	client, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: cfg.MinioUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio init: %w", err)
	}

	ctx := context.Background()
	exists, err := client.BucketExists(ctx, cfg.MinioBucket)
	if err != nil {
		return nil, fmt.Errorf("minio bucket check: %w", err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, cfg.MinioBucket, minio.MakeBucketOptions{}); err != nil {
			return nil, fmt.Errorf("minio make bucket: %w", err)
		}
		// Set public read policy
		policy := fmt.Sprintf(`{
			"Version":"2012-10-17",
			"Statement":[{
				"Effect":"Allow",
				"Principal":{"AWS":["*"]},
				"Action":["s3:GetObject"],
				"Resource":["arn:aws:s3:::%s/*"]
			}]
		}`, cfg.MinioBucket)
		client.SetBucketPolicy(ctx, cfg.MinioBucket, policy)
	}

	return &minioStorage{client: client, bucket: cfg.MinioBucket, cfg: cfg}, nil
}

func (s *minioStorage) Upload(file multipart.File, header *multipart.FileHeader, userID uint64) (*models.PostMedia, error) {
	const maxBytes = 50 << 20 // 50 MB

	if header.Size > maxBytes {
		return nil, fmt.Errorf("file exceeds 50 MB limit (got %.1f MB)", float64(header.Size)/(1<<20))
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	mediaType, err := detectMediaType(ext)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("users/%d/%s/%s%s",
		userID,
		time.Now().Format("2006/01"),
		uuid.New().String(),
		ext,
	)

	contentType := "image/jpeg"
	if mediaType == models.MediaTypeVideo {
		contentType = "video/mp4"
	}

	ctx := context.Background()
	_, err = s.client.PutObject(ctx, s.bucket, key, file, header.Size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return nil, fmt.Errorf("upload to minio: %w", err)
	}

	mediaURL := fmt.Sprintf("%s/%s/%s", s.cfg.MinioPublicURL, s.bucket, key)

	return &models.PostMedia{
		MediaKey:  key,
		MediaURL:  mediaURL,
		MediaType: mediaType,
		SizeBytes: header.Size,
	}, nil
}

func (s *minioStorage) Delete(mediaKey string) error {
	return s.client.RemoveObject(context.Background(), s.bucket, mediaKey, minio.RemoveObjectOptions{})
}

func detectMediaType(ext string) (models.MediaType, error) {
	imageExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	videoExts := map[string]bool{".mp4": true, ".mov": true, ".avi": true, ".mkv": true, ".webm": true}

	if imageExts[ext] {
		return models.MediaTypeImage, nil
	}
	if videoExts[ext] {
		return models.MediaTypeVideo, nil
	}
	return "", fmt.Errorf("unsupported file type: %s (allowed: jpg, jpeg, png, gif, webp, mp4, mov, avi, mkv, webm)", ext)
}
