// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package servicesindex

import (
	"fmt"
	"os"
	"slices"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"
)

type ServicesIndex struct {
	Services []Service `yaml:"services"`
}

type Service struct {
	ServiceID       string   `yaml:"service_id"`
	Name            string   `yaml:"name"`
	Description     string   `yaml:"description,omitempty"`
	Category        string   `yaml:"category"`
	SupportedBoards []string `yaml:"supported_boards"`

	ComposeFile *paths.Path `yaml:"-"` // brick_compose.yaml file path, optional
}

func Load(dir *paths.Path) (*ServicesIndex, error) {
	// If assets/<version>/services does not exist, we return an empty index without error, to allow the CLI to work without services
	if !dir.IsDir() {
		return &ServicesIndex{}, nil
	}
	services, err := loadFromFolder(dir)
	if err != nil {
		return nil, err
	}
	return &ServicesIndex{Services: services}, nil
}

func (s Service) GetComposeFile() (*paths.Path, bool) {
	if s.ComposeFile == nil || s.ComposeFile.NotExist() {
		return nil, false
	}
	return s.ComposeFile, true
}

func (s *ServicesIndex) FindServiceByID(id string) (*Service, bool) {
	idx := slices.IndexFunc(s.Services, func(service Service) bool {
		return service.ServiceID == id
	})
	if idx == -1 {
		return nil, false
	}
	return &s.Services[idx], true
}

func loadFromFolder(dir *paths.Path) ([]Service, error) {
	pathsList, err := dir.ReadDirRecursiveFiltered(nil, paths.AndFilter(paths.FilterDirectories(), func(file *paths.Path) bool {
		return file.Join("service_config.yaml").Exist()
	}))
	if err != nil {
		return nil, err
	}

	services := make([]Service, 0, len(pathsList))
	for _, path := range pathsList {
		service, err := load(path)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	return services, nil
}

func load(servicePath *paths.Path) (a Service, err error) {
	serviceConfigPath := servicePath.Join("service_config.yaml")
	if serviceConfigPath.NotExist() {
		return Service{}, fmt.Errorf("service_config.yaml does not exist: %v", serviceConfigPath)
	}
	serviceConfigContent, err := os.ReadFile(serviceConfigPath.String())
	if err != nil {
		return Service{}, fmt.Errorf("cannot read service_config.yaml: %w", err)
	}
	var service Service
	if err := yaml.Unmarshal(serviceConfigContent, &service); err != nil {
		return Service{}, fmt.Errorf("cannot unmarshal service_config.yaml: %w", err)
	}
	service.ComposeFile = servicePath.Join("service_compose.yaml")
	return service, nil
}
