package main
import "testing"
import "github.com/stretchr/testify/require"
import "os/exec"
import "os"
import "io"
import "fmt"
import "sync"
import "strings"

const realNetrc = "~/.netrc"
const testNetrc = "./test/netrc"

// HTTP auth header for httpbin in ./test/netrc
const testAuthHeader = "Basic dGVzdDpzZWNyZXQ="

// zero hash used for nix derivation testing
const zeroHash = "sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

type CommandOutput struct {
	success bool
	stdout string
	stderr string
}

func spawnReadRoutine(t *testing.T, wg *sync.WaitGroup, linePrefix string, success *bool, dest *string, pipe io.Reader) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf, err := io.ReadAll(pipe); if err != nil {
			t.Logf("read failed: %v", err)
			*success = false
		} else {
			*dest = string(buf)
			logLines(t, linePrefix, *dest)
		}
	}()
}

func logLines(t *testing.T, prefix string, output string) {
	for _, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		t.Logf("%s%s", prefix, line)
	}
}

func commandOutput(t *testing.T, config Config) CommandOutput {
	output := CommandOutput{ success: true }
	t.Logf("exec: %+v", config)

	_, err := WithProc(config, func(proc *exec.Cmd) (bool, error) {
		proc.Stderr = nil
		proc.Stdout = nil
		stdoutPipe, err := proc.StdoutPipe(); if err != nil {
			t.Fatalf("pipe failed: %v", err)
		}
		stderrPipe, err := proc.StderrPipe(); if err != nil {
			t.Fatalf("pipe failed: %v", err)
		}

		// always set nixpkgs-overlays, it's harmless on non-nix tests
		nixPath := os.Getenv("NIX_PATH")
		proc.Env = append(proc.Env, fmt.Sprintf("NIX_PATH=nixpkgs-overlays=nix/overlay.nix:%s", nixPath))

		err = proc.Start(); if err != nil {
			t.Fatalf("start failed: %v", err)
		}
		wg := sync.WaitGroup{}
		spawnReadRoutine(t, &wg, "[stdout] ", &output.success, &output.stdout, stdoutPipe)
		spawnReadRoutine(t, &wg, "[stderr] ", &output.success, &output.stderr, stderrPipe)

		wg.Wait()
		err = proc.Wait()
		if err != nil {
			t.Logf("Note: Command failed: %v", err)
			output.success = false
		} else {
			t.Logf("Command succeeded")
		}
		return true, nil
	})
	if err != nil {
		t.Fatalf("Error running command: %+v", err)
	}

	return output
}

func makeConfig(t *testing.T, netrcPath string, cmd ...string) Config {
	return Config {
		verbose: true,
		port: 0,
		listenIface: "localhost",
		netrcPath: netrcPath,
		cmd: cmd,
		suppressPrintf: true,
		info: func(msg string, args ...interface{}) {
			t.Logf(msg, args...)
		},
	}
}

func commandStdout(t *testing.T, config Config) string {
	output := commandOutput(t, config)
	if !output.success {
		t.Fatal("Command failed")
	}
	return output.stdout
}

func commandStderr(t *testing.T, config Config) string {
	output := commandOutput(t, config)

	// NOTE: we don't check output.success as we expect stderr checking
	// commands to fail
	return output.stderr
}

func expectHashMismatch(t *testing.T, config Config, expr string, expectedHash string) {
	// to ensure we never use a locally cached version, all fetchers
	// should fail by specifying the zero hash. If we get
	// a hash mismatch, that means we successfully downloaded the resource
	config.cmd = []string{
		"nix-build", "--no-out-link", "--show-trace",
		"--expr", fmt.Sprintf("with (import <nixpkgs> {}); %s", expr),
	}

	output := commandStderr(t, config)
	require.Contains(t, output, fmt.Sprintf("specified: %s", zeroHash))
	require.Contains(t, output, expectedHash)
}

func TestCurl(t *testing.T) {
	output := commandStdout(t, makeConfig(t, testNetrc, "curl", "-sSL", "https://httpbin.org/headers"))
	require.Contains(t, output, testAuthHeader)
}

func TestGit(t *testing.T) {
	output := commandStdout(t, makeConfig(t, realNetrc,
		"git", "ls-remote", "https://github.com/timbertson/test-private-repo",
	))
	require.Contains(t, output, "refs/tags/v1")
}

func TestNixFetchGit(t *testing.T) {
	config := makeConfig(t, realNetrc)
	expectHashMismatch(t, config, fmt.Sprintf(`pkgs.fetchgit {
		url = "https://github.com/timbertson/test-private-repo";
		rev = "v1";
		hash = "%s";
	}`, zeroHash), "sha256-qHsiDx2pAy9iNNyeyYXlwS3Qsb5g9DaZikTb3vaIHSs=")
}

func TestNixFetchGoModule(t *testing.T) {
	config := makeConfig(t, realNetrc)
	expectHashMismatch(t, config, fmt.Sprintf(`pkgs.buildGoModule {
		src = ./test/gomod;
		pname = "test";
		version = "test";
		vendorHash = "%s";
	}`, zeroHash), "sha256-/4wHYfaQ8CKjVQwHJt+upE8fxt7AactX4Ca+qNVmgPI=")
}
