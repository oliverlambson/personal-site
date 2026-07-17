package main

import (
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestListenAddress(t *testing.T) {
	t.Run("production default", func(t *testing.T) {
		t.Setenv(addrEnv, "")
		if got := listenAddress(); got != defaultAddr {
			t.Fatalf("listenAddress() = %q, want %q", got, defaultAddr)
		}
	})

	t.Run("explicit development override", func(t *testing.T) {
		const override = "0.0.0.0:2960"
		t.Setenv(addrEnv, override)
		if got := listenAddress(); got != override {
			t.Fatalf("listenAddress() = %q, want %q", got, override)
		}
	})
}

func TestListenerFailureExitsNonZero(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve listener: %v", err)
	}
	defer listener.Close()

	t.Setenv("PERSONAL_SITE_TEST_MAIN", "1")
	t.Setenv(addrEnv, listener.Addr().String())
	cmd := exec.Command(os.Args[0], "-test.run=^TestMainHelperProcess$")
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("listener failure process exited successfully; output: %s", output)
	}
	exitError, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("listener failure error = %T %v, want *exec.ExitError", err, err)
	}
	if exitError.ExitCode() == 0 {
		t.Fatalf("listener failure exit code = 0; output: %s", output)
	}
	if !strings.Contains(string(output), "address already in use") {
		t.Fatalf("listener failure output = %q, want address error", output)
	}
}

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("PERSONAL_SITE_TEST_MAIN") != "1" {
		return
	}
	main()
}
