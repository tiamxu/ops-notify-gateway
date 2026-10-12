package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestGatewayListenFailureExitsNonzero(t *testing.T) {
	if os.Getenv("NOTIFY_TEST_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := fmt.Sprintf("server:\n  address: %s\nauth:\n  enabled: false\nchannels:\n  local:\n    platform: feishu\n    webhook: unused\n", listener.Addr().String())
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGatewayListenFailureExitsNonzero$")
	cmd.Env = append(os.Environ(), "NOTIFY_TEST_MAIN=1", "CONFIG_FILE="+path)
	err = cmd.Run()
	if ctx.Err() != nil {
		t.Fatal("gateway did not terminate on listen failure")
	}
	if err == nil {
		t.Fatal("listen failure exited successfully")
	}
}
