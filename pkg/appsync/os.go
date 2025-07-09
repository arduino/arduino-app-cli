package appsync

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

type OsFSWriter struct {
	Base string
}

func (o OsFSWriter) MkDirAll(path string) error {
	return os.MkdirAll(filepath.Join(o.Base, path), 0755)
}

func (o OsFSWriter) WriteFile(path string, data io.ReadCloser) error {
	out, err := os.Create(filepath.Join(o.Base, path))
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, data)
	return err
}

func (o OsFSWriter) RmFile(path string) error {
	return os.Remove(filepath.Join(o.Base, path))
}

func (o OsFSWriter) Open(name string) (fs.File, error) {
	return os.Open(filepath.Join(o.Base, name))
}
