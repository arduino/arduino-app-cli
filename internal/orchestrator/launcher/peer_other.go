// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !linux

package launcher

import "net"

// socketPeerPID cannot tell the peer here: the daemon runs on Linux boards, this is development only.
func socketPeerPID(net.Conn) (int, error) {
	return 0, nil
}
