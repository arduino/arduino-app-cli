package bricksindex

import (
	"path"
	"slices"

	yaml "github.com/goccy/go-yaml"
	"go.bug.st/f"
	semver "go.bug.st/relaxed-semver"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/assets"
)

type BricksIndex map[string]*BricksCollections

func (b *BricksIndex) GetNamespace(name string) (*BricksCollections, bool) {
	if b == nil {
		b = f.Ptr(make(BricksIndex))
	}
	if collection, ok := (*b)[name]; ok {
		return collection, true
	}
	return nil, false
}

func (b *BricksIndex) GetCollection(namespace, name string) (*BricksCollection, bool) {
	if b == nil {
		b = f.Ptr(make(BricksIndex))
	}
	if collections, ok := (*b)[namespace]; ok {
		if collection, ok := (*collections)[name]; ok {
			return collection, true
		}
	}
	return &BricksCollection{}, false
}

func (b *BricksIndex) AddCollection(namespace string, collection *BricksCollection) {
	if b == nil {
		b = f.Ptr(make(BricksIndex))
	}
	if _, ok := (*b)[namespace]; !ok {
		(*b)[namespace] = &BricksCollections{}
	}
	(*(*b)[namespace])[collection.Name] = collection
}

type BricksCollections map[string]*BricksCollection

func (b *BricksCollections) GetCollection(name string) (*BricksCollection, bool) {
	if collection, ok := (*b)[name]; ok {
		return collection, true
	}
	return &BricksCollection{}, false
}

type BricksCollection struct {
	Name          string          `yaml:"-"`
	Package       string          `yaml:"package"`
	Repository    string          `yaml:"repository"`
	LatestRelease *semver.Version `yaml:"latest-release"`
	Releases      []*BrickRelease `yaml:"releases"`
}

func (b *BricksCollection) UnmarshalYAML(unmarshal func(any) error) error {
	type brickCollectionAlias struct {
		Package       string          `yaml:"package"`
		Repository    string          `yaml:"repository"`
		LatestRelease string          `yaml:"latest-release"`
		Releases      []*BrickRelease `yaml:"releases"`
	}
	var raw brickCollectionAlias
	if err := unmarshal(&raw); err != nil {
		return err
	}
	b.Package = raw.Package
	b.Repository = raw.Repository
	if raw.LatestRelease != "" {
		r, err := semver.Parse(raw.LatestRelease)
		if err != nil {
			return err
		}
		b.LatestRelease = r
	}
	b.Releases = raw.Releases
	return nil
}

func (b *BricksCollection) GetRelease(version *semver.Version) (*BrickRelease, bool) {
	if version == nil {
		return nil, false
	}
	for i := range b.Releases {
		if b.Releases[i].Version.Equal(version) {
			return b.Releases[i], true
		}
	}
	return nil, false
}

type BrickRelease struct {
	Version *semver.Version `yaml:"version"`
	Bricks  []*Brick        `yaml:"bricks"`
}

func (b *BrickRelease) FindBrickByID(brickName string) (*Brick, bool) {
	brickIdx := slices.IndexFunc(b.Bricks, func(b *Brick) bool {
		return b.ID == brickName
	})
	if brickIdx == -1 {
		return nil, false
	}
	return b.Bricks[brickIdx], true
}

func (b *BrickRelease) UnmarshalYAML(unmarshal func(any) error) error {
	type brickReleaseAlias BrickRelease // Trick to avoid infinite recursion
	var raw brickReleaseAlias
	if err := unmarshal(&raw); err != nil {
		return err
	}
	b.Version = raw.Version
	b.Bricks = raw.Bricks
	for i := range b.Bricks {
		b.Bricks[i].Version = raw.Version.String()
	}
	return nil
}

type Brick struct {
	ID          string                          `yaml:"id"`
	Name        string                          `yaml:"name"`
	Version     string                          `yaml:"-"`
	Variables   map[string]BrickReleaseVariable `yaml:"variables,omitempty"`
	Description string                          `yaml:"description,omitempty"`
	Ports       []string                        `yaml:"ports,omitempty"`
}

type BrickReleaseVariable struct {
	DefaultValue string `yaml:"default_value,omitempty"`
	Description  string `yaml:"description,omitempty"`
}

type assetsBrickList struct {
	Bricks []assetsBricks `yaml:"bricks"`
}
type assetsVariables struct {
	Name         string `yaml:"name"`
	DefaultValue string `yaml:"default_value"`
	Description  string `yaml:"description,omitempty"`
}
type assetsBricks struct {
	ID                string            `yaml:"id"`
	Name              string            `yaml:"name"`
	ModuleDescription string            `yaml:"description"`
	RequireContainer  bool              `yaml:"require_container"`
	Variables         []assetsVariables `yaml:"variables,omitempty"`
	Ports             []string          `yaml:"ports,omitempty"`
}

func GenerateBricksIndex() (*BricksIndex, error) {
	versions, err := assets.FS.ReadDir("static")
	if err != nil {
		return nil, err
	}

	index := make(BricksIndex)
	collection := BricksCollection{
		Name:       "app-bricks",
		Package:    "app-bricks",
		Repository: "https://github.com/bcmi-labs/appslab-python-modules",
		Releases:   make([]*BrickRelease, len(versions)),
	}
	for i, version := range versions {
		bricksList, err := assets.FS.ReadFile(path.Join("static", version.Name(), "bricks-list.yaml"))
		if err != nil {
			return nil, err
		}
		var list assetsBrickList
		if err := yaml.Unmarshal(bricksList, &list); err != nil {
			return nil, err
		}
		brickRelease := &BrickRelease{
			Version: semver.MustParse(version.Name()),
			Bricks:  make([]*Brick, len(list.Bricks)),
		}
		for j, brick := range list.Bricks {
			variables := make(map[string]BrickReleaseVariable, len(brick.Variables))
			for _, variable := range brick.Variables {
				variables[variable.Name] = BrickReleaseVariable{
					DefaultValue: variable.DefaultValue,
					Description:  variable.Description,
				}
			}
			brickRelease.Bricks[j] = &Brick{
				ID:          brick.ID,
				Name:        brick.Name,
				Version:     version.Name(),
				Variables:   variables,
				Description: brick.ModuleDescription,
				Ports:       brick.Ports,
			}
		}
		collection.Releases[i] = brickRelease
	}

	// Sort on top the newest releases
	slices.SortFunc(collection.Releases, func(a, b *BrickRelease) int {
		if b.Version.GreaterThan(a.Version) {
			return 1
		}
		if b.Version.LessThan(a.Version) {
			return -1
		}
		return 0
	})

	collection.LatestRelease = collection.Releases[0].Version
	index.AddCollection("arduino", &collection)

	return &index, nil
}
