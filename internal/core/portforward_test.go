// SPDX-License-Identifier: Apache-2.0

package core

import (
	"net"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// validatePortForwardPorts
// ---------------------------------------------------------------------------

func TestValidatePortForwardPorts_ValidRange(t *testing.T) {
	cases := []struct {
		name       string
		localPort  int
		remotePort int
	}{
		{"minimum valid", 1024, 1},
		{"maximum valid", 65535, 65535},
		{"typical kubectl range", 8080, 8080},
		{"high ephemeral", 50000, 9090},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validatePortForwardPorts(tc.localPort, tc.remotePort); err != nil {
				t.Errorf("expected no error for localPort=%d remotePort=%d, got: %v",
					tc.localPort, tc.remotePort, err)
			}
		})
	}
}

func TestValidatePortForwardPorts_LocalPortPrivileged(t *testing.T) {
	privileged := []int{0, 1, 80, 443, 1023}

	for _, port := range privileged {
		t.Run("", func(t *testing.T) {
			err := validatePortForwardPorts(port, 8080)
			if err == nil {
				t.Errorf("expected error for privileged localPort=%d, got nil", port)
			}
			if !strings.Contains(err.Error(), "out of range") {
				t.Errorf("expected 'out of range' in error for localPort=%d, got: %v", port, err)
			}
		})
	}
}

func TestValidatePortForwardPorts_LocalPortTooHigh(t *testing.T) {
	err := validatePortForwardPorts(65536, 8080)
	if err == nil {
		t.Fatal("expected error for localPort=65536, got nil")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("expected 'out of range' in error, got: %v", err)
	}
}

func TestValidatePortForwardPorts_RemotePortZero(t *testing.T) {
	err := validatePortForwardPorts(8080, 0)
	if err == nil {
		t.Fatal("expected error for remotePort=0, got nil")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("expected 'out of range' in error, got: %v", err)
	}
}

func TestValidatePortForwardPorts_RemotePortTooHigh(t *testing.T) {
	err := validatePortForwardPorts(8080, 65536)
	if err == nil {
		t.Fatal("expected error for remotePort=65536, got nil")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("expected 'out of range' in error, got: %v", err)
	}
}

func TestValidatePortForwardPorts_ReservedLocalPorts(t *testing.T) {
	cases := []struct {
		name    string
		port    int
		service string
	}{
		{"Vite dev server", 5173, "Vite dev server"},
		{"Wails IPC bridge primary", 34115, "Wails IPC bridge"},
		{"Wails IPC bridge secondary", 34116, "Wails IPC bridge (secondary)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePortForwardPorts(tc.port, 8080)
			if err == nil {
				t.Fatalf("expected conflict error for localPort=%d (%s), got nil",
					tc.port, tc.service)
			}
			if !strings.Contains(err.Error(), "conflicts with") {
				t.Errorf("expected 'conflicts with' in error, got: %v", err)
			}
			if !strings.Contains(err.Error(), tc.service) {
				t.Errorf("expected service name %q in error, got: %v", tc.service, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// checkLocalPortAvailable
// ---------------------------------------------------------------------------

func TestCheckLocalPortAvailable_FreePort(t *testing.T) {
	// Grab an OS-assigned free port number, release it, then verify our
	// function agrees the port is available.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not open test listener: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close() // release before probing

	if err := checkLocalPortAvailable(port); err != nil {
		t.Errorf("expected port %d to be free, got: %v", port, err)
	}
}

func TestCheckLocalPortAvailable_PortInUse(t *testing.T) {
	// Bind a real listener on loopback, then confirm our check detects it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("could not open test listener: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port

	err = checkLocalPortAvailable(port)
	if err == nil {
		t.Fatalf("expected error for in-use port %d, got nil", port)
	}
	if !strings.Contains(err.Error(), "already in use") {
		t.Errorf("expected 'already in use' in error, got: %v", err)
	}
}
