// This file is part of arduino-app-cli.
//
// SPDX-FileCopyrightText: Arduino s.r.l. and/or its affiliated companies
// SPDX-License-Identifier: GPL-3.0-or-later

// This file implements just enough of the adb client/server protocol to open
// a binary-clean stdio pipe to a process running on a device, without going
// through `adb shell`.
//
// `adb shell` is not 8-bit-clean on Windows: adb.exe inherits console handles
// from cmd.exe / pwsh and those handles perform line-ending and codepage
// translations that corrupt binary streams. There is no flag combination that
// fully disables this. The workaround is the same one `adb push` / `adb pull`
// use internally: talk to the local adb server (tcp:127.0.0.1:5037) directly
// and use its `exec:` service, which is a raw stdio pipe with no PTY and no
// translation.
//
// The adb wire protocol is text-framed. A request is:
//
//	<4 ASCII hex digits = payload length><payload bytes>
//
// A response starts with a 4-byte status (OKAY or FAIL). On FAIL, a
// length-prefixed error string follows. For per-device services like `exec:`,
// the socket must first be bound to a device with `host:transport:<serial>`;
// after the second OKAY the socket becomes a raw bidirectional pipe.
//
// Reference: https://android.googlesource.com/platform/packages/modules/adb/+/refs/heads/main/SERVICES.TXT
package adb

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/arduino/go-paths-helper"
)

const defaultAdbServerPort = 5037

// handshakeTimeout bounds the small text exchange before the data stream
// takes over.
const handshakeTimeout = 5 * time.Second

// OpenExecStream opens a binary-clean bidirectional stream to a process
// started on the device identified by serial. The returned net.Conn carries
// the remote process' stdin (writes) and stdout+stderr muxed (reads); closing
// it terminates the remote process.
//
// The `exec:` service is used (not `shell:`) because it never allocates a
// PTY and performs no LF/CRLF or codepage translation, so it is 8-bit-clean
// on every OS. See the file-level comment for why this matters on Windows.
func OpenExecStream(adbPath, serial, command string) (net.Conn, error) {
	if err := ensureAdbServerRunning(adbPath); err != nil {
		return nil, fmt.Errorf("failed to start adb server: %w", err)
	}

	conn, err := net.DialTimeout("tcp", adbServerAddress(), handshakeTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to adb server at %s: %w", adbServerAddress(), err)
	}
	handshakeOK := false
	defer func() {
		if !handshakeOK {
			_ = conn.Close()
		}
	}()
	_ = conn.SetDeadline(time.Now().Add(handshakeTimeout))

	// Bind this connection to the target device.
	if err := writeProtocolRequest(conn, "host:transport:"+serial); err != nil {
		return nil, err
	}
	if err := readProtocolStatus(conn); err != nil {
		return nil, fmt.Errorf("failed to attach to device %q: %w", serial, err)
	}

	// Start the remote process on the binary-clean exec: service.
	if err := writeProtocolRequest(conn, "exec:"+command); err != nil {
		return nil, err
	}
	if err := readProtocolStatus(conn); err != nil {
		return nil, fmt.Errorf("failed to start remote process %q on device %q: %w", command, serial, err)
	}

	// The data stream is caller-managed; drop the handshake deadline.
	_ = conn.SetDeadline(time.Time{})
	handshakeOK = true
	return conn, nil
}

// ensureAdbServerRunning makes sure the local adb server is up. The adb
// client binary is still required to start it because it carries the USB
// stack, auth keys handling and device discovery; once running, all per-device
// traffic goes over the TCP socket and never touches the binary again.
func ensureAdbServerRunning(adbPath string) error {
	if conn, err := net.DialTimeout("tcp", adbServerAddress(), time.Second); err == nil {
		_ = conn.Close()
		return nil
	}
	cmd, err := paths.NewProcess(nil, adbPath, "start-server")
	if err != nil {
		return fmt.Errorf("failed to create adb start-server command: %w", err)
	}
	if out, err := cmd.RunAndCaptureCombinedOutput(context.Background()); err != nil {
		return fmt.Errorf("failed to start adb server: %w: %s", err, out)
	}
	return nil
}

// adbServerAddress returns the TCP address of the local adb server, honoring
// ANDROID_ADB_SERVER_PORT (same as the upstream adb client).
func adbServerAddress() string {
	port := defaultAdbServerPort
	if v := os.Getenv("ANDROID_ADB_SERVER_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			port = p
		}
	}
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// writeProtocolRequest writes a single request frame: 4 ASCII hex digits with
// the payload length, followed by the payload. The 16-bit cap is part of the
// adb protocol.
func writeProtocolRequest(w io.Writer, payload string) error {
	if len(payload) > 0xFFFF {
		return fmt.Errorf("adb protocol message too long: %d bytes (max %d)", len(payload), 0xFFFF)
	}
	if _, err := fmt.Fprintf(w, "%04x%s", len(payload), payload); err != nil {
		return fmt.Errorf("failed to write adb protocol request: %w", err)
	}
	return nil
}

// readProtocolString reads a length-prefixed string.
func readProtocolString(r io.Reader) (string, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return "", err
	}
	length, err := strconv.ParseUint(string(header), 16, 16)
	if err != nil {
		return "", fmt.Errorf("invalid adb length prefix %q: %w", header, err)
	}
	if length == 0 {
		return "", nil
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// readProtocolStatus reads the 4-byte OKAY/FAIL status. On FAIL it also
// consumes the trailing error message and surfaces it as an error.
func readProtocolStatus(r io.Reader) error {
	status := make([]byte, 4)
	if _, err := io.ReadFull(r, status); err != nil {
		return fmt.Errorf("failed to read adb status: %w", err)
	}
	switch string(status) {
	case "OKAY":
		return nil
	case "FAIL":
		msg, err := readProtocolString(r)
		if err != nil {
			return fmt.Errorf("adb returned FAIL but failed to read error message: %w", err)
		}
		return fmt.Errorf("adb server replied FAIL: %s", msg)
	default:
		return fmt.Errorf("unexpected adb status %q (expected OKAY or FAIL)", status)
	}
}
