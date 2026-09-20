package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_NoArgs_PrintsUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit 0 for no args, got %d", code)
	}
	if !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", out.String())
	}
}

func TestRun_Version(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"version"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit 0 for version, got %d", code)
	}
	if !strings.Contains(out.String(), "vecto version") {
		t.Fatalf("expected version output, got %q", out.String())
	}
}

func TestRun_Help(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"help"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit 0 for help, got %d", code)
	}
	if !strings.Contains(out.String(), "Commands:") {
		t.Fatalf("expected commands output, got %q", out.String())
	}
}

func TestRun_CleanBadFlag_ReturnsTwo(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"clean", "--no-such-flag"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("expected exit 2 for bad flag, got %d", code)
	}
}

// Unknown task without a vecto.yaml must return non-zero WITHOUT exiting
// the test process — this is the whole point of run() int (ADR-0011).
func TestRun_MissingConfig_DoesNotExit(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"run", "definitely-not-a-task-xyz"}, &out, &errOut)
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing config, got 0")
	}
}
