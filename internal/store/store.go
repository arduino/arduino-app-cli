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
	baseDir          string
	composePath      string
	docsPath         string
	assetsPath       *paths.Path
	apiDocsPath      string
	codeExamplesPath string
}

func NewStaticStore(baseDir string) *StaticStore {
	return &StaticStore{
		baseDir:          baseDir,
		composePath:      filepath.Join(baseDir, "compose"),
		docsPath:         filepath.Join(baseDir, "docs"),
		apiDocsPath:      filepath.Join(baseDir, "api-docs"),
		codeExamplesPath: filepath.Join(baseDir, "examples"),
		assetsPath:       paths.New(baseDir),
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
	namespace, brickName, err := parseBrickID(brickID)
	if err != nil {
		return "", err
	}
	content, err := os.ReadFile(filepath.Join(s.docsPath, namespace, brickName, "README.md"))
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (s *StaticStore) GetBrickComposeFilePathFromID(brickID string) (*paths.Path, error) {
	namespace, brickName, err := parseBrickID(brickID)
	if err != nil {
		return nil, err
	}
	return paths.New(s.composePath, namespace, brickName, "brick_compose.yaml"), nil
}

func (s *StaticStore) GetBrickApiDocPathFromID(brickID string) (string, error) {
	namespace, brickName, err := parseBrickID(brickID)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.apiDocsPath, namespace, "app_bricks", brickName, "API.md"), nil
}

func (s *StaticStore) GetBrickCodeExamplesPathFromID(brickID string) (paths.PathList, error) {
	namespace, brickName, err := parseBrickID(brickID)
	if err != nil {
		return nil, err
	}
	targetDir := paths.New(s.codeExamplesPath, namespace, brickName)
	dirEntries, err := targetDir.ReadDir()
	if err != nil {
		return nil, fmt.Errorf("cannot read examples directory %q: %w", targetDir, err)
	}
	return dirEntries, nil
}

func parseBrickID(brickID string) (namespace, name string, err error) {
	namespace, brickName, ok := strings.Cut(brickID, ":")
	if !ok {
		return "", "", errors.New("invalid ID")
	}
	return namespace, brickName, nil
}
