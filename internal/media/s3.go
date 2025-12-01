package media

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

// Config represents the settings required to talk to S3 or an S3-compatible API.
type Config struct {
	Bucket         string
	Region         string
	Endpoint       string
	PublicURL      string
	KeyPrefix      string
	ForcePathStyle bool
}

// NewUploader wires an S3 client if the configuration is complete, otherwise a disabled uploader.
func NewUploader(ctx context.Context, cfg Config) (Uploader, error) {
	if cfg.Bucket == "" || cfg.Region == "" {
		return Disabled(), nil
	}

	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}

	if cfg.Endpoint != "" {
		endpoint := cfg.Endpoint
		loadOpts = append(loadOpts, config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
				if service == s3.ServiceID {
					return aws.Endpoint{
						URL:           endpoint,
						PartitionID:   "aws",
						SigningRegion: cfg.Region,
					}, nil
				}
				return aws.Endpoint{}, &aws.EndpointNotFoundError{}
			}),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("load aws sdk config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.UsePathStyle = cfg.ForcePathStyle
		}
	})

	keyPrefix := strings.Trim(cfg.KeyPrefix, "/")

	// Fallback so S3-compatible storage without PublicURL still works for reads.
	publicURL := strings.TrimSuffix(cfg.PublicURL, "/")
	if publicURL == "" && cfg.Endpoint != "" && cfg.ForcePathStyle {
		publicURL = fmt.Sprintf("%s/%s", strings.TrimSuffix(cfg.Endpoint, "/"), cfg.Bucket)
	}

	return &s3Uploader{
		client:  client,
		bucket:  cfg.Bucket,
		region:  cfg.Region,
		baseURL: publicURL,
		prefix:  keyPrefix,
	}, nil
}

type s3Uploader struct {
	client  *s3.Client
	bucket  string
	region  string
	baseURL string
	prefix  string
}

// Upload stores the incoming file in the configured bucket and returns a public URL.
func (u *s3Uploader) Upload(ctx context.Context, input UploadInput) (UploadResult, error) {
	if input.Body == nil {
		return UploadResult{}, errors.New("upload body is required")
	}

	key := u.buildKey(input.Filename)

	putInput := &s3.PutObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(key),
		Body:   input.Body,
	}
	if input.ContentType != "" {
		putInput.ContentType = aws.String(input.ContentType)
	}
	if input.Size > 0 {
		putInput.ContentLength = aws.Int64(input.Size)
	}

	if _, err := u.client.PutObject(ctx, putInput); err != nil {
		return UploadResult{}, fmt.Errorf("put object: %w", err)
	}

	return UploadResult{
		Key: key,
		URL: u.objectURL(key),
	}, nil
}

func (u *s3Uploader) buildKey(filename string) string {
	name := uuid.NewString()
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != "" {
		name += ext
	}

	if u.prefix == "" {
		return name
	}

	return path.Join(u.prefix, name)
}

func (u *s3Uploader) objectURL(key string) string {
	if u.baseURL != "" {
		return fmt.Sprintf("%s/%s", u.baseURL, key)
	}

	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", u.bucket, u.region, key)
}
