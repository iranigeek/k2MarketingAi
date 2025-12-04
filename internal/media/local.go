package media

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalUploader stores files on the local filesystem (typically /tmp) for short-lived processing.
type LocalUploader struct {
	BaseDir string
}

// NewLocalUploader constructs an uploader that writes to the provided directory.
// If baseDir is empty, os.TempDir() is used.
func NewLocalUploader(baseDir string) (*LocalUploader, error) {
	dir := baseDir
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create local media dir: %w", err)
	}
	return &LocalUploader{BaseDir: dir}, nil
}

// Upload writes the incoming content to a temp file and returns its absolute path.
func (l *LocalUploader) Upload(_ context.Context, input UploadInput) (UploadResult, error) {
	if input.Body == nil {
		return UploadResult{}, fmt.Errorf("upload body is required")
	}

	ext := filepath.Ext(input.Filename)
	if len(ext) > 10 {
		ext = ext[:10]
	}

	tmpFile, err := os.CreateTemp(l.BaseDir, "k2media-*"+ext)
	if err != nil {
		return UploadResult{}, fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, input.Body); err != nil {
		os.Remove(tmpFile.Name())
		return UploadResult{}, fmt.Errorf("write temp file: %w", err)
	}

	return UploadResult{
		Key: tmpFile.Name(),
		URL: "",
	}, nil
}
