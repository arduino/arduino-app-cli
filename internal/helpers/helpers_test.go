// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package helpers

import (
	"net"
	"testing"
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
