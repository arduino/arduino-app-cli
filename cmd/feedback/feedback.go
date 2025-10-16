package feedback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/arduino/arduino-app-cli/cmd/i18n"
)

// OutputFormat is an output format
type OutputFormat int

const (
	// Text is the plain text format, suitable for interactive terminals
	Text OutputFormat = iota
	// JSON format
	JSON
	// MinifiedJSON format
	MinifiedJSON
)

var formats = map[string]OutputFormat{
	"json":     JSON,
	"jsonmini": MinifiedJSON,
	"text":     Text,
}

func (f OutputFormat) String() string {
	for res, format := range formats {
		if format == f {
			return res
		}
	}
	panic("unknown output format")
}

// ParseOutputFormat parses a string and returns the corresponding OutputFormat.
// The boolean returned is true if the string was a valid OutputFormat.
func ParseOutputFormat(in string) (OutputFormat, bool) {
	format, found := formats[in]
	return format, found
}

var (
	stdOut         io.Writer
	stdErr         io.Writer
	feedbackOut    io.Writer
	feedbackErr    io.Writer
	bufferOut      *bytes.Buffer
	bufferErr      *bytes.Buffer
	bufferWarnings []string
	format         OutputFormat
	formatSelected bool
)

// nolint:gochecknoinits
func init() {
	reset()
}

// reset resets the feedback package to its initial state, useful for unit testing
func reset() {
	stdOut = os.Stdout
	stdErr = os.Stderr
	feedbackOut = os.Stdout
	feedbackErr = os.Stderr
	bufferOut = bytes.NewBuffer(nil)
	bufferErr = bytes.NewBuffer(nil)
	bufferWarnings = nil
	format = Text
	formatSelected = false
}

// Result is anything more complex than a sentence that needs to be printed
// for the user.
type Result interface {
	fmt.Stringer
	Data() interface{}
}

// ErrorResult is a result embedding also an error. In case of textual output
// the error will be printed on stderr.
type ErrorResult interface {
	Result
	ErrorString() string
}

// SetOut can be used to change the out writer at runtime
func SetOut(out io.Writer) {
	if formatSelected {
		panic("output format already selected")
	}
	stdOut = out
}

// SetErr can be used to change the err writer at runtime
func SetErr(err io.Writer) {
	if formatSelected {
		panic("output format already selected")
	}
	stdErr = err
}

// SetFormat can be used to change the output format at runtime
func SetFormat(f OutputFormat) {
	if formatSelected {
		panic("output format already selected")
	}
	format = f
	formatSelected = true

	if format == Text {
		feedbackOut = io.MultiWriter(bufferOut, stdOut)
		feedbackErr = io.MultiWriter(bufferErr, stdErr)
	} else {
		feedbackOut = bufferOut
		feedbackErr = bufferErr
		bufferWarnings = nil
	}
}

// GetFormat returns the output format currently set
func GetFormat() OutputFormat {
	return format
}

// Printf behaves like fmt.Printf but writes on the out writer and adds a newline.
func Printf(format string, v ...interface{}) {
	Print(fmt.Sprintf(format, v...))
}

// Print behaves like fmt.Print but writes on the out writer and adds a newline.
func Print(v string) {
	fmt.Fprintln(feedbackOut, v)
}

// Warning outputs a warning message.
func Warnf(msg string, args ...interface{}) {
	msg = fmt.Sprintf(msg, args...)
	if format == Text {
		fmt.Fprintln(feedbackErr, msg)
	} else {
		bufferWarnings = append(bufferWarnings, msg)
	}
	slog.Warn(msg)
}

// FatalError outputs the error and exits with status exitCode.
func FatalError(err error, exitCode ExitCode) {
	Fatal(err.Error(), exitCode)
}

// FatalResult outputs the result and exits with status exitCode.
func FatalResult(res ErrorResult, exitCode ExitCode) {
	PrintResult(res)
	os.Exit(int(exitCode))
}

// Fatal outputs the errorMsg and exits with status exitCode.
func Fatal(errorMsg string, exitCode ExitCode) {
	if format == Text {
		fmt.Fprintln(stdErr, errorMsg)
		os.Exit(int(exitCode))
	}

	type FatalError struct {
		Error  string               `json:"error"`
		Output *OutputStreamsResult `json:"output,omitempty"`
	}
	res := &FatalError{
		Error: errorMsg,
	}
	if output := getOutputStreamResult(); !output.Empty() {
		res.Output = output
	}
	var d []byte
	switch format {
	case JSON:
		d, _ = json.MarshalIndent(augment(res), "", "  ")
	case MinifiedJSON:
		d, _ = json.Marshal(augment(res))
	default:
		panic("unknown output format")
	}
	fmt.Fprintln(stdErr, string(d))
	os.Exit(int(exitCode))
}

func augment(data interface{}) interface{} {
	if len(bufferWarnings) == 0 {
		return data
	}
	d, err := json.Marshal(data)
	if err != nil {
		return data
	}
	var res interface{}
	if err := json.Unmarshal(d, &res); err != nil {
		return data
	}
	if m, ok := res.(map[string]interface{}); ok {
		m["warnings"] = bufferWarnings
	}
	return res
}

// PrintResult is a convenient wrapper to provide feedback for complex data,
// where the contents can't be just serialized to JSON but requires more
// structure.
func PrintResult(res Result) {
	var data string
	var dataErr string
	switch format {
	case JSON:
		d, err := json.MarshalIndent(augment(res.Data()), "", "  ")
		if err != nil {
			Fatal(i18n.Tr("Error during JSON encoding of the output: %v", err), ErrGeneric)
		}
		data = string(d)
	case MinifiedJSON:
		d, err := json.Marshal(augment(res.Data()))
		if err != nil {
			Fatal(i18n.Tr("Error during JSON encoding of the output: %v", err), ErrGeneric)
		}
		data = string(d)
	case Text:
		data = res.String()
		if resErr, ok := res.(ErrorResult); ok {
			dataErr = resErr.ErrorString()
		}
	default:
		panic("unknown output format")
	}
	if data != "" {
		fmt.Fprintln(stdOut, data)
	}
	if dataErr != "" {
		fmt.Fprintln(stdErr, dataErr)
	}
}
