package brick

import (
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/arduino-app-cli/internal/servicelocator"
	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/orchestrator/bricks"
	"github.com/arduino/arduino-app-cli/internal/tablestyle"
)

func newBricksListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all available bricks",
		Run: func(cmd *cobra.Command, args []string) {
			bricksListHandler()
		},
	}
}
func bricksListHandler() {
	res, err := servicelocator.GetBrickService().List()
	if err != nil {
		feedback.Fatal(err.Error(), feedback.ErrGeneric)
	}
	feedback.PrintResult(brickListResult{Bricks: res.Bricks})
}

type brickListResult struct {
	Bricks []bricks.BrickListItem `json:"bricks"`
}

func (r brickListResult) String() string {
	t := table.NewWriter()
	t.SetStyle(tablestyle.CustomCleanStyle)
	t.AppendHeader(table.Row{"ID", "NAME", "AUTHOR"})

	for _, brick := range r.Bricks {
		t.AppendRow(table.Row{
			brick.ID,
			brick.Name,
			brick.Author,
		})
	}
	return t.Render()
}

func (r brickListResult) Data() interface{} {
	return r
}
