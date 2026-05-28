// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package servicesindex

import (
	"fmt"
	"os"
	"slices"

	"github.com/arduino/go-paths-helper"
	"github.com/goccy/go-yaml"

	"github.com/arduino/arduino-app-cli/internal/platform"
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

func Load(platform platform.Platform, dir *paths.Path) (*ServicesIndex, error) {
	// If assets/<version>/services does not exist, we return an empty index without error, to allow the CLI to work without services
	if !dir.IsDir() {
		return &ServicesIndex{}, nil
	}
	services, err := loadFromFolder(platform, dir)
	if err != nil {
		return nil, err
	}
	services = slices.DeleteFunc(services, func(service Service) bool {
		return platform.BoardName != "" &&
			len(service.SupportedBoards) != 0 &&
			!slices.Contains(service.SupportedBoards, platform.BoardName)
	})
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

func loadFromFolder(platform platform.Platform, dir *paths.Path) ([]Service, error) {
	pathsList, err := dir.ReadDirRecursiveFiltered(nil, paths.AndFilter(paths.FilterDirectories(), func(file *paths.Path) bool {
		return file.Join("service_config.yaml").Exist()
	}))
	if err != nil {
		return nil, err
	}

	services := make([]Service, 0, len(pathsList))
	for _, path := range pathsList {
		service, err := load(platform, path)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}
	return services, nil
}

func load(platform platform.Platform, servicePath *paths.Path) (a Service, err error) {
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
	composeFile := servicePath.Join("service_compose.yaml")
	if platform.BoardName != "" {
		if platformCompose := servicePath.Join("service_compose." + platform.BoardName + ".yaml"); platformCompose.Exist() {
			composeFile = platformCompose
		}
	}
	service.ComposeFile = composeFile
	return service, nil
}
