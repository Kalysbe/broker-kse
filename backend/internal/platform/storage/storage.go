package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"statebroker/sbis/internal/platform/id"
)

type Store interface {
	Put(ctx context.Context, contentType string, r io.Reader) (key string, size int64, err error)
	Get(ctx context.Context, key string) (io.ReadCloser, error)
}

type Filesystem struct {
	Root string
}

func (f Filesystem) Put(ctx context.Context, contentType string, r io.Reader) (string, int64, error) {
	_ = contentType
	key := id.New().String()
	dir := filepath.Join(f.Root, key[:2])
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", 0, err
	}
	path := filepath.Join(dir, key)
	out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o640)
	if err != nil {
		return "", 0, err
	}
	defer out.Close()
	n, err := io.Copy(out, r)
	return key, n, err
}

func (f Filesystem) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(f.Root, key[:2], key))
}
