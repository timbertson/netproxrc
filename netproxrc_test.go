package main
import "testing"
import "github.com/stretchr/testify/require"
import "os/exec"
import "os"
import "io"
import "fmt"

// HTTP auth header for `test@secret` in ./test/netrc
const testAuthHeader = "Basic dGVzdDpzZWNyZXQ="

func commandStdout(t *testing.T, cmd ...string) string {
	output := ""
	config := makeConfig(cmd)
	t.Logf("exec: %+v", config)
	success, err := WithProc(makeConfig(cmd), func(proc *exec.Cmd) (bool, error) {
		proc.Stderr = os.Stderr // TODO pipe and goroutine
		proc.Stdout = nil
		pipe, err := proc.StdoutPipe(); if err != nil {
			t.Fatalf("pipe failed: %v", err)
		}
		err = proc.Start(); if err != nil {
			t.Fatalf("start failed: %v", err)
		}
		buf, err := io.ReadAll(pipe); if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		err = proc.Wait()
		output = string(buf)
		t.Logf("[exec] %s", output)
		return true, err
	})
	if err != nil {
		t.Fatalf("Error running command: %+v", err)
	}
	if !success {
		t.Fatal("Error running command (nonzero return)")
	}

	return output
}

func makeConfig(cmd []string) Config {
	return Config {
		verbose: true,
		port: 0,
		listenIface: "localhost",
		netrcPath: "test/netrc",
		cmd: cmd,
	}
}

func commandStderr(t *testing.T, cmd ...string) string {
	output := ""
	config := makeConfig(cmd)
	t.Logf("exec: %+v", config)
	success, err := WithProc(makeConfig(cmd), func(proc *exec.Cmd) (bool, error) {
		proc.Stdout = os.Stderr // TODO pipe and goroutine
		proc.Stderr = nil
		pipe, err := proc.StderrPipe(); if err != nil {
			t.Fatalf("pipe failed: %v", err)
		}
		err = proc.Start(); if err != nil {
			t.Fatalf("start failed: %v", err)
		}
		buf, err := io.ReadAll(pipe); if err != nil {
			t.Fatalf("read failed: %v", err)
		}
		_ = proc.Wait() // allow failure as we're testing stderr
		output = string(buf)
		t.Logf("[exec] %s", output)
		return true, nil
	})
	if err != nil {
		t.Fatalf("Error running command: %+v", err)
	}
	if !success {
		t.Fatal("Error running command (nonzero return)")
	}

	return output
}

func expectHashMismatch(t *testing.T, expr string) {
	output := commandStderr(t,
		"nix-build", "--no-out-link",
		"--expr", fmt.Sprintf("with (import <nixpkgs> {}); %s", expr),
	)
	require.Equal(t, "TODO", output)
}

func TestCurl(t *testing.T) {
	// expectHashMismatch(t, "true")
	output := commandStdout(t, "curl", "-sSL", "https://httpbin.org/headers")
	require.Contains(t, output, testAuthHeader)
}
