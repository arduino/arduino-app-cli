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
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

const linuxConfigTool = "arduino-linux-config"

func CarrierShow() (*CarrierStatusOutput, error) {
	if _, err := exec.LookPath(linuxConfigTool); err != nil {
		return nil, fmt.Errorf("arduino-linux-config tool not found in PATH: %w", err)
	}

	cmd := exec.Command(linuxConfigTool, "carrier", "show", "--format", "json")

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to execute 'arduino-linux-config carrier show': %w\nstderr: %s", err, stderr.String())
	}

	// 3. parsing JSON
	var result CarrierStatusOutput
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON from 'arduino-linux-config carrier show': %w\noutput: %s", err, out.String())
	}

	return &result, nil
}
