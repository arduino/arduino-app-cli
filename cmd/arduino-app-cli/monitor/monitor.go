// This file is part of arduino-app-cli.
//
// Copyright (C) Arduino s.r.l. and/or its affiliated companies
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

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
