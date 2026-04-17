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
	"fmt"
	"net"
	"strconv"

	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
)

func ArduinoCLIDownloadProgressToString(progress *rpc.DownloadProgress) string {
	switch {
	case progress.GetStart() != nil:
		return fmt.Sprintf("Download started: %s", progress.GetStart().GetUrl())
	case progress.GetUpdate() != nil:
		return fmt.Sprintf("Download progress: %s", progress.GetUpdate())
	case progress.GetEnd() != nil:
		return fmt.Sprintf("Download completed: %s", progress.GetEnd())
	}
	return progress.String()
}

func ArduinoCLITaskProgressToString(progress *rpc.TaskProgress) string {
	data := fmt.Sprintf("Task %s:", progress.GetName())
	if progress.GetMessage() != "" {
		data += fmt.Sprintf(" (%s)", progress.GetMessage())
	}
	if progress.GetCompleted() {
		data += " completed"
	} else {
		data += fmt.Sprintf(" %.2f%%", progress.GetPercent())
	}
	return data
}

// getDefaultNetworkInterfaceAndIP attempts to determine the IPv4 address of the default network interface
func getDefaultNetworkInterfaceAndIP() (string, error) {
	conn, err := net.Dial("udp4", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || localAddr.IP == nil {
		return "", fmt.Errorf("unable to determine local address")
	}

	localIP := localAddr.IP.To4()
	if localIP == nil {
		return "", fmt.Errorf("default route does not use an IPv4 address")
	}

	return localIP.String(), nil
}

func GetHostIP() (string, error) {
	if ip, err := getDefaultNetworkInterfaceAndIP(); err == nil {
		return ip, nil
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	// Collect IP Addresses from all running, non-loopback interfaces
	found := map[string]string{}
	for _, iface := range ifaces {
		// Skip interfaces that are not running or are loopback
		if iface.Flags&net.FlagRunning == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			// Filter all non-loopback IPv4 addresses
			if ip := ipv4FromAddr(addr); ip != nil {
				found[iface.Name] = ip.String()
				break
			}
		}
	}

	// Prefer known interface names like "eth0" or "wlan0"
	if ip, ok := found["eth0"]; ok {
		return ip, nil
	}
	if ip, ok := found["wlan0"]; ok {
		return ip, nil
	}

	// If no known interfaces, return the first found IP address
	for _, ip := range found {
		return ip, nil
	}

	// If no IP address found, return an error
	return "", fmt.Errorf("no IP address found")
}

func ipv4FromAddr(addr net.Addr) net.IP {
	switch value := addr.(type) {
	case *net.IPNet:
		if value.IP.IsLoopback() {
			return nil
		}
		return value.IP.To4()
	case *net.IPAddr:
		if value.IP.IsLoopback() {
			return nil
		}
		return value.IP.To4()
	default:
		return nil
	}
}

func ToHumanMiB(bytes int64) string {
	return strconv.FormatFloat(float64(bytes)/(1024.0*1024.0), 'f', 2, 64) + "MiB"
}
