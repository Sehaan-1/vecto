package ui

import (
	"fmt"
	"io"
	"os"
)

// Terminal handles formatting and output display.
type Terminal struct {
	Writer io.Writer
	IsTTY  bool
}

// New creates a new Terminal instance.
func New() *Terminal {
	return &Terminal{
		Writer: os.Stdout,
		IsTTY:  true,
	}
}

// PrintBanner prints the Vecto execution header.
func (t *Terminal) PrintBanner(taskCount int) {
	fmt.Fprintf(t.Writer, "Vecto running %d task(s)\n", taskCount)
}
