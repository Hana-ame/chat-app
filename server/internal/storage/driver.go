package storage

import "io"

type PutResult struct {
	ID      string
	ETag    string
	Path    string
	DupPath string
}

type StorageDriver interface {
	Put(path, contentType string, body io.Reader) (*PutResult, error)
	Get(path string) (body io.ReadCloser, contentLength int64, contentType string, err error)
	Delete(path string) error
	Head(path string) (contentLength int64, contentType string, err error)
}
