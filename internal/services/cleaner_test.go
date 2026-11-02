package services

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bnema/bnetctl/internal/adapters/wine"
	"github.com/bnema/bnetctl/internal/adapters/xdg"
	"github.com/bnema/bnetctl/internal/domain"
	"github.com/bnema/bnetctl/internal/ports"
	"github.com/charmbracelet/log"
)

type cleanupFilesystem struct {
	*xdg.Filesystem
	cache    string
	cleaning chan struct{}
	resume   chan struct{}
}

func (f *cleanupFilesystem) Remove(path string) error {
	if path == f.cache {
		close(f.cleaning)
		<-f.resume
	}
	return f.Filesystem.Remove(path)
}

type cleanupDesktop struct{ ports.DesktopPort }

func (cleanupDesktop) RemoveEntry(string) error { return nil }

type contentionWriter struct {
	once    sync.Once
	waiting chan struct{}
}

func (w *contentionWriter) Write(p []byte) (int, error) {
	if strings.Contains(string(p), "waiting for prefix maintenance lock") {
		w.once.Do(func() { close(w.waiting) })
	}
	return len(p), nil
}

func TestCleanAllPreservesLockForWaitingInvocation(t *testing.T) {
	root := t.TempDir()
	cfg := &domain.Config{DataDir: filepath.Join(root, "data"), CacheDir: filepath.Join(root, "cache")}
	cfg.PrefixDir = filepath.Join(cfg.DataDir, "prefix")
	for _, dir := range []string{cfg.PrefixDir, cfg.CacheDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(cfg.DataDir, "icon"), []byte("icon"), 0600); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	for _, name := range []string{"wine", "wineboot", "wineserver"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\nif [ \"$1\" = --version ]; then echo wine-11.21; fi\nexit 0\n"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("BNETCTL_WINE", filepath.Join(bin, "wine"))
	fs := &cleanupFilesystem{Filesystem: xdg.NewFilesystem(), cache: cfg.CacheDir, cleaning: make(chan struct{}), resume: make(chan struct{})}
	var resumeOnce sync.Once
	resume := func() { resumeOnce.Do(func() { close(fs.resume) }) }
	defer resume()
	logger := log.New(io.Discard)
	svc := NewCleanerService(wine.NewAdapter(logger), cleanupDesktop{}, fs, cfg, logger)
	cleaned := make(chan error, 1)
	go func() { cleaned <- svc.CleanAll() }()
	select {
	case <-fs.cleaning:
	case <-time.After(3 * time.Second):
		t.Fatal("cleanup did not start")
	}
	lock := cfg.PrefixDir + ".lock"
	before, err := os.Stat(lock)
	if err != nil {
		t.Fatal(err)
	}
	parentBefore, err := os.Stat(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	writer := &contentionWriter{waiting: make(chan struct{})}
	waiter := wine.NewAdapter(log.New(writer))
	waited := make(chan error, 1)
	go func() {
		waited <- waiter.WithStoppedPrefix(cfg.PrefixDir, 3*time.Second, func() error {
			after, err := os.Stat(lock)
			if err != nil {
				return err
			}
			if !os.SameFile(before, after) {
				t.Error("lock inode replaced")
			}
			return nil
		})
	}()
	select {
	case <-writer.waiting:
	case err := <-waited:
		t.Fatalf("waiter did not contend: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("no observed contention")
	}
	resume()
	if err := <-cleaned; err != nil {
		t.Fatal(err)
	}
	if err := <-waited; err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(lock)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("lock identity lost: %v", err)
	}
	parentAfter, err := os.Stat(cfg.DataDir)
	if err != nil || !os.SameFile(parentBefore, parentAfter) {
		t.Fatalf("lock parent replaced: %v", err)
	}
	entries, err := os.ReadDir(cfg.DataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "prefix.lock" {
		t.Fatalf("unexpected remaining data: %v", entries)
	}
	if _, err := os.Stat(cfg.CacheDir); !os.IsNotExist(err) {
		t.Fatalf("cache remains: %v", err)
	}
}
