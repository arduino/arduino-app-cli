package detector

import (
	"log/slog"
	"os"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"
)

type AppLocalBrick struct {
	FullPath    *paths.Path
	ConfigFile  *paths.Path
	ComposeFile *paths.Path

	Brick bricksindex.Brick
}

func detectLocalAppBricks(appPath *paths.Path) (appBricks []AppLocalBrick, err error) {
	pathsList, err := appPath.ReadDirRecursiveFiltered(func(file *paths.Path) bool {
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
	return bricks, nil
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
