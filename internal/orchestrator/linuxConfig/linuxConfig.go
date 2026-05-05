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

package linuxconfig

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/arduino/go-paths-helper"
)

const linuxConfigTool = "arduino-linux-config"

func GetEnabledCarriers(ctx context.Context) ([]string, error) {
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

	var enabled []string
	for _, c := range carriersStatus.Carriers {
		if c.CurrentEnabled {
			enabled = append(enabled, c.CarrierName)
		}
	}
	return enabled, nil
}
