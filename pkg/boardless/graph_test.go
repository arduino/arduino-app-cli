// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package boardless_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// What talks to docker stays out of the package, natively and for the PWA. Data types
// (compose-go, moby's api/types) are allowed: they only describe.
var dockerClient = []string{
	"github.com/docker/cli/cli/command",
	"github.com/docker/compose/",
	"github.com/moby/moby/client",
	"github.com/arduino/arduino-app-cli/internal/dockerhelper",
}

func TestNoDockerClientInGraph(t *testing.T) {
	for _, goos := range []string{"", "js"} {
		t.Run("GOOS="+goos, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "go", "list", "-e", "-deps", ".")
			cmd.Env = os.Environ()
			if goos == "js" {
				cmd.Env = append(cmd.Env, "GOOS=js", "GOARCH=wasm")
			}
			out, err := cmd.Output()
			require.NoError(t, err)
			for pkg := range strings.Lines(string(out)) {
				pkg = strings.TrimSpace(pkg)
				for _, banned := range dockerClient {
					require.False(t, pkg == banned || strings.HasPrefix(pkg, banned), "%s is in the import graph", pkg)
				}
			}
		})
	}
}
