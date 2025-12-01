package media

import (
	"context"
	"errors"
	"io"
)

// ErrUploaderDisabled indicates that uploads are not currently enabled.
var ErrUploaderDisabled = errors.New("media uploader disabled")

// UploadInput wraps the payload required for persisting a file.
type UploadInput struct {
	Filename    string
	ContentType string
	Body        io.Reader
	Size        int64
}

// UploadResult captures the canonical object key and its accessible URL.
type UploadResult struct {
	Key string
	URL string
}

// Uploader hides the backing implementation for storing files.
type Uploader interface {
	Upload(ctx context.Context, input UploadInput) (UploadResult, error)
}

type disabledUploader struct{}

func (disabledUploader) Upload(_ context.Context, _ UploadInput) (UploadResult, error) {
	return UploadResult{}, ErrUploaderDisabled
}

// Disabled returns an uploader that always signals disabled uploads.
func Disabled() Uploader {
	return disabledUploader{}
}
