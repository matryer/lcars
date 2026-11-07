package main

import (
	"bytes"
	"github.com/matryer/lcars/internal/buildinfo"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandHelp(t *testing.T) {
	var out, errs bytes.Buffer
	if err := run([]string{"--help"}, &out, &errs); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "lcars snapshot --data") {
		t.Fatal("unified snapshot command missing from help")
	}
	if err := run([]string{"not-a-command"}, &out, &errs); err == nil {
		t.Fatal("unknown command accepted")
	}
}

func TestSnapshotValidatesBeforeStartingBrowser(t *testing.T) {
	var out, errs bytes.Buffer
	root := t.TempDir()
	tab := filepath.Join(root, "overview")
	if err := os.Mkdir(tab, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tab, "bad.html"), []byte(`<meta name="lcars:size" content="impossible">`), 0600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "captures")
	err := run([]string{"snapshot", "--data", root, "--out", output, "--browser", "/no/such/browser"}, &out, &errs)
	if err == nil {
		t.Fatal("snapshot accepted invalid content")
	}
	if strings.Contains(err.Error(), "browser") {
		t.Fatalf("browser launched before validation: %v", err)
	}
	if _, err = os.Stat(output); !os.IsNotExist(err) {
		t.Fatalf("created output before validation: %v", err)
	}
}

func TestScreenshotRequiresPanelForExpectedHash(t *testing.T) {
	var out, errs bytes.Buffer
	err := run([]string{"screenshot", "--url", "http://example.test", "--expect-data-hash", "missing"}, &out, &errs)
	if err == nil || !strings.Contains(err.Error(), "require --panel") {
		t.Fatalf("got %v", err)
	}
}

func TestSkillDiscoveryAndExport(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--help"}, "lcars skill"},
		{[]string{"skill"}, "name: lcars-authoring"},
		{[]string{"skill", "--guide"}, "# Authoring LCARS panels"},
		{[]string{"skill", "--starter"}, "LCARS.onData"},
	} {
		var out, errs bytes.Buffer
		if err := run(tc.args, &out, &errs); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), tc.want) {
			t.Errorf("%v: missing %q", tc.args, tc.want)
		}
	}
	dest := filepath.Join(t.TempDir(), "lcars-authoring")
	var out, errs bytes.Buffer
	if err := run([]string{"skill", "--out", dest}, &out, &errs); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"SKILL.md", "references/guide.md", "assets/starter.html"} {
		exported, err := os.ReadFile(filepath.Join(dest, name))
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := os.ReadFile(filepath.Join("../../skills/lcars-authoring", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(exported, canonical) {
			t.Errorf("export changed %s", name)
		}
	}
	if err := run([]string{"skill", "--out", dest}, &out, &errs); err == nil {
		t.Fatal("overwrote existing skill directory")
	}
	if err := run([]string{"skill", "--guide", "--starter"}, &out, &errs); err == nil {
		t.Fatal("accepted conflicting skill options")
	}
}

func TestBuildVersion(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--version"}, {"version", "--json"}} {
		var out, errs bytes.Buffer
		if err := run(args, &out, &errs); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), buildinfo.Current().Version) || !strings.Contains(out.String(), "1.0") {
			t.Fatalf("missing build/contract version: %s", &out)
		}
	}
}

func TestSubcommandHelpSucceeds(t *testing.T) {
	for _, command := range []string{"serve", "validate", "screenshot", "snapshot", "skill", "version"} {
		var out, errs bytes.Buffer
		if err := run([]string{command, "--help"}, &out, &errs); err != nil {
			t.Errorf("%s help: %v", command, err)
		}
	}
}

func TestServeBindFailureDoesNotRunScripts(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python unavailable")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	root := t.TempDir()
	dir := filepath.Join(root, "overview")
	os.Mkdir(dir, 0700)
	os.WriteFile(filepath.Join(dir, "weather.html"), []byte("<title>Weather</title>"), 0600)
	os.WriteFile(filepath.Join(dir, "weather.1s.py"), []byte(`from pathlib import Path; Path('ran').write_text('bad'); print('{"version":1,"data":42}')`), 0600)
	var out, errs bytes.Buffer
	if err := run([]string{"serve", "--data", root, "--listen", listener.Addr().String(), "--run-scripts", "--script-py", python}, &out, &errs); err == nil {
		t.Fatal("bound occupied listener")
	}
	if strings.Contains(out.String(), "LCARS listening") {
		t.Fatal("reported a server that never started")
	}
	if _, err := os.Stat(filepath.Join(dir, "ran")); !os.IsNotExist(err) {
		t.Fatal("ran scripts despite bind failure")
	}
}
