// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

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
