package cmd

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/bnema/bnetctl/internal/domain"
)

// TestPrintStopResult pins the output of `bnetctl kill`, which is the only evidence
// a user gets that the command did something.
func TestPrintStopResult(t *testing.T) {
	tests := []struct {
		name     string
		result   domain.StopResult
		verified bool
		want     []string
		absent   []string
	}{
		{
			name:     "wineserver stopped",
			result:   domain.StopResult{WineserverStopped: true},
			verified: true,
			want:     []string{"stopped the wineserver"},
			absent:   []string{"nothing was running"},
		},
		{
			name:     "leftover killed",
			result:   domain.StopResult{Leftovers: []domain.StoppedProcess{{PID: 42, Name: "explorer.exe"}}},
			verified: true,
			want:     []string{"killed leftover explorer.exe (pid 42)"},
			absent:   []string{"nothing was running"},
		},
		{
			name:     "idle prefix",
			result:   domain.StopResult{},
			verified: true,
			want:     []string{"nothing was running"},
			absent:   []string{"stopped the wineserver", "killed leftover"},
		},
		{
			name:     "unverified stop claims nothing",
			result:   domain.StopResult{},
			verified: false,
			absent:   []string{"nothing was running", "stopped the wineserver", "killed leftover"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := captureStdout(t, func() { printStopResult(test.result, test.verified) })

			for _, want := range test.want {
				if !strings.Contains(out, want) {
					t.Errorf("output %q does not contain %q", out, want)
				}
			}
			for _, absent := range test.absent {
				if strings.Contains(out, absent) {
					t.Errorf("output %q should not contain %q", out, absent)
				}
			}
		})
	}
}

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	original := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = original }()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
