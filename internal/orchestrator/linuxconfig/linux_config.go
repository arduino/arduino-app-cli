// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package linuxconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/arduino/go-paths-helper"
	"go.bug.st/f"
)

const linuxConfigTool = "arduino-linux-config"

func GetEnabledCarriers(ctx context.Context) ([]Carrier, error) {
	if _, err := exec.LookPath(linuxConfigTool); err != nil {
		return nil, fmt.Errorf("arduino-linux-config tool not found in PATH: %w", err)
	}

	cmd, err := paths.NewProcess(nil, linuxConfigTool, "carrier", "show", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("failed to create process 'arduino-linux-config carrier show': %w", err)
	}

	stdout, stderr, err := cmd.RunAndCaptureOutput(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to execute 'arduino-linux-config carrier show': %w\nstderr: %s", err, string(stderr))
	}

	var carriersStatus CarrierStatusOutput
	if err := json.Unmarshal(stdout, &carriersStatus); err != nil {
		return nil, fmt.Errorf("failed to parse JSON from 'arduino-linux-config carrier show': %w\noutput: %s", err, string(stdout))
	}

	return f.Filter(carriersStatus.Carriers, func(c Carrier) bool { return c.CurrentEnabled }), nil
}
