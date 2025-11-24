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

package bricks

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

type AppBrickInstancesResult struct {
	BrickInstances []BrickInstance `json:"bricks"`
}

type BrickInstance struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Author          string                `json:"author"`
	Category        string                `json:"category"`
	Status          string                `json:"status"`
	Variables       map[string]string     `json:"variables,omitempty" description:"Deprecated: use config_variables instead. This field is kept for backward compatibility."`
	ConfigVariables []BrickConfigVariable `json:"config_variables,omitempty"`
	ModelID         string                `json:"model,omitempty"`
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

type BrickDetailsResult struct {
	ID           string                   `json:"id"`
	Name         string                   `json:"name"`
	Author       string                   `json:"author"`
	Description  string                   `json:"description"`
	Category     string                   `json:"category"`
	Status       string                   `json:"status"`
	Variables    map[string]BrickVariable `json:"variables,omitempty"`
	Readme       string                   `json:"readme"`
	ApiDocsPath  string                   `json:"api_docs_path"`
	CodeExamples []CodeExample            `json:"code_examples"`
	UsedByApps   []AppReference           `json:"used_by_apps"`
}
