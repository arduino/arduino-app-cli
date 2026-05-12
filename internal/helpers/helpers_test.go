// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package helpers

import (
	"net"
	"testing"

	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
)

func TestGetHostIP(t *testing.T) {
	ip, err := GetHostIP()
	if err != nil {
		t.Fatalf("GetHostIP returned an error: %v", err)
	}
	if ip == "" {
		t.Fatal("GetHostIP returned an empty string")
	}
	t.Logf("GetHostIP returned: %s", ip)
}

func TestIPv4FromAddr(t *testing.T) {
	tests := []struct {
		name string
		addr net.Addr
		want string
	}{
		{
			name: "ipv4 from IPNet",
			addr: &net.IPNet{IP: net.ParseIP("192.168.1.10")},
			want: "192.168.1.10",
		},
		{
			name: "ipv4 from IPAddr",
			addr: &net.IPAddr{IP: net.ParseIP("10.0.0.15")},
			want: "10.0.0.15",
		},
		{
			name: "ignore loopback",
			addr: &net.IPNet{IP: net.ParseIP("127.0.0.1")},
		},
		{
			name: "ignore ipv6",
			addr: &net.IPNet{IP: net.ParseIP("2001:db8::1")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ipv4FromAddr(tt.addr)
			if tt.want == "" {
				if got != nil {
					t.Fatalf("ipv4FromAddr() = %v, want nil", got)
				}
				return
			}

			if got == nil || got.String() != tt.want {
				t.Fatalf("ipv4FromAddr() = %v, want %s", got, tt.want)
			}
		})
	}
}

func TestArduinoCLIDownloadProgressToString(t *testing.T) {
	tests := []struct {
		name     string
		input    *rpc.DownloadProgress
		expected string
	}{
		{
			name: "Download Start",
			input: &rpc.DownloadProgress{
				Message: &rpc.DownloadProgress_Start{
					Start: &rpc.DownloadProgressStart{
						Url: "http://example.com/index.json",
					},
				},
			},
			expected: "Download started: http://example.com/index.json",
		},
		{
			name: "Download Update",
			input: &rpc.DownloadProgress{
				Message: &rpc.DownloadProgress_Update{
					Update: &rpc.DownloadProgressUpdate{
						Downloaded: 1048576,
						TotalSize:  5242880,
					},
				},
			},
			expected: "Download progress: 1.00MiB / 5.00MiB",
		},
		{
			name: "Download End Success",
			input: &rpc.DownloadProgress{
				Message: &rpc.DownloadProgress_End{
					End: &rpc.DownloadProgressEnd{
						Success: true,
						Message: "Done",
					},
				},
			},
			expected: "Download completed",
		},
		{
			name: "Download End Failure",
			input: &rpc.DownloadProgress{
				Message: &rpc.DownloadProgress_End{
					End: &rpc.DownloadProgressEnd{
						Success: false,
						Message: "Network error",
					},
				},
			},
			expected: "Download failed: Network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ArduinoCLIDownloadProgressToString(tt.input)
			if res != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, res)
			}
		})
	}
}

func TestArduinoCLITaskProgressToString(t *testing.T) {
	tests := []struct {
		name     string
		input    *rpc.TaskProgress
		expected string
	}{
		{
			name: "Task Running",
			input: &rpc.TaskProgress{
				Name:    "Install",
				Percent: 50.0,
			},
			expected: "Install: 50%",
		},
		{
			name: "Task With Message",
			input: &rpc.TaskProgress{
				Name:    "Unpacking",
				Message: "Extracting...",
				Percent: 10.0,
			},
			expected: "Unpacking (Extracting...): 10%",
		},
		{
			name: "Task Completed",
			input: &rpc.TaskProgress{
				Name:      "Install",
				Completed: true,
			},
			expected: "Install: completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := ArduinoCLITaskProgressToString(tt.input)
			if res != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, res)
			}
		})
	}
}
