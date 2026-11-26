// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

package helpers

import (
	"fmt"
	"net"
	"path"
	"strconv"

	rpc "github.com/arduino/arduino-cli/rpc/cc/arduino/cli/commands/v1"
)

// ArduinoCLIDownloadProgressToString states the file a download gets, and its failure.
// The chunks and a success are empty.
func ArduinoCLIDownloadProgressToString(progress *rpc.DownloadProgress) string {
	switch {
	case progress.GetStart() != nil:
		return "Downloading " + path.Base(progress.GetStart().GetUrl())
	case progress.GetUpdate() != nil:
		return ""
	case progress.GetEnd() != nil:
		end := progress.GetEnd()
		if !end.GetSuccess() {
			return fmt.Sprintf("Download failed: %s", end.GetMessage())
		}
		return ""
	}
	return progress.String()
}

// ArduinoCLITaskProgressToString states a task; a task with no name is empty.
func ArduinoCLITaskProgressToString(progress *rpc.TaskProgress) string {
	if progress.GetName() == "" && progress.GetMessage() == "" {
		return ""
	}
	name := progress.GetName()
	if msg := progress.GetMessage(); msg != "" {
		name += fmt.Sprintf(" (%s)", msg)
	}

	if progress.GetCompleted() {
		return fmt.Sprintf("%s: completed", name)
	}
	// Not every task reports a percentage: a platform install, for one, never sets
	// it. Printing a bare 0% for the whole task would be misleading.
	if progress.GetPercent() == 0 {
		return name
	}
	return fmt.Sprintf("%s: %.0f%%", name, progress.GetPercent())
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
