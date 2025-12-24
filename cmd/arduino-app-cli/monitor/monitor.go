// This file is part of arduino-app-cli.
//
// Copyright 2025 ARDUINO SA (http://www.arduino.cc/)
//
// This software is released under the GNU General Public License version 3,
// which covers the main part of arduino-app-cli.
// The terms of this license can be found at:
// https://www.gnu.org/licenses/gpl-3.0.en.html
//
// You can be released from the requirements of the above licenses by purchasing
// a commercial license. Buying such a license is mandatory if you want to
// modify or otherwise use the software for commercial activities involving the
// Arduino software without disclosing the source code of your own applications.
// To purchase a commercial license, send an email to license@arduino.cc.

package monitor

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/arduino/arduino-app-cli/cmd/feedback"
	"github.com/arduino/arduino-app-cli/internal/monitor"
)

func NewMonitorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "monitor",
		Short: "Attach to the microcontroller serial monitor",
		RunE: func(cmd *cobra.Command, args []string) error {
			stdout, _, err := feedback.DirectStreams()
			if err != nil {
				return err
			}
			start, err := monitor.NewMonitorHandler(&combinedReadWrite{r: os.Stdin, w: stdout}) // nolint:forbidigo
			if err != nil {
				return err
			}
			go start()
			<-cmd.Context().Done()
			return nil
		},
	}
}

type combinedReadWrite struct {
	r io.Reader
	w io.Writer
}

func (crw *combinedReadWrite) Read(p []byte) (n int, err error) {
	return crw.r.Read(p)
}

func (crw *combinedReadWrite) Write(p []byte) (n int, err error) {
	return crw.w.Write(p)
}

func (crw *combinedReadWrite) Close() error {
	return nil
}
