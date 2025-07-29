package results

import (
	"encoding/base64"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/arduino/go-paths-helper"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

type AppListResult struct {
	Apps       []orchestrator.AppInfo       `json:"apps"`
	BrokenApps []orchestrator.BrokenAppInfo `json:"brokenApps"`
}

func (r AppListResult) String() string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tICON\tSTATUS\tEXAMPLE")

	for _, app := range r.Apps {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%t\n",
			IdToAlias(app.ID),
			app.Name,
			app.Icon,
			app.Status,
			app.Example,
		)
	}
	if len(r.BrokenApps) > 0 {
		fmt.Fprintln(w, "\nAPP\tERROR")
		for _, app := range r.BrokenApps {
			fmt.Fprintf(w, "%s\t%s\n",
				app.Name,
				app.Error,
			)
		}
	}
	w.Flush()
	return b.String()
}

func (r AppListResult) Data() interface{} {
	return r
}
func IdToAlias(id orchestrator.ID) string {
	v := id.String()
	res, err := base64.RawURLEncoding.DecodeString(v)
	if err != nil {
		return v
	}

	v = string(res)
	if strings.Contains(v, ":") {
		return v
	}

	wd, err := paths.Getwd()
	if err != nil {
		return v
	}
	rel, err := paths.New(v).RelFrom(wd)
	if err != nil {
		return v
	}
	if !strings.HasPrefix(rel.String(), "./") && !strings.HasPrefix(rel.String(), "../") {
		return "./" + rel.String()
	}
	return rel.String()
}

type CreateAppResult struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Result  string `json:"result"`
}

func (r CreateAppResult) String() string {
	return fmt.Sprintf("%s: %s (%s)", r.Message, r.Path, r.Result)
}
func (r CreateAppResult) Data() interface{} {
	return r
}

type StartAppResult struct {
	AppName string                        `json:"appName"`
	Status  string                        `json:"status"`
	Output  *feedback.OutputStreamsResult `json:"output,omitempty"`
}

func (r StartAppResult) String() string {
	return fmt.Sprintf("✓ App %q started successfully", r.AppName)
}

func (r StartAppResult) Data() interface{} {
	return r
}

type StopAppResult struct {
	AppName string                        `json:"appName"`
	Status  string                        `json:"status"`
	Output  *feedback.OutputStreamsResult `json:"output,omitempty"`
}

func (r StopAppResult) String() string {
	return fmt.Sprintf("✓ App '%q stopped successfully.", r.AppName)
}

func (r StopAppResult) Data() interface{} {
	return r
}
