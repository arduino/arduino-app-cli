package bricks

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"go.bug.st/f"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/modelsindex"
	"github.com/arduino/arduino-app-cli/internal/store"
)

var (
	ErrBrickNotFound   = errors.New("brick not found")
	ErrCannotSaveBrick = errors.New("cannot save brick instance")
)

type Service struct {
	modelsIndex *modelsindex.ModelsIndex
	bricksIndex *bricksindex.BricksIndex
	staticStore *store.StaticStore
}

func NewService(
	modelsIndex *modelsindex.ModelsIndex,
	bricksIndex *bricksindex.BricksIndex,
	staticStore *store.StaticStore,
) *Service {
	return &Service{
		modelsIndex: modelsIndex,
		bricksIndex: bricksIndex,
		staticStore: staticStore,
	}
}

func (s *Service) List() (BrickListResult, error) {
	res := BrickListResult{Bricks: make([]BrickListItem, len(s.bricksIndex.Bricks))}
	for i, brick := range s.bricksIndex.Bricks {
		res.Bricks[i] = BrickListItem{
			ID:          brick.ID,
			Name:        brick.Name,
			Author:      "Arduino", // TODO: for now we only support our bricks
			Description: brick.Description,
			Category:    brick.Category,
			Status:      "installed",
			Models: f.Map(s.modelsIndex.GetModelsByBrick(brick.ID), func(m modelsindex.AIModel) string {
				return m.ID
			}),
		}
	}
	return res, nil
}

func (s *Service) AppBrickInstancesList(a *app.ArduinoApp) (AppBrickInstancesResult, error) {
	res := AppBrickInstancesResult{BrickInstances: make([]BrickInstance, len(a.Descriptor.Bricks))}
	for i, brickInstance := range a.Descriptor.Bricks {
		brick, found := s.bricksIndex.FindBrickByID(brickInstance.ID)
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

func (s *Service) AppBrickInstanceDetails(a *app.ArduinoApp, brickID string) (BrickInstance, error) {
	brick, found := s.bricksIndex.FindBrickByID(brickID)
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

func (s *Service) BricksDetails(id string) (BrickDetailsResult, error) {
	brick, found := s.bricksIndex.FindBrickByID(id)
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

	readme, err := s.staticStore.GetBrickReadmeFromID(brick.ID)
	if err != nil {
		return BrickDetailsResult{}, fmt.Errorf("cannot open docs for brick %s: %w", id, err)
	}

	return BrickDetailsResult{
		ID:          id,
		Name:        brick.Name,
		Author:      "Arduino", // TODO: for now we only support our bricks
		Description: brick.Description,
		Category:    brick.Category,
		Status:      "installed", // For now every Arduino brick are installed
		Variables:   variables,
		Readme:      readme,
	}, nil
}

type BrickCreateUpdateRequest struct {
	ID        string            `json:"-"`
	Model     *string           `json:"model"`
	Variables map[string]string `json:"variables,omitempty"`
}

func (s *Service) BrickCreate(
	req BrickCreateUpdateRequest,
	appCurrent app.ArduinoApp,
) error {
	brick, present := s.bricksIndex.FindBrickByID(req.ID)
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

	if req.Model != nil {
		models := s.modelsIndex.GetModelsByBrick(brickInstance.ID)
		idx := slices.IndexFunc(models, func(m modelsindex.AIModel) bool { return m.ID == *req.Model })
		if idx == -1 {
			return fmt.Errorf("model %s does not exsist", *req.Model)
		}

		brickInstance.Model = models[idx].ID
	}
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

func (s *Service) BrickUpdate(
	req BrickCreateUpdateRequest,
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
		models := s.modelsIndex.GetModelsByBrick(req.ID)
		idx := slices.IndexFunc(models, func(m modelsindex.AIModel) bool { return m.ID == *req.Model })
		if idx == -1 {
			return fmt.Errorf("model %s does not exsist", *req.Model)
		}
		brickModel = *req.Model
	}
	brick, present := s.bricksIndex.FindBrickByID(brickID)
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

func (s *Service) BrickDelete(
	appCurrent *app.ArduinoApp,
	id string,
) error {
	if _, present := s.bricksIndex.FindBrickByID(id); !present {
		return ErrBrickNotFound
	}

	appCurrent.Descriptor.Bricks = slices.DeleteFunc(appCurrent.Descriptor.Bricks, func(b app.Brick) bool {
		return b.ID == id
	})

	if err := appCurrent.Save(); err != nil {
		return ErrCannotSaveBrick
	}
	return nil
}
