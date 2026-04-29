// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package bricks

type BrickListResult struct {
	Bricks []BrickListItem `json:"bricks"`
}

type BrickListItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Author       string `json:"author"`
	Description  string `json:"description"`
	Category     string `json:"category"`
	Status       string `json:"status"`
	RequireModel bool   `json:"require_model"`
}

type AppBrickInstancesResult struct {
	BrickInstances []BrickInstance `json:"bricks"`
}

type BrickInstance struct {
	ID               string                `json:"id"`
	Name             string                `json:"name"`
	Author           string                `json:"author"`
	Category         string                `json:"category"`
	Status           string                `json:"status"`
	Variables        map[string]string     `json:"variables,omitempty" description:"Deprecated: use config_variables instead. This field is kept for backward compatibility."`
	ConfigVariables  []BrickConfigVariable `json:"config_variables,omitempty"`
	RequireModel     bool                  `json:"require_model"`
	ModelID          string                `json:"model,omitempty"`
	CompatibleModels []AIModel             `json:"compatible_models"`
	Readme           string                `json:"readme"`
}

type AIModel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
type BrickConfigVariable struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type BrickVariable struct {
	DefaultValue string `json:"default_value,omitempty"`
	Description  string `json:"description,omitempty"`
	Required     bool   `json:"required"`
}

type CodeExample struct {
	Path string `json:"path"`
}
type AppReference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

type LocalBrickRenameResult struct {
	ID string `json:"id"`
}

type BrickDetailsResult struct {
	ID               string                   `json:"id"`
	Name             string                   `json:"name"`
	Author           string                   `json:"author"`
	Description      string                   `json:"description"`
	Category         string                   `json:"category"`
	Status           string                   `json:"status"`
	RequireModel     bool                     `json:"require_model"`
	Variables        map[string]BrickVariable `json:"variables,omitempty" description:"Deprecated: use config_variables instead. This field is kept for backward compatibility."`
	Readme           string                   `json:"readme"`
	ApiDocsPath      string                   `json:"api_docs_path"`
	CodeExamples     []CodeExample            `json:"code_examples"`
	UsedByApps       []AppReference           `json:"used_by_apps"`
	CompatibleModels []AIModel                `json:"compatible_models"`
	ConfigVariables  []BrickConfigVariable    `json:"config_variables"`
}
