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
			name: "wineserver and its clients",
			result: domain.StopResult{
				Wineserver: &domain.StoppedProcess{PID: 9, Name: "wineserver"},
				Session: domain.StoppedProcesses{
					{PID: 42, Name: "Battle.net.exe"},
					{PID: 43, Name: "explorer.exe"},
				},
			},
			verified: true,
			want: []string{
				"stopped wineserver (pid 9)",
				"stopped 2 wine processes: Battle.net.exe (pid 42), explorer.exe (pid 43)",
			},
			absent: []string{"nothing was running"},
		},
		{
			name:     "single client",
			result:   domain.StopResult{Session: domain.StoppedProcesses{{PID: 7, Name: "notepad.exe"}}},
			verified: true,
			want:     []string{"stopped 1 wine process: notepad.exe (pid 7)"},
			absent:   []string{"wine processes"},
		},
		{
			name:     "leftovers without a wineserver",
			result:   domain.StopResult{Leftovers: domain.StoppedProcesses{{PID: 42, Name: "explorer.exe"}}},
			verified: true,
			want:     []string{"killed explorer.exe (pid 42)"},
			absent:   []string{"nothing was running", "stopped wineserver"},
		},
		{
			name:     "idle prefix",
			result:   domain.StopResult{},
			verified: true,
			want:     []string{"nothing was running"},
			absent:   []string{"stopped wineserver", "killed "},
		},
		{
			name:     "unverified stop claims nothing",
			result:   domain.StopResult{},
			verified: false,
			absent:   []string{"nothing was running", "stopped wineserver", "killed "},
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
