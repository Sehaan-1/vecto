package e2e_test

// Remote round-trip through both real binaries (ADR-0013):
// cold run uploads to the stub, local cache is wiped, hot run restores
// everything from remote with zero command re-execution.

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		t.Fatalf("cannot resolve repo root: %v", err)
	}
	return root
}

func buildBinary(t *testing.T, root, pkg, out string) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not on PATH")
	}
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building %s: %v\n%s", pkg, err, out)
	}
}

func waitForServerFile(t *testing.T, dir string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
					return
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for upload in %s", dir)
}

func runVecto(t *testing.T, vectoBin, projectDir, remoteURL string) (string, int) {
	t.Helper()
	cmd := exec.Command(vectoBin, "run", "build")
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), "VECTO_REMOTE_CACHE_URL="+remoteURL)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("running vecto: %v", err)
		}
	}
	return string(out), code
}

func TestE2E_RemoteCache_ColdCleanHot(t *testing.T) {
	root := repoRoot(t)

	binDir := t.TempDir()
	vectoBin := filepath.Join(binDir, "vecto")
	serverBin := filepath.Join(binDir, "vecto-server")
	if runtime.GOOS == "windows" {
		vectoBin += ".exe"
		serverBin += ".exe"
	}
	buildBinary(t, root, "./cmd/vecto", vectoBin)
	buildBinary(t, root, "./cmd/vecto-server", serverBin)

	// Sample project: gen.go writes the artifact and appends one line to
	// runs.log per execution. runs.log is NOT a cached output, so its line
	// count proves how many times the command really ran.
	projectDir := t.TempDir()
	manifest := `version: "1"
tasks:
  build:
    command: "go run gen.go"
    inputs: ["gen.go"]
    outputs: ["dist/out.txt"]
`
	gen := `package main

import "os"

func main() {
	os.MkdirAll("dist", 0755)
	os.WriteFile("dist/out.txt", []byte("remote artifact v1\n"), 0644)
	f, _ := os.OpenFile("runs.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	f.WriteString("ran\n")
	f.Close()
}
`
	if err := os.WriteFile(filepath.Join(projectDir, "vecto.yaml"), []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "gen.go"), []byte(gen), 0644); err != nil {
		t.Fatal(err)
	}

	// Start the stub on an ephemeral port; it prints the bound address.
	serverDir := t.TempDir()
	serverCmd := exec.Command(serverBin, "--addr", "127.0.0.1:0", "--dir", serverDir)
	serverOut, err := serverCmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	serverCmd.Stderr = os.Stderr
	if err := serverCmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { serverCmd.Process.Kill() })

	addrCh := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(serverOut).ReadString('\n')
		addrCh <- line
	}()
	var addrLine string
	select {
	case addrLine = <-addrCh:
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for server startup line")
	}
	m := regexp.MustCompile(`listening on (\S+)`).FindStringSubmatch(addrLine)
	if m == nil {
		t.Fatalf("cannot parse server address from %q", addrLine)
	}
	remoteURL := "http://" + m[1]

	// 1. Cold: executes the command and uploads the entry.
	coldOut, coldCode := runVecto(t, vectoBin, projectDir, remoteURL)
	if coldCode != 0 {
		t.Fatalf("cold run failed (exit %d):\n%s", coldCode, coldOut)
	}
	waitForServerFile(t, serverDir, 30*time.Second)

	// 2. Clean local: wipe cache and outputs, keep runs.log as the witness.
	if err := os.RemoveAll(filepath.Join(projectDir, ".vecto")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(projectDir, "dist")); err != nil {
		t.Fatal(err)
	}

	// 3. Hot-from-remote: everything restores, nothing re-executes.
	hotOut, hotCode := runVecto(t, vectoBin, projectDir, remoteURL)
	if hotCode != 0 {
		t.Fatalf("hot run failed (exit %d):\n%s", hotCode, hotOut)
	}
	if !strings.Contains(hotOut, "CACHED] build") {
		t.Fatalf("hot run did not restore build from remote:\n%s", hotOut)
	}
	restored, err := os.ReadFile(filepath.Join(projectDir, "dist", "out.txt"))
	if err != nil || string(restored) != "remote artifact v1\n" {
		t.Fatalf("artifact not restored from remote: %v %q", err, restored)
	}
	runsLog, err := os.ReadFile(filepath.Join(projectDir, "runs.log"))
	if err != nil {
		t.Fatalf("witness file missing: %v", err)
	}
	if strings.Count(string(runsLog), "ran\n") != 1 {
		t.Fatalf("command re-executed on hot run, runs.log:\n%s", runsLog)
	}
}
