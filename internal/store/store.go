package store

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"unsafe"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/assets"
)

type StaticStore struct {
	fs            *embed.FS
	runnerVersion string
	composePath   string
	docsPath      string
	assetsPath    string
}

func NewStaticStore(runnerVersion string) *StaticStore {
	return &StaticStore{
		fs:            &assets.FS,
		runnerVersion: runnerVersion,
		composePath:   "static/" + runnerVersion + "/compose",
		docsPath:      "static/" + runnerVersion + "/docs",
		assetsPath:    "static/" + runnerVersion,
	}
}

func (s *StaticStore) SaveComposeFolderTo(dst string) error {
	composeFS, err := s.GetComposeFolder()
	if err != nil {
		return fmt.Errorf("failed to get compose folder FS: %w", err)
	}
	if err := os.CopyFS(dst, composeFS); err != nil {
		return fmt.Errorf("failed to copy assets directory: %w", err)
	}
	return nil
}

func (s *StaticStore) GetAssetsFolder() (fs.FS, error) {
	return fs.Sub(assets.FS, s.assetsPath)
}

func (s *StaticStore) GetComposeFolder() (fs.FS, error) {
	return fs.Sub(assets.FS, s.composePath)
}

func (s *StaticStore) GetBrickReadmeFromID(brickID string) (string, error) {
	namespace, brickName, ok := strings.Cut(brickID, ":")
	if !ok {
		return "", errors.New("invalid ID")
	}
	content, err := s.fs.ReadFile(s.docsPath + "/" + namespace + "/" + brickName + "/README.md")
	if err != nil {
		return "", err
	}
	// In our case this is safe.
	return unsafe.String(unsafe.SliceData(content), len(content)), nil
}

func (s *StaticStore) GetBricksListFile() (io.ReadCloser, error) {
	return s.fs.Open(s.assetsPath + "/bricks-list.yaml")
}

func (s *StaticStore) GetModelsListFile() (io.ReadCloser, error) {
	return s.fs.Open(s.assetsPath + "/models-list.yaml")
}
