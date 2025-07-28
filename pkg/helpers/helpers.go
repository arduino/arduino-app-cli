package helpers

import (
	"fmt"

	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
)

func ArduinoCLIDownloadProgressToString(progress *rpc.DownloadProgress) string {
	switch {
	case progress.GetStart() != nil:
		return fmt.Sprintf("Download started: %s", progress.GetStart().GetUrl())
	case progress.GetUpdate() != nil:
		return fmt.Sprintf("Download progress: %s", progress.GetUpdate())
	case progress.GetEnd() != nil:
		return fmt.Sprintf("Download completed: %s", progress.GetEnd())
	}
	return progress.String()
}

func ArduinoCLITaskProgressToString(progress *rpc.TaskProgress) string {
	data := fmt.Sprintf("Task %s:", progress.GetName())
	if progress.GetMessage() != "" {
		data += fmt.Sprintf(" (%s)", progress.GetMessage())
	}
	if progress.GetCompleted() {
		data += " completed"
	} else {
		data += fmt.Sprintf(" %.2f%%", progress.GetPercent())
	}
	return data
}
