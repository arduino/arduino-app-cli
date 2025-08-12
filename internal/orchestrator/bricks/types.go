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
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Author    string            `json:"author"`
	Category  string            `json:"category"`
	Status    string            `json:"status"`
	Variables map[string]string `json:"variables,omitempty"`
	ModelID   string            `json:"model,omitempty"`
}

type BrickVariable struct {
	DefaultValue string `json:"default_value,omitempty"`
	Description  string `json:"description,omitempty"`
	Required     bool   `json:"required"`
}

type AppReference struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

type BrickDetailsResult struct {
	ID          string                   `json:"id"`
	Name        string                   `json:"name"`
	Author      string                   `json:"author"`
	Description string                   `json:"description"`
	Category    string                   `json:"category"`
	Status      string                   `json:"status"`
	Variables   map[string]BrickVariable `json:"variables,omitempty"`
	Readme      string                   `json:"readme"`
	UsedByApps  []AppReference           `json:"used_by_apps"`
}
