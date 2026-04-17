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
