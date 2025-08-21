package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arduino/go-paths-helper"
)

type StaticStore struct {
	baseDir     string
	composePath string
	docsPath    string
	assetsPath  *paths.Path
}

func NewStaticStore(baseDir string) *StaticStore {
	return &StaticStore{
		baseDir:     baseDir,
		composePath: filepath.Join(baseDir, "compose"),
		docsPath:    filepath.Join(baseDir, "docs"),
		assetsPath:  paths.New(baseDir),
	}
}

func (s *StaticStore) SaveComposeFolderTo(dst string) error {
	composeFS := s.GetComposeFolder()
	dstPath := paths.New(dst)
	_ = dstPath.RemoveAll()
	if err := composeFS.CopyDirTo(dstPath); err != nil {
		return fmt.Errorf("failed to copy assets directory: %w", err)
	}
	return nil
}

func (s *StaticStore) GetAssetsFolder() *paths.Path {
	return s.assetsPath
}

func (s *StaticStore) GetComposeFolder() *paths.Path {
	return paths.New(s.composePath)
}

func (s *StaticStore) GetBrickReadmeFromID(brickID string) (string, error) {
	namespace, brickName, ok := strings.Cut(brickID, ":")
	if !ok {
		return "", errors.New("invalid ID")
	}
	content, err := os.ReadFile(filepath.Join(s.docsPath, namespace, brickName, "README.md"))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (s *StaticStore) GetBrickComposeFilePathFromID(brickID string) (*paths.Path, error) {
	namespace, brickName, ok := strings.Cut(brickID, ":")
	if !ok {
		return nil, errors.New("invalid ID")
	}
	return paths.New(s.composePath, namespace, brickName, "brick_compose.yaml"), nil
}
