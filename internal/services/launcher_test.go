package services

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/log"

	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/ports"
)

// fakeRuntime implements only the RuntimePort methods used by LaunchService.
// The embedded interface panics if an unexpected method is called.
type fakeRuntime struct {
	ports.RuntimePort
	order []string
}

func (f *fakeRuntime) Detect() (*domain.WineRuntime, error) {
	return &domain.WineRuntime{WineBin: "/usr/bin/wine"}, nil
}

func (f *fakeRuntime) IsProcessRunning(string) bool { return false }

func (f *fakeRuntime) DisableSystray(string) error {
	f.order = append(f.order, "disable-systray")
	return nil
}

func (f *fakeRuntime) RunExeAsync(string, string, *domain.WineEnv, ...string) (<-chan error, error) {
	f.order = append(f.order, "run-exe")
	done := make(chan error, 1)
	done <- nil
	return done, nil
}

func (f *fakeRuntime) GracefulKillPrefix(string, time.Duration) error { return nil }

// fakeFilesystem implements only the FilesystemPort methods used by LaunchService.
type fakeFilesystem struct {
	ports.FilesystemPort
}

func (fakeFilesystem) Exists(string) bool                          { return true }
func (fakeFilesystem) MkdirAll(string, os.FileMode) error          { return nil }
func (fakeFilesystem) ReadFile(string) ([]byte, error)             { return nil, os.ErrNotExist }
func (fakeFilesystem) WriteFile(string, []byte, os.FileMode) error { return nil }

// TestLaunchDisablesSystrayBeforeStartingBattleNet guards the fix for the stray
// systray window: the setting must be applied on every launch, not only when a
// prefix is created, and before Battle.net registers its tray icon.
func TestLaunchDisablesSystrayBeforeStartingBattleNet(t *testing.T) {
	runtime := &fakeRuntime{}
	svc := NewLaunchService(
		runtime,
		fakeFilesystem{},
		&domain.Config{PrefixDir: "/tmp/prefix"},
		func() *domain.WineEnv { return &domain.WineEnv{Vars: map[string]string{}} },
		log.New(io.Discard),
	)

	if _, err := svc.Launch(); err != nil {
		t.Fatalf("Launch: %v", err)
	}

	want := []string{"disable-systray", "run-exe"}
	if len(runtime.order) != len(want) {
		t.Fatalf("expected calls %v, got %v", want, runtime.order)
	}
	for i := range want {
		if runtime.order[i] != want[i] {
			t.Fatalf("expected calls %v, got %v", want, runtime.order)
		}
	}
}
