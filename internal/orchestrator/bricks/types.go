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
	ModelID          string                `json:"model,omitempty" description:"The model this brick instance uses, as the plain id app.yaml holds. Unlike \"id\" on the models endpoints, it is not base64url encoded; encode it before naming a model in a path."`
	CompatibleModels []AIModel             `json:"compatible_models"`
	Readme           string                `json:"readme"`
}

// AIModel names a model a brick can use. Its id is the plain one, the same form the
// brick's own "model" field carries and the same one app.yaml holds, because a brick
// answer describes what an app.yaml says rather than addressing a model.
//
// This is the one place an id leaves the API unencoded. The models endpoints report the
// base64url form as "id" and the plain one as "id_decoded", and take only the encoded
// form back. So an id read here has to be encoded before it names a model in a path or in
// a brick request's "model".
type AIModel struct {
	ID          string `json:"id" description:"The model's plain id, not base64url encoded. Encode it before sending it back on a models path or in a brick request's \"model\"."`
	Name        string `json:"name"`
	Description string `json:"description" description:"Deprecated: This field is kept for backward compatibility."`
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
	Path        string `json:"path"`
	EncodedID   string `json:"encoded_id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
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
