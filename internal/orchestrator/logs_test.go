package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertDockerLogsToMessage(t *testing.T) {
	log1 := `models-runner-1  | INFO: Created TensorFlow Lite XNNPACK delegate for CPU.`
	assert.Equal(t, LogMessage{
		Name:    "models-runner",
		Content: "INFO: Created TensorFlow Lite XNNPACK delegate for CPU.",
	}, convertDockerLogToLogMessage(log1))

	log2 := `main-1           | Using CPython 3.13.3 interpreter at: /usr/local/bin/python`
	assert.Equal(t, LogMessage{
		Name:    "main",
		Content: "Using CPython 3.13.3 interpreter at: /usr/local/bin/python",
	}, convertDockerLogToLogMessage(log2))

	log3 := `|`
	assert.Equal(t, LogMessage{
		Name:    "",
		Content: "",
	}, convertDockerLogToLogMessage(log3))

	log4 := `main-1 |`
	assert.Equal(t, LogMessage{
		Name:    "main",
		Content: "",
	}, convertDockerLogToLogMessage(log4))

	log5 := "main-1 |\n"
	assert.Equal(t, LogMessage{
		Name:    "main",
		Content: "",
	}, convertDockerLogToLogMessage(log5))

	// Non parsable cases
	for _, log := range []string{
		``,
		`\n`,
		`main-1 missing the pipe to separate the service name from the message`,
	} {
		assert.Equal(t, LogMessage{
			Name:    "",
			Content: log,
		}, convertDockerLogToLogMessage(log))
	}
}
