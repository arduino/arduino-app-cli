package results

import (
	"fmt"
	"strings"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

type ConfigResult struct {
	Config orchestrator.ConfigResponse
}

func (r ConfigResult) String() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Data Directory:     %s\n", r.Config.Directories.Data))
	b.WriteString(fmt.Sprintf("Apps Directory:     %s\n", r.Config.Directories.Apps))
	b.WriteString(fmt.Sprintf("Examples Directory: %s\n", r.Config.Directories.Examples))

	return b.String()
}

func (r ConfigResult) Data() interface{} {
	return r.Config
}
