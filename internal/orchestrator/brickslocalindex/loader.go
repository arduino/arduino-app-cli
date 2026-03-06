package brickslocalindex

import (
	"log/slog"
	"os"
	"slices"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

type BricksIndex struct {
	LocalBricks []AppLocalBrick
}

type AppLocalBrick struct {
	FullPath    *paths.Path
	ConfigFile  *paths.Path // brick_config.yaml file path
	ComposeFile *paths.Path // brick_compose.yaml file path, optional

	Brick bricksindex.Brick // the brick as defined in the brick_config.yaml file
}

func Load(appPath *paths.Path) (localBrickIndex *BricksIndex, err error) {
	bricksFolder := appPath.Join("bricks")
	if !bricksFolder.Exist() {
		slog.Warn("bricks filder not founs in the app", "app_path", appPath)
		return &BricksIndex{LocalBricks: []AppLocalBrick{}}, nil
	}
	pathsList, err := bricksFolder.ReadDirRecursiveFiltered(func(file *paths.Path) bool {
		if file.Join("brick_config.yaml").NotExist() {
			// let's continue scanning, the model can be in a subfolder
			return true
		}
		return false
	}, paths.FilterDirectories())
	if err != nil {
		return nil, err
	}
	bricks := []AppLocalBrick{}
	for _, path := range pathsList {
		brick, err := loadLocalAppBrick(path)
		if err != nil {
			slog.Warn("Cannot load local brick", "err", err, "path", path)
			continue
		}
		bricks = append(bricks, brick)
	}
	return &BricksIndex{LocalBricks: bricks}, nil
}

func loadLocalAppBrick(brickPath *paths.Path) (a AppLocalBrick, err error) {
	brickConfigPath := brickPath.Join("brick_config.yaml")
	if brickConfigPath.NotExist() {
		return AppLocalBrick{}, os.ErrNotExist
	}
	brickConfigContent, err := os.ReadFile(brickConfigPath.String())
	if err != nil {
		return AppLocalBrick{}, err
	}
	customBrick := bricksindex.Brick{}
	if err := yaml.Unmarshal(brickConfigContent, &customBrick); err != nil {
		return AppLocalBrick{}, err
	}

	var composeFile *paths.Path = nil
	brickComposeFile := brickPath.Join("brick_compose.yaml")
	if brickComposeFile.Exist() {
		composeFile = brickComposeFile
	}

	return AppLocalBrick{
		FullPath:    brickPath,
		ConfigFile:  brickConfigPath,
		ComposeFile: composeFile,
		Brick:       customBrick,
	}, nil
}

func (b *BricksIndex) FindBrickByID(id string) (*AppLocalBrick, bool) {
	idx := slices.IndexFunc(b.LocalBricks, func(local AppLocalBrick) bool {
		return local.Brick.ID == id
	})
	if idx == -1 {
		return nil, false
	}
	return &b.LocalBricks[idx], true
}
