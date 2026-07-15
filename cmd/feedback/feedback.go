// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package feedback

import (
	"bytes"
	"encoding/json"
	"errors"
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
	// JSON is a single, indented JSON document
	JSON
	// MinifiedJSON is a single, compact JSON document
	MinifiedJSON
	// JSONLines is a stream of compact JSON documents
	JSONLines
)

var formats = map[string]OutputFormat{
	"json":       JSON,
	"jsonmini":   MinifiedJSON,
	"json-lines": JSONLines,
	"text":       Text,
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
	Data() any
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

func GetStdin() *os.File {
	return os.Stdin
}

// GetFormat returns the output format currently set
func GetFormat() OutputFormat {
	return format
}

// Printf behaves like fmt.Printf but writes on the out writer and adds a newline.
func Printf(format string, v ...any) {
	Print(fmt.Sprintf(format, v...))
}

// Print behaves like fmt.Print but writes on the out writer and adds a newline.
func Print(v string) {
	fmt.Fprintln(feedbackOut, v)
}

// Warning outputs a warning message.
func Warnf(msg string, args ...any) {
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
	case MinifiedJSON, JSONLines:
		d, _ = json.Marshal(augment(res))
	default:
		panic("unknown output format")
	}
	fmt.Fprintln(stdErr, string(d))
	os.Exit(int(exitCode))
}

func augment(data any) any {
	if len(bufferWarnings) == 0 {
		return data
	}
	d, err := json.Marshal(data)
	if err != nil {
		return data
	}
	var res any
	if err := json.Unmarshal(d, &res); err != nil {
		return data
	}
	if m, ok := res.(map[string]any); ok {
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
	case MinifiedJSON, JSONLines:
		// A single result under JSONLines is just one compact line.
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

// NewResultStream returns a function to print an unbounded stream of Results,
// like the events emitted by a long running command. The results are written to
// the out writer as they come, they are not accumulated, so this is the
// counterpart of PrintResult for a stream (and the two must not be mixed).
//
// The JSON and MinifiedJSON formats are single-document formats, so they are
// not available for a stream and the function errors: JSONLines must be used
// instead, it renders the stream as one JSON object per line.
func NewResultStream() (func(Result), error) {
	if !formatSelected {
		panic("output format not yet selected")
	}
	switch format {
	case Text:
		return func(res Result) {
			if data := res.String(); data != "" {
				fmt.Fprintln(stdOut, data)
			}
		}, nil
	case JSONLines:
		enc := json.NewEncoder(stdOut)
		return func(res Result) {
			// Encode writes a single line per result, the warnings are not
			// augmented in: they would be repeated in every result of the stream.
			_ = enc.Encode(res.Data())
		}, nil
	default:
		return nil, errors.New(i18n.Tr("streaming output is not available in %[1]s format, use %[2]s to get one JSON object per line", format, JSONLines))
	}
}
