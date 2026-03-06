package brickfinder

import (
	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/brickslocalindex"
	"github.com/arduino/arduino-app-cli/internal/store"
)

type BrickResolver struct {
	local   *brickslocalindex.BricksIndex
	builtin *bricksindex.BricksIndex
	store   *store.StaticStore
}

func New(bricksIndex *bricksindex.BricksIndex, localBrickIndex *brickslocalindex.BricksIndex, staticStore *store.StaticStore) *BrickResolver {
	return &BrickResolver{
		local:   localBrickIndex,
		builtin: bricksIndex,
		store:   staticStore,
	}
}

func (f *BrickResolver) FindBrickByID(id string) (*bricksindex.Brick, bool) {
	if f.local != nil {
		if l, found := f.local.FindBrickByID(id); found {
			return &l.Brick, true
		}
	}
	return f.builtin.FindBrickByID(id)
}

func (f *BrickResolver) GetBrickComposeFilePathFromID(brickID string) (*paths.Path, error) {
	if f.local != nil {
		if l, found := f.local.FindBrickByID(brickID); found {
			return l.ComposeFile, nil
		}
	}
	return f.store.GetBrickComposeFilePathFromID(brickID)
}
