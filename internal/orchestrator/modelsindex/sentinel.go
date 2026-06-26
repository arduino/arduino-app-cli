package modelsindex

import (
	"encoding/json"
	"fmt"

	"github.com/arduino/go-paths-helper"
	"go.bug.st/f"
)

// Artifact represents a single file produced by a model download,
// together with its size in bytes captured at download time.
type Artifact struct {
	Path string `json:"path"`
	Size uint64 `json:"size"`
}

type Sentinel struct {
	Artifacts []Artifact `json:"artifacts"`
}

// TotalSize returns the sum of the sizes of all artifacts in the sentinel.
func (s Sentinel) TotalSize() uint64 {
	var total uint64
	for _, a := range s.Artifacts {
		total += a.Size
	}
	return total
}

// Paths returns just the artifact file paths.
func (s Sentinel) Paths() []string {
	return f.Map(s.Artifacts, func(a Artifact) string { return a.Path })
}

func getSentinelPath(modelsDir *paths.Path, modelID string) *paths.Path {
	return modelsDir.Join(fmt.Sprintf(".%s.downloaded.json", modelID))
}

func LoadSentinel(modelsDir *paths.Path, modelID string) (Sentinel, error) {
	sentinelPath := getSentinelPath(modelsDir, modelID)

	f, err := sentinelPath.Open()
	if err != nil {
		return Sentinel{}, err
	}
	defer f.Close()

	var sentinel Sentinel
	if err := json.NewDecoder(f).Decode(&sentinel); err != nil {
		return Sentinel{}, err
	}

	return sentinel, nil
}

func SaveSentinel(modelsDir *paths.Path, modelID string, sentinel Sentinel) error {
	sentinelPath := getSentinelPath(modelsDir, modelID)

	f, err := sentinelPath.Create()
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(sentinel); err != nil {
		return err
	}

	return nil
}

func DeleteSentinel(modelsDir *paths.Path, modelID string) error {
	sentinelPath := getSentinelPath(modelsDir, modelID)

	if err := sentinelPath.Remove(); err != nil {
		return err
	}

	return nil
}
