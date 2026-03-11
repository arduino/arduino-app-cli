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

type BricksIndex struct {
	Bricks []AppBrick
}

type AppBrick struct {
	FullPath *paths.Path

	composeFile  *paths.Path // brick_compose.yaml file path, optional
	readmeFile   *paths.Path // README.md file path, optional
	examplesPath *paths.Path // code examples folder path, optional
	docsAPIPath  *paths.Path // API docs file path, optional

	Brick bricksindex.Brick // the brick as defined in the brick_config.yaml file
}

func Load(dir *paths.Path) (index BricksIndex, err error) {
	if dir == nil {
		return BricksIndex{}, errors.New("empty path provided for local bricks")
	}
	if !dir.Exist() {
		return BricksIndex{}, fmt.Errorf("local bricks folder not found %v", dir)
	}
	pathsList, err := dir.ReadDirRecursiveFiltered(func(file *paths.Path) bool {
		if file.Join("brick_config.yaml").NotExist() {
			// let's continue scanning, the model can be in a subfolder
			return true
		}
		return false
	}, paths.FilterDirectories(), paths.FilterOutNames(".cache"))
	if err != nil {
		return BricksIndex{}, err
	}
	bricks := []AppBrick{}
	for _, path := range pathsList {
		brick, err := loadLocalAppBrick(path)
		if err != nil {
			slog.Warn("Cannot load local app brick", "err", err, "path", path)
			continue
		}
		bricks = append(bricks, brick)
	}
	return BricksIndex{Bricks: bricks}, nil
}

func (b BricksIndex) GetByID(id string) (*bricksindex.Brick, bool) {
	brick, ok := b.getBrick(id)
	if !ok {
		return nil, false
	}
	return &brick.Brick, true
}

func (b BricksIndex) getBrick(id string) (*AppBrick, bool) {
	idx := slices.IndexFunc(b.Bricks, func(local AppBrick) bool {
		return local.Brick.ID == id
	})
	if idx == -1 {
		return nil, false
	}
	return &b.Bricks[idx], true
}

func (b BricksIndex) ListBricks() ([]bricksindex.Brick, bool) {
	if len(b.Bricks) == 0 {
		return nil, false
	}
	bricks := make([]bricksindex.Brick, len(b.Bricks))
	for i, local := range b.Bricks {
		bricks[i] = local.Brick
	}
	return bricks, true
}

func (b BricksIndex) GetComposePath(id string) (*paths.Path, bool) {
	local, ok := b.getBrick(id)
	if !ok {
		return nil, false
	}

	return local.composeFile, local.composeFile != nil
}

func (b BricksIndex) GetApiDocPath(id string) (*paths.Path, error) {
	local, ok := b.getBrick(id)
	if !ok {
		return nil, fmt.Errorf("brick %s not found", id)
	}
	if local.docsAPIPath == nil || local.docsAPIPath.NotExist() {
		return nil, fmt.Errorf("API docs not found for brick %s", id)
	}
	return local.docsAPIPath, nil
}

func (b BricksIndex) GetReadme(id string) (string, error) {
	local, ok := b.getBrick(id)
	if !ok {
		return "", fmt.Errorf("brick %s not found", id)
	}
	if local.readmeFile.NotExist() {
		return "", fmt.Errorf("README.md not found for brick %s", id)
	}
	content, err := os.ReadFile(local.readmeFile.String())
	if err != nil {
		return "", fmt.Errorf("cannot read README.md for brick %s: %w", id, err)
	}
	return string(content), nil
}

func (b BricksIndex) GetExamplesPath(id string) (paths.PathList, error) {
	brick, ok := b.getBrick(id)
	if !ok {
		return nil, fmt.Errorf("brick %s not found", id)
	}
	if brick.examplesPath == nil || brick.examplesPath.NotExist() {
		return nil, nil
	}

	dirEntries, err := brick.examplesPath.ReadDir()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read examples directory %q: %w", brick.examplesPath, err)
	}
	return dirEntries, nil
}

func loadLocalAppBrick(brickPath *paths.Path) (a AppBrick, err error) {
	brickConfigPath := brickPath.Join("brick_config.yaml")
	if brickConfigPath.NotExist() {
		return AppBrick{}, fmt.Errorf("brick_config.yaml does not exist: %v", brickConfigPath)
	}
	brickConfigContent, err := os.ReadFile(brickConfigPath.String())
	if err != nil {
		return AppBrick{}, fmt.Errorf("cannot read brick_config.yaml: %w", err)
	}
	customBrick := bricksindex.Brick{}
	if err := yaml.Unmarshal(brickConfigContent, &customBrick); err != nil {
		return AppBrick{}, fmt.Errorf("cannot unmarshal brick_config.yaml: %w", err)
	}

	var composeFile *paths.Path = nil
	brickComposeFile := brickPath.Join("brick_compose.yaml")
	if brickComposeFile.Exist() {
		composeFile = brickComposeFile
	}

	return AppBrick{
		FullPath:     brickPath,
		composeFile:  composeFile,
		readmeFile:   brickPath.Join("README.md"),
		examplesPath: brickPath.Join("examples"),
		docsAPIPath:  brickPath.Join("docs/API.md"),
		Brick:        customBrick,
	}, nil
}
