package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/matryer/lcars/internal/content"
)

func TestArgumentsBeforeNativeStartup(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"--unknown"}, {"unexpected"}, {"--data", filepath.Join(t.TempDir(), "missing")}} {
		var out bytes.Buffer
		err := run(args, &out)
		if err == nil {
			t.Fatalf("%v should stop before opening a native window", args)
		}
		if args[0] == "--help" && !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("help: %v", err)
		}
	}
}

func TestDataDirectory(t *testing.T) {
	// XDG_CONFIG_HOME controls UserConfigDir on Linux; platform-native CI also
	// verifies explicit paths without touching the user's real config folder.
	explicit := filepath.Join(t.TempDir(), "missing")
	root, err := dataDirectory(explicit)
	if err != nil || root != explicit {
		t.Fatalf("explicit directory: %q, %v", root, err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("an explicit missing directory must not be silently created")
	}
	if _, err := content.New(root); err == nil {
		t.Fatal("missing explicit data must be rejected")
	}
}

func TestDefaultDataDirectory(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("XDG_CONFIG_HOME is Linux-specific")
	}
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	want := filepath.Join(config, "lcars", "content")
	for range 2 {
		root, err := dataDirectory("")
		if err != nil || root != want {
			t.Fatalf("default directory: %q, %v", root, err)
		}
		if _, err := content.New(root); err != nil {
			t.Fatal(err)
		}
	}
}
