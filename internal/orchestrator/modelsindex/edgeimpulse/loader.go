package edgeimpulse

import (
	"io/fs"
	"path/filepath"

	"github.com/arduino/go-paths-helper"
	yaml "github.com/goccy/go-yaml"
)

type Loader struct {
	dir *paths.Path
}

func New(dir *paths.Path) *Loader {
	return &Loader{dir: dir}
}

type ModelDescriptor struct {
	ProjectId   int    `yaml:"project-id"`
	ImpulseID   int    `yaml:"impulse-id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Category    string `yaml:"category"`
	Path        string `yaml:"-"`
}

func (e *Loader) List() ([]ModelDescriptor, error) {
	models := make([]ModelDescriptor, 0)

	err := filepath.WalkDir(e.dir.String(), func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			return nil
		}

		base := filepath.Base(path)
		if base != "metadata.yml" && base != "metadata.yaml" {
			return nil
		}

		f, err := paths.New(path).Open()
		if err != nil {
			return err
		}
		defer f.Close()

		var mf ModelDescriptor
		if err := yaml.NewDecoder(f).Decode(&mf); err != nil {
			return err
		}
		mf.Path = filepath.Dir(path)

		models = append(models, mf)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return models, nil
}
