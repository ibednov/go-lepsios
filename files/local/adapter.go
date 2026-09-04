package local

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/ibednov/go-lepsios/files"
)

type Adapter struct {
	basePath string
}

func New(basePath string) (*Adapter, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, files.WrapError(files.CodeInternal, "files.local.adapter.create-dir", err)
	}

	return &Adapter{basePath: basePath}, nil
}

func (s *Adapter) Create(ctx context.Context, in files.CreateInput) error {
	fullPath := filepath.Join(s.basePath, in.Path)

	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return files.WrapError(files.CodeInternal, "files.local.adapter.create-dir", err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return files.WrapError(files.CodeInternal, "files.local.adapter.create-file", err)
	}
	defer file.Close()

	_, err = io.Copy(file, in.Reader)
	if err != nil {
		os.Remove(fullPath)
		return files.WrapError(files.CodeInternal, "files.local.adapter.write-file", err)
	}

	return nil
}

func (s *Adapter) Get(ctx context.Context, in files.GetInput) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.basePath, in.Path)

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, files.WrapError(files.CodeNotFound, "files.local.adapter.open-file", err)
		}
		return nil, files.WrapError(files.CodeInternal, "files.local.adapter.open-file", err)
	}

	return file, nil
}

func (s *Adapter) Delete(ctx context.Context, in files.DeleteInput) error {
	fullPath := filepath.Join(s.basePath, in.Path)

	err := os.Remove(fullPath)
	if err != nil && !os.IsNotExist(err) {
		return files.WrapError(files.CodeInternal, "files.local.adapter.delete-file", err)
	}

	return nil
}

func (s *Adapter) PublicURL(path string) string {
	return ""
}
