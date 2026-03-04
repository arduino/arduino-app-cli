package brickfinder

import (
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricksindex"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/brickslocalindex"
)

type Resolver interface {
	FindBrickByID(id string) (*bricksindex.Brick, bool)
}

type BrickResolver struct {
	local *brickslocalindex.BricksIndex
	index *bricksindex.BricksIndex
}

func New(bricksIndex *bricksindex.BricksIndex, localBrickIndex *brickslocalindex.BricksIndex) *BrickResolver {
	return &BrickResolver{
		local: localBrickIndex,
		index: bricksIndex,
	}
}

func (f *BrickResolver) FindBrickByID(id string) (*bricksindex.Brick, bool) {
	if f.local != nil {
		if brick, found := f.local.FindBrickByID(id); found {
			return brick, true
		}
	}
	return f.index.FindBrickByID(id)
}
