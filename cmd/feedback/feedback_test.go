// This file is part of arduino-app-cli.
//
// Copyright 2020 ARDUINO SA (http://www.arduino.cc/)
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package feedback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOutputSelection(t *testing.T) {
	reset()

	myErr := new(bytes.Buffer)
	myOut := new(bytes.Buffer)
	SetOut(myOut)
	SetErr(myErr)
	SetFormat(Text)

	// Could not change output stream after format has been set
	require.Panics(t, func() { SetOut(nil) })
	require.Panics(t, func() { SetErr(nil) })

	// Coule not change output format twice
	require.Panics(t, func() { SetFormat(JSON) })

	Print("Hello")
	require.Equal(t, myOut.String(), "Hello\n")
}

func TestJSONOutputStream(t *testing.T) {
	reset()

	require.Panics(t, func() { OutputStreams() })

	SetFormat(JSON)
	stdout, stderr, res := OutputStreams()
	fmt.Fprint(stdout, "Hello")
	fmt.Fprint(stderr, "Hello ERR")

	d, err := json.Marshal(res())
	require.NoError(t, err)
	require.JSONEq(t, `{"stdout":"Hello","stderr":"Hello ERR"}`, string(d))

	stdout.Write([]byte{0xc2, 'A'}) // Invaid UTF-8

	d, err = json.Marshal(res())
	require.NoError(t, err)
	require.JSONEq(t, string(d), `{"stdout":"Hello\ufffdA","stderr":"Hello ERR"}`)
}

func TestJsonOutputOnCustomStreams(t *testing.T) {
	reset()

	myErr := new(bytes.Buffer)
	myOut := new(bytes.Buffer)
	SetOut(myOut)
	SetErr(myErr)
	SetFormat(JSON)

	// Could not change output stream after format has been set
	require.Panics(t, func() { SetOut(nil) })
	require.Panics(t, func() { SetErr(nil) })
	// Could not change output format twice
	require.Panics(t, func() { SetFormat(JSON) })

	Print("Hello") // Output interactive data

	require.Equal(t, "", myOut.String())
	require.Equal(t, "", myErr.String())
	require.Equal(t, "Hello\n", bufferOut.String())

	PrintResult(&testResult{Success: true})

	require.JSONEq(t, myOut.String(), `{ "success": true }`)
	require.Equal(t, myErr.String(), "")
	myOut.Reset()

	_, _, res := OutputStreams()
	PrintResult(&testResult{Success: false, Output: res()})

	require.JSONEq(t, `
{
  "success": false,
  "output": {
    "stdout": "Hello\n",
    "stderr": ""
  }
}`, myOut.String())
	require.Equal(t, myErr.String(), "")
}

type testResult struct {
	Success bool                 `json:"success"`
	Output  *OutputStreamsResult `json:"output,omitempty"`
}

func (r *testResult) Data() any {
	return r
}

func (r *testResult) String() string {
	if r.Success {
		return "Success"
	}
	return "Failure"
}

func TestResultStreamOnTextFormat(t *testing.T) {
	reset()

	myOut := &bytes.Buffer{}
	SetOut(myOut)
	SetFormat(Text)

	printResult, err := NewResultStream()
	require.NoError(t, err)

	printResult(&testResult{Success: true})
	printResult(&testResult{Success: false})

	require.Equal(t, "Success\nFailure\n", myOut.String())
}

func TestResultStreamOnJSONLinesFormat(t *testing.T) {
	reset()

	myOut := &bytes.Buffer{}
	SetOut(myOut)
	SetFormat(JSONLines)

	printResult, err := NewResultStream()
	require.NoError(t, err)

	printResult(&testResult{Success: true})
	printResult(&testResult{Success: false})

	// The stream is rendered as one JSON object per line.
	require.Equal(t, "{\"success\":true}\n{\"success\":false}\n", myOut.String())
}

func TestResultStreamIsNotAvailableOnSingleDocumentFormat(t *testing.T) {
	// A stream can't be rendered as a single JSON document.
	reset()
	SetOut(&bytes.Buffer{})
	SetFormat(JSON)

	_, err := NewResultStream()
	require.Error(t, err)
}
