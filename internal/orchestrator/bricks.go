package orchestrator

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
)

var ErrBrickNotFound = errors.New("brick not found")
var ErrCannotSave = errors.New("cannot save brick instance")

type BrickListResult struct {
	Bricks []BrickListItem `json:"bricks"`
}
type BrickListItem struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Author      string   `json:"author"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Status      string   `json:"status"`
	Models      []string `json:"models"`
}
type BrickCreateUpdateRequest struct {
	ID        string            `json:"-"`
	Model     *string           `json:"model"`
	Variables map[string]string `json:"variables,omitempty"`
}
type BrickVariable struct {
	DefaultValue string `json:"default_value,omitempty"`
	Description  string `json:"description,omitempty"`
	Required     bool   `json:"required"`
}
type AppReference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

type BrickDetailsResult struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Author      string                   `json:"author"`
	Description string                   `json:"description"`
	Category    string                   `json:"category"`
	Status      string                   `json:"status"`
	Variables   map[string]BrickVariable `json:"variables,omitempty"`
	Readme      string                   `json:"readme"`
	UsedByApps  []AppReference           `json:"used_by_apps"`
}

type BrickInstance struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Author    string            `json:"author"`
	Category  string            `json:"category"`
	Status    string            `json:"status"`
	Variables map[string]string `json:"variables,omitempty"`
	ModelID   string            `json:"model,omitempty"`
}

type AppBrickInstancesResult struct {
	BrickInstances []BrickInstance `json:"bricks"`
}

func BricksList(modelsIndex *modelsindex.ModelsIndex, bricksIndex *bricksindex.BricksIndex) (BrickListResult, error) {
	res := BrickListResult{Bricks: make([]BrickListItem, len(bricksIndex.Bricks))}
	for i, brick := range bricksIndex.Bricks {
		res.Bricks[i] = BrickListItem{
			ID:          brick.ID,
			Name:        brick.Name,
			Author:      "Arduino", // TODO: for now we only support our bricks
			Description: brick.Description,
			Category:    brick.Category,
			Status:      "installed",
			Models: f.Map(modelsIndex.GetModelsByBrick(brick.ID), func(m modelsindex.AIModel) string {
				return m.ID
			}),
		}
	}
	return res, nil
}

func AppBrickInstancesList(a *app.ArduinoApp, bricksIndex *bricksindex.BricksIndex) (AppBrickInstancesResult, error) {
	res := AppBrickInstancesResult{BrickInstances: make([]BrickInstance, len(a.Descriptor.Bricks))}
	for i, brickInstance := range a.Descriptor.Bricks {
		brick, found := bricksIndex.FindBrickByID(brickInstance.ID)
		if !found {
			return AppBrickInstancesResult{}, fmt.Errorf("brick not found with id %s", brickInstance.ID)
		}
		res.BrickInstances[i] = BrickInstance{
			ID:        brick.ID,
			Name:      brick.Name,
			Author:    "Arduino", // TODO: for now we only support our bricks
			Category:  brick.Category,
			Status:    "installed",
			ModelID:   brickInstance.Model,     // TODO: in case is not set by the user, should we return the default model?
			Variables: brickInstance.Variables, // TODO: do we want to show also the default value of not explicitly set variables?
		}
	}
	return res, nil
}

func AppBrickInstanceDetails(a *app.ArduinoApp, bricksIndex *bricksindex.BricksIndex, brickID string) (BrickInstance, error) {
	brick, found := bricksIndex.FindBrickByID(brickID)
	if !found {
		return BrickInstance{}, ErrBrickNotFound
	}
	// Check if the brick is already added in the app
	brickIndex := slices.IndexFunc(a.Descriptor.Bricks, func(b app.Brick) bool { return b.ID == brickID })
	if brickIndex == -1 {
		return BrickInstance{}, fmt.Errorf("brick %s not added in the app", brickID)
	}

	variables := make(map[string]string, len(brick.Variables))
	for _, v := range brick.Variables {
		variables[v.Name] = v.DefaultValue
	}
	// Add/Update the variables with the ones from the app descriptor
	maps.Copy(variables, a.Descriptor.Bricks[brickIndex].Variables)

	modelID := a.Descriptor.Bricks[brickIndex].Model
	if modelID == "" {
		modelID = brick.ModelName
	}

	return BrickInstance{
		ID:        brickID,
		Name:      brick.Name,
		Author:    "Arduino", // TODO: for now we only support our bricks
		Category:  brick.Category,
		Status:    "installed", // For now every Arduino brick are installed
		Variables: variables,
		ModelID:   modelID,
	}, nil
}

func BricksDetails(docsFS fs.FS, bricksIndex *bricksindex.BricksIndex, id string) (BrickDetailsResult, error) {
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

	// TODO: here would be cool to create a store abrstraction
	brickPath := filepath.Join(strings.Split(brick.ID, ":")...)
	docFile, err := docsFS.Open(path.Join(brickPath, "README.md"))
	if err != nil {
		return BrickDetailsResult{}, fmt.Errorf("cannot open docs for brick %s: %w", id, err)
	}
	defer docFile.Close()

	readme, err := io.ReadAll(docFile)
	if err != nil {
		return BrickDetailsResult{}, err
	}

	return BrickDetailsResult{
		ID:          id,
		Name:        brick.Name,
		Author:      "Arduino", // TODO: for now we only support our bricks
		Description: brick.Description,
		Category:    brick.Category,
		Status:      "installed", // For now every Arduino brick are installed
		Variables:   variables,
		Readme:      string(readme),
	}, nil
}

func BrickCreate(
	req BrickCreateUpdateRequest,
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
	appCurrent app.ArduinoApp,
) error {
	brick, present := bricksIndex.FindBrickByID(req.ID)
	if !present {
		return fmt.Errorf("brick not found with id %s", req.ID)
	}

	for name, reqValue := range req.Variables {
		value, exist := brick.GetVariable(name)
		if !exist {
			return errors.New("variable does not exist")
		}
		if value.DefaultValue == "" && reqValue == "" {
			return errors.New("variable default value cannot be empty")
		}
	}

	for _, brickVar := range brick.Variables {
		if brickVar.DefaultValue == "" {
			if _, exist := req.Variables[brickVar.Name]; !exist {
				return errors.New("variable does not exist")
			}
			return errors.New("variable default value cannot be empty")
		}
	}

	brickIndex := -1
	var brickInstance app.Brick

	for index, b := range appCurrent.Descriptor.Bricks {
		if b.ID == req.ID {
			brickIndex = index
			brickInstance = b
			break
		}
	}

	brickInstance.ID = req.ID

	if req.Model == nil {
		return fmt.Errorf("received empty model ")
	}
	models := modelsIndex.GetModelsByBrick(brickInstance.ID)
	idx := slices.IndexFunc(models, func(m modelsindex.AIModel) bool { return m.ID == *req.Model })
	if idx == -1 {
		return fmt.Errorf("model %s does not exsist", *req.Model)
	}

	brickInstance.Model = models[idx].ID
	brickInstance.Variables = req.Variables

	if brickIndex == -1 {

		appCurrent.Descriptor.Bricks = append(appCurrent.Descriptor.Bricks, brickInstance)

	} else {
		appCurrent.Descriptor.Bricks[brickIndex] = brickInstance

	}

	err := appCurrent.Save()
	if err != nil {
		return fmt.Errorf("cannot save brick instance with id %s", req.ID)
	}

	return nil
}

func BrickUpdate(
	req BrickCreateUpdateRequest,
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
	appCurrent app.ArduinoApp,
) error {
	index := slices.IndexFunc(appCurrent.Descriptor.Bricks, func(b app.Brick) bool { return b.ID == req.ID })
	if index == -1 {
		return fmt.Errorf("brick not found with id %s", req.ID)
	}
	brickID := appCurrent.Descriptor.Bricks[index].ID
	brickVariables := appCurrent.Descriptor.Bricks[index].Variables
	if len(brickVariables) == 0 {
		brickVariables = make(map[string]string)
	}
	brickModel := appCurrent.Descriptor.Bricks[index].Model

	if req.Model != nil && *req.Model != brickModel {
		models := modelsIndex.GetModelsByBrick(req.ID)
		idx := slices.IndexFunc(models, func(m modelsindex.AIModel) bool { return m.ID == *req.Model })
		if idx == -1 {
			return fmt.Errorf("model %s does not exsist", *req.Model)
		}
		brickModel = *req.Model
	}
	brick, present := bricksIndex.FindBrickByID(brickID)
	if !present {
		return fmt.Errorf("brick not found with id %s", brickID)
	}
	for name, updateValue := range req.Variables {
		value, exist := brick.GetVariable(name)
		if !exist {
			return errors.New("variable does not exist")
		}
		if value.DefaultValue == "" && updateValue == "" {
			return errors.New("variable default value cannot be empty")
		}
		updated := false
		for _, v := range brickVariables {
			if v == name {
				brickVariables[name] = updateValue
				updated = true
				break
			}
		}

		if !updated {
			brickVariables[name] = updateValue
		}
	}

	appCurrent.Descriptor.Bricks[index].Model = brickModel
	appCurrent.Descriptor.Bricks[index].Variables = brickVariables

	err := appCurrent.Save()
	if err != nil {
		return fmt.Errorf("cannot save brick instance with id %s", req.ID)
	}
	return nil

}

func BrickDelete(
	bricksIndex *bricksindex.BricksIndex,
	id string,
	appCurrent *app.ArduinoApp,
) error {
	_, present := bricksIndex.FindBrickByID(id)
	if !present {
		return ErrBrickNotFound
	}

	appCurrent.Descriptor.Bricks = slices.DeleteFunc(appCurrent.Descriptor.Bricks, func(b app.Brick) bool {
		return b.ID == id
	})

	if err := appCurrent.Save(); err != nil {
		return ErrCannotSave
	}
	return nil
}
