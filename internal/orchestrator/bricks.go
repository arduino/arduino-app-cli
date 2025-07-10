package orchestrator

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/assets"
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
	Status      string `json:"status"`
}

func BricksList() (BrickListResult, error) {
	res := BrickListResult{Bricks: make([]BrickListItem, len(bricksIndex.Bricks))}
	for i, brick := range bricksIndex.Bricks {
		res.Bricks[i] = BrickListItem{
			ID:          brick.ID,
			Name:        brick.Name,
			Author:      "Arduino", //TODO: for now we only support our bricks
			Description: brick.Description,
			Icon:        "", // TODO: not implemented yet
			Status:      "installed",
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
	Status      string                   `json:"status"`
	Variables   map[string]BrickVariable `json:"variables,omitempty"`
	Readme      string                   `json:"readme"`
	UsedByApps  []AppReference           `json:"used_by_apps"`
}

type AppReference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

type BrickVariable struct {
	DefaultValue string `json:"default_value,omitempty"`
	Description  string `json:"description,omitempty"`
	Required     bool   `json:"required"`
}

func BricksDetails(id string) (BrickDetailsResult, error) {
	brick, found := bricksIndex.FindBrickByID(id)
	if !found {
		return BrickDetailsResult{}, ErrBrickNotFound
	}

	variables := make(map[string]BrickVariable, len(brick.Variables))
	for _, v := range brick.Variables {
		variables[v.Name] = BrickVariable{
			DefaultValue: v.DefaultValue,
			Description:  v.Description,
			Required:     v.IsRequired(),
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
		Icon:        "",          // TODO: not implemented yet
		Status:      "installed", // For now every Arduino brick are installed
		Variables:   variables,
		Readme:      string(readme),
	}, nil
}
