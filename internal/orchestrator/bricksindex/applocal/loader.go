package applocal

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

type AppBricksIndex struct {
	LocalBricks []AppBrick
}

type AppBrick struct {
	FullPath    *paths.Path
	ConfigFile  *paths.Path // brick_config.yaml file path
	ComposeFile *paths.Path // brick_compose.yaml file path, optional

	Brick bricksindex.Brick // the brick as defined in the brick_config.yaml file
}

func (b *AppBricksIndex) GetByID(id string) (*bricksindex.Brick, bool) {
	idx := slices.IndexFunc(b.LocalBricks, func(local AppBrick) bool {
		return local.Brick.ID == id
	})
	if idx == -1 {
		return nil, false
	}
	return &b.LocalBricks[idx].Brick, true
}

func (b *AppBricksIndex) GetComposePath(id string) (*paths.Path, bool) {
	idx := slices.IndexFunc(b.LocalBricks, func(local AppBrick) bool {
		return local.Brick.ID == id
	})
	if idx == -1 {
		return nil, false
	}
	return b.LocalBricks[idx].ComposeFile, b.LocalBricks[idx].ComposeFile != nil
}

func Load(dir *paths.Path) (localBrickIndex *AppBricksIndex, err error) {
	if dir == nil {
		return nil, errors.New("empty path provided for local bricks")
	}
	if !dir.Exist() {
		return nil, fmt.Errorf("local bricks folder not found %v", dir)
	}
	pathsList, err := dir.ReadDirRecursiveFiltered(func(file *paths.Path) bool {
		if file.Join("brick_config.yaml").NotExist() {
			// let's continue scanning, the model can be in a subfolder
			return true
		}
		return false
	}, paths.FilterDirectories())
	if err != nil {
		return nil, err
	}
	bricks := []AppBrick{}
	for _, path := range pathsList {
		brick, err := loadLocalAppBrick(path)
		if err != nil {
			slog.Warn("Cannot load local brick", "err", err, "path", path)
			continue
		}
		bricks = append(bricks, brick)
	}
	return &AppBricksIndex{LocalBricks: bricks}, nil
}

func loadLocalAppBrick(brickPath *paths.Path) (a AppBrick, err error) {
	brickConfigPath := brickPath.Join("brick_config.yaml")
	if brickConfigPath.NotExist() {
		return AppBrick{}, os.ErrNotExist
	}
	brickConfigContent, err := os.ReadFile(brickConfigPath.String())
	if err != nil {
		return AppBrick{}, err
	}
	customBrick := bricksindex.Brick{}
	if err := yaml.Unmarshal(brickConfigContent, &customBrick); err != nil {
		return AppBrick{}, err
	}

	var composeFile *paths.Path = nil
	brickComposeFile := brickPath.Join("brick_compose.yaml")
	if brickComposeFile.Exist() {
		composeFile = brickComposeFile
	}

	return AppBrick{
		FullPath:    brickPath,
		ConfigFile:  brickConfigPath,
		ComposeFile: composeFile,
		Brick:       customBrick,
	}, nil
}

// ListBricks returns all local bricks as []bricksindex.Brick.
func (b *AppBricksIndex) ListBricks() ([]bricksindex.Brick, bool) {
	if b == nil || len(b.LocalBricks) == 0 {
		return nil, false
	}
	bricks := make([]bricksindex.Brick, len(b.LocalBricks))
	for i, local := range b.LocalBricks {
		bricks[i] = local.Brick
	}
	return bricks, true
}

// GetBrickReadmeFromID returns the contents of README.md for the brick, if present.
func (b *AppBricksIndex) GetBrickReadmeFromID(id string) (string, error) {
	idx := slices.IndexFunc(b.LocalBricks, func(local AppBrick) bool {
		return local.Brick.ID == id
	})
	if idx == -1 {
		return "", fmt.Errorf("brick %s not found", id)
	}
	readmePath := b.LocalBricks[idx].FullPath.Join("README.md")
	if readmePath.NotExist() {
		return "", fmt.Errorf("README.md not found for brick %s", id)
	}
	content, err := os.ReadFile(readmePath.String())
	if err != nil {
		return "", fmt.Errorf("cannot read README.md for brick %s: %w", id, err)
	}
	return string(content), nil
}

func (b *AppBricksIndex) GetBrickApiDocPathFromID(id string) (string, error) {
	panic("API doc for local bricks is not supported yet")
}

func (b *AppBricksIndex) GetBrickCodeExamplesPathFromID(id string) (paths.PathList, error) {
	panic("Examples  for local bricks is not supported yet")
}
