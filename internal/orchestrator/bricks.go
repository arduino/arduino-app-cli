package orchestrator

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/assets"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
)

var ErrBrickNotFound = errors.New("brick not found")

type BrickListResult struct {
	Bricks []BrickListItem `json:"bricks"`
}
type BrickListItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Author      string `json:"author"`
	Description string `json:"description"`
	Icon        string `json:"icon"` // TODO: not implemented yet
	IsInstalled bool   `json:"installed"`
}

func BricksList() (BrickListResult, error) {
	collection, found := bricksIndex.GetCollection("arduino", "app-bricks")
	if !found {
		return BrickListResult{}, errors.New("collection not found")
	}

	release, found := collection.GetRelease(bricksVersion)
	if !found {
		return BrickListResult{}, errors.New("release not found")
	}

	res := BrickListResult{Bricks: make([]BrickListItem, len(release.Bricks))}
	for i, brick := range release.Bricks {
		res.Bricks[i] = BrickListItem{
			ID:          brick.ID,
			Name:        brick.Name,
			Author:      "Arduino", //TODO: for now we only support our bricks
			Description: brick.Description,
			Icon:        "", // TODO: not implemented yet
			IsInstalled: true,
		}
	}
	return res, nil
}

type BrickDetailsResult struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Author      string                   `json:"author"`
	Description string                   `json:"description"`
	Icon        string                   `json:"icon"` // TODO: not implemented yet
	IsInstalled bool                     `json:"installed"`
	Variables   map[string]BrickVariable `json:"variables,omitempty"`
	Readme      string                   `json:"readme"`
}

type BrickVariable struct {
	DefaultValue string `json:"default_value,omitempty"`
	Description  string `json:"description,omitempty"`
	Required     bool   `json:"required"`
}

func BricksDetails(id string) (BrickDetailsResult, error) {
	collection, found := bricksIndex.GetCollection("arduino", "app-bricks")
	if !found {
		return BrickDetailsResult{}, errors.New("collection not found")
	}

	release, found := collection.GetRelease(bricksVersion)
	if !found {
		return BrickDetailsResult{}, errors.New("release not found")
	}

	var brick *bricksindex.Brick
	for _, b := range release.Bricks {
		if b.ID == id {
			brick = b
		}
	}
	if brick == nil {
		return BrickDetailsResult{}, ErrBrickNotFound
	}

	variables := make(map[string]BrickVariable, len(brick.Variables))
	for k, v := range brick.Variables {
		variables[k] = BrickVariable{
			DefaultValue: v.DefaultValue,
			Description:  v.Description,
			Required:     v.DefaultValue == "",
		}
	}

	readme, err := assets.FS.ReadFile(filepath.Join("static", bricksVersion.String(), "docs", id, "README.md"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return BrickDetailsResult{}, err
	}

	return BrickDetailsResult{
		ID:          id,
		Name:        brick.Name,
		Author:      "Arduino", // TODO: for now we only support our bricks
		Description: brick.Description,
		Icon:        "",   // TODO: not implemented yet
		IsInstalled: true, // For now every Arduino brick are installed
		Variables:   variables,
		Readme:      string(readme),
	}, nil
}
