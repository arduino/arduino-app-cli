// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package servicesindex

import (
	"testing"

	"github.com/arduino/go-paths-helper"
	"github.com/stretchr/testify/require"

	"github.com/arduino/arduino-app-cli/internal/platform"
)

func TestLoadServicesIndex(t *testing.T) {
	servicesIndex, err := Load(platform.GetPlatform(nil), paths.New("testdata/services"))
	require.NoError(t, err)

	service, ok := servicesIndex.FindServiceByID("arduino:foobar")
	require.True(t, ok)
	require.Equal(t, "Foobar Service", service.Name)
	require.Equal(t, "test", service.Category)
	require.Equal(t, []string{"foo", "bar"}, service.SupportedBoards)

	compose, ok := service.GetComposeFile()
	require.True(t, ok)
	require.Equal(t, paths.New("testdata", "services", "arduino", "foobar", "service_compose.yaml").String(), compose.String())
}

func TestLoadServicesSupportedBoard(t *testing.T) {
	service1 := Service{ServiceID: "arduino:bar"}
	service2 := Service{ServiceID: "arduino:foo"}
	service3 := Service{ServiceID: "arduino:foobar"}

	tests := []struct {
		name         string
		platform     platform.Platform
		wantServices []Service
	}{
		{
			name:         "all services supported when no board specified",
			platform:     platform.Platform{BoardName: ""},
			wantServices: []Service{service1, service2, service3},
		},
		{
			name:         "all bar services and services without supported board specified",
			platform:     platform.Platform{BoardName: "bar"},
			wantServices: []Service{service1, service2, service3},
		},
		{
			name:         "only foo services and services without supported board specified",
			platform:     platform.Platform{BoardName: "foo"},
			wantServices: []Service{service2, service3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			servicesIndex, err := Load(tt.platform, paths.New("testdata/services"))
			require.NoError(t, err)

			for i := range servicesIndex.Services {
				require.Equal(t, tt.wantServices[i].ServiceID, servicesIndex.Services[i].ServiceID)
			}
		})
	}
}

func TestServicePorts(t *testing.T) {
	testCases := []struct {
		name           string
		composeContent string
		want           []string
	}{
		{
			name: "ports are extracted from the service compose file",
			composeContent: `
services:
  influx:
    image: influxdb:2.7
    ports:
      - "127.0.0.1:8086:8086"
  proxy:
    image: nginx
    ports:
      - "8085"
`,
			want: []string{"8085", "8086"},
		},
		{
			name: "ports declared more than once are deduplicated",
			composeContent: `
services:
  a:
    image: nginx
    ports: ["9000:9000"]
  b:
    image: nginx
    ports: ["9000:9000"]
`,
			want: []string{"9000"},
		},
		{
			name: "a service publishing nothing has no ports",
			composeContent: `
services:
  worker:
    image: busybox
`,
			want: nil,
		},
		{
			name:           "an unreadable compose file is skipped without failing the load",
			composeContent: "services\n  broken:\n",
			want:           nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			servicePath := paths.New(t.TempDir())
			require.NoError(t, servicePath.Join("service_config.yaml").WriteFile([]byte("service_id: arduino:test\nname: Test Service\n")))
			require.NoError(t, servicePath.Join("service_compose.yaml").WriteFile([]byte(tc.composeContent)))

			service, err := load(platform.GetPlatform(nil), servicePath)
			require.NoError(t, err)
			require.Equal(t, tc.want, service.GetPorts())
		})
	}
}
