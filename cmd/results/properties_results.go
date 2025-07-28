package results

import (
	"fmt"

	"github.com/arduino/arduino-app-cli/internal/orchestrator/app"
)

type DefaultAppResult struct {
	App *app.ArduinoApp `json:"app,omitempty"`
}

func (r DefaultAppResult) String() string {
	if r.App == nil {
		return "No default app set"
	}
	return fmt.Sprintf("Default app: %s (%s)", r.App.Name, r.App.FullPath)
}

func (r DefaultAppResult) Data() interface{} {
	return r
}
