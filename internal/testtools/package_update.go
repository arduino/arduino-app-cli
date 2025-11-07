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
package testtools

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

func DockerBuild(t *testing.T) {

	if runtime.GOOS != "linux" && os.Getenv("CI") != "" {
		t.Skip("Skipping tests in CI that requires docker on non-Linux systems")
	}
	t.Helper()

	cmd := exec.Command("docker", "build", "-t", "adbd", "-f", "test.Dockerfile", ".")
	cmd.Dir = getBaseProjectPath(t)
	err := cmd.Run()
	if err != nil {
		t.Fatalf("failed to build adb daemon: %v", err)
	}

}
