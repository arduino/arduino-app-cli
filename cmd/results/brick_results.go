package results

import (
	"fmt"
	"strings"

	"mkuznets.com/go/tabwriter"

	"github.com/arduino/arduino-app-cli/internal/orchestrator"
)

type BrickListResult struct {
	Bricks []orchestrator.BrickListItem `json:"bricks"`
}

func (r BrickListResult) String() string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 8, 3, ' ', 0)

	fmt.Fprintln(w, "ID\tNAME\tAUTHOR\tDESCRIPTION")
	for _, brick := range r.Bricks {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", brick.ID, brick.Name, brick.Author, brick.Description)
	}

	w.Flush()
	return b.String()
}

func (r BrickListResult) Data() interface{} {
	return r
}

type BrickDetailsResult struct {
	orchestrator.BrickDetailsResult
}

func (r BrickDetailsResult) String() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Name:        %s\n", r.Name))
	b.WriteString(fmt.Sprintf("ID:          %s\n", r.ID))
	b.WriteString(fmt.Sprintf("Author:      %s\n", r.Author))
	b.WriteString(fmt.Sprintf("Category:    %s\n", r.Category))
	b.WriteString(fmt.Sprintf("Status:      %s\n", r.Status))
	b.WriteString(fmt.Sprintf("\nDescription:\n%s\n", r.Description))

	if len(r.Variables) > 0 {
		b.WriteString("\nVariables:\n")
		for name, variable := range r.Variables {
			b.WriteString(fmt.Sprintf("  - %s (default: '%s', required: %t)\n", name, variable.DefaultValue, variable.Required))
		}
	}

	if r.Readme != "" {
		b.WriteString("\n--- README ---\n")
		b.WriteString(r.Readme)
	}

	return b.String()
}

func (r BrickDetailsResult) Data() interface{} {
	return r.BrickDetailsResult
}
