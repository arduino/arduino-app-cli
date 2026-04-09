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

package servicesindex

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"
)

func TestLoadServicesIndex(t *testing.T) {
	servicesIndex, err := Load(paths.New("testdata/services"))
	require.NoError(t, err)

	service, ok := servicesIndex.FindServiceByID("arduino:foo")
	require.True(t, ok)
	require.Equal(t, "Foo Service", service.Name)
	require.Equal(t, "test", service.Category)
	require.Equal(t, []string{"foobar"}, service.SupportedBoards)

	compose, ok := service.GetComposeFile()
	require.True(t, ok)
	require.Equal(t, paths.New("testdata", "services", "arduino", "foo", "service_compose.yaml").String(), compose.String())
}
