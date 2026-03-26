package bricksindex

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
)

func loadFromFolder(dir *paths.Path) []Brick {
	bricks := []Brick{}
	pathsList, err := dir.ReadDirRecursiveFiltered(func(file *paths.Path) bool {
		if file.Join("brick_config.yaml").NotExist() {
			// let's continue scanning, the model can be in a subfolder
			return true
		}
		return false
	}, paths.FilterDirectories())
	if err != nil {
		slog.Warn("error reading app bricks folder, skipping loading bricks", "err", err, "path", dir)
		return bricks
	}

	for _, path := range pathsList {
		brick, err := load(path)
		if err != nil {
			slog.Warn("Cannot load local app brick", "err", err, "path", path)
			continue
		}
		bricks = append(bricks, brick)
	}
	return bricks
}

func load(brickPath *paths.Path) (a Brick, err error) {
	brickConfigPath := brickPath.Join("brick_config.yaml")
	if brickConfigPath.NotExist() {
		return Brick{}, fmt.Errorf("brick_config.yaml does not exist: %v", brickConfigPath)
	}
	brickConfigContent, err := os.ReadFile(brickConfigPath.String())
	if err != nil {
		return Brick{}, fmt.Errorf("cannot read brick_config.yaml: %w", err)
	}
	brick := Brick{}
	if err := yaml.Unmarshal(brickConfigContent, &brick); err != nil {
		return Brick{}, fmt.Errorf("cannot unmarshal brick_config.yaml: %w", err)
	}
	brick.Source = "custom" // TODO: find a better name

	var composeFile *paths.Path = nil
	brickComposeFile := brickPath.Join("brick_compose.yaml")
	if brickComposeFile.Exist() {
		composeFile = brickComposeFile
	}
	brick.Source = "custom" // TODO: find a better name ?
	brick.composeFile = composeFile
	brick.readmeFile = brickPath.Join("README.md")
	brick.examplesPath = brickPath.Join("examples")
	brick.docsAPIPath = brickPath.Join("docs/API.md")
	return brick, nil
}
