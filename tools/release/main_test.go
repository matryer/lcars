package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestBump(t *testing.T) {
	for _, tc := range []struct{ current, bump, want string }{
		{"0.1.9", "patch", "0.1.10"}, {"1.2.9", "minor", "1.3.0"}, {"1.2.9", "major", "2.0.0"}, {"0.1.0", "", "0.1.0"}, {"0.1.0", "none", "0.1.0"},
	} {
		got, err := nextVersion(tc.current, tc.bump)
		if err != nil || got != tc.want {
			t.Fatalf("%+v: %q %v", tc, got, err)
		}
	}
	for _, tc := range [][2]string{{"01.2.3", "patch"}, {"v1.2.3", "patch"}, {"1.2", ""}, {"1.2.3", "oops"}, {"1.2.3-pre", "patch"}} {
		if _, err := nextVersion(tc[0], tc[1]); err == nil {
			t.Errorf("accepted %v", tc)
		}
	}
}

func TestArchivesPreserveSkillAndExecutable(t *testing.T) {
	files := []releaseFile{{"lcars", []byte("binary"), 0755}, {"skills/lcars-authoring/SKILL.md", []byte("skill"), 0644}}
	for _, ext := range []string{".tar.gz", ".zip"} {
		path := filepath.Join(t.TempDir(), "release"+ext)
		if err := writeArchive(path, files); err != nil {
			t.Fatal(err)
		}
		found := map[string]string{}
		if ext == ".zip" {
			r, err := zip.OpenReader(path)
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			for _, f := range r.File {
				r, err := f.Open()
				if err != nil {
					t.Fatal(err)
				}
				b, err := io.ReadAll(r)
				r.Close()
				if err != nil {
					t.Fatal(err)
				}
				found[f.Name] = string(b)
				if f.Name == "lcars" && f.Mode().Perm() != 0755 {
					t.Fatal("executable bit lost")
				}
			}
		} else {
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			gz, err := gzip.NewReader(f)
			if err != nil {
				t.Fatal(err)
			}
			defer gz.Close()
			tr := tar.NewReader(gz)
			for {
				h, err := tr.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				b, err := io.ReadAll(tr)
				if err != nil {
					t.Fatal(err)
				}
				found[h.Name] = string(b)
				if h.Name == "lcars" && h.Mode != 0755 {
					t.Fatal("executable bit lost")
				}
			}
		}
		if found["lcars"] != "binary" || found["skills/lcars-authoring/SKILL.md"] != "skill" {
			t.Fatalf("bad archive: %v", found)
		}
	}
}

func TestVersionOnlyDefaultsToPatch(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "internal/buildinfo"), 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, versionFile)
	if err := os.WriteFile(path, []byte("1.2.3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)
	if err = run([]string{"--version-only"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "1.2.4\n" {
		t.Fatalf("default bump: %q", b)
	}
}

func TestDesktopTargets(t *testing.T) {
	for _, tc := range []struct {
		value, host string
		want        []string
	}{
		{"linux/amd64", "linux/amd64", []string{"linux/amd64"}},
		{"darwin/arm64,windows/amd64", "darwin/arm64", []string{"darwin/arm64", "windows/amd64"}},
		{"windows/arm64", "linux/amd64", []string{"windows/arm64"}},
		{"none", "linux/amd64", []string{}},
	} {
		got, err := parseDesktopTargets(tc.value, tc.host)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%+v: %v, %v", tc, got, err)
		}
	}
	for _, value := range []string{"", "none,windows/amd64", "linux/arm64", "darwin/amd64", "windows/386", "linux/amd64,linux/amd64"} {
		if _, err := parseDesktopTargets(value, "linux/amd64"); err == nil {
			t.Fatalf("accepted unsupported desktop targets %q", value)
		}
	}
}

func releaseFixture(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	for name, body := range map[string]string{
		versionFile: "1.2.3\n", "README.md": "readme", "LICENSE": "license",
		"docs/desktop.md": "desktop guide", "docs/reference.md": "reference",
		"web/toolkit/fonts/OFL.txt": "font license",
	} {
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.name=Release Test", "-c", "user.email=release@example.invalid", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "fixture"},
	} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git fixture: %v: %s", err, output)
		}
	}
}

func TestReleaseBuildsAndChecksumsDesktop(t *testing.T) {
	releaseFixture(t)
	var desktopChecks, builds int
	runner := func(dir string, env []string, name string, args ...string) error {
		if args[0] != "build" {
			if dir == "desktop" {
				desktopChecks++
			}
			return nil
		}
		builds++
		if dir == "desktop" && (!slices.Contains(env, "CGO_ENABLED=0") || !strings.Contains(strings.Join(args, " "), "-H windowsgui")) {
			t.Fatal("Windows desktop must build without cgo and without a console")
		}
		if !strings.Contains(strings.Join(args, " "), "internal/buildinfo.commit=") || !strings.Contains(strings.Join(args, " "), "internal/buildinfo.builtAt=") {
			t.Fatal("release metadata missing from build")
		}
		for i, arg := range args {
			if arg == "-o" {
				if !filepath.IsAbs(args[i+1]) {
					t.Fatal("output must work from desktop module directory")
				}
				return os.WriteFile(args[i+1], []byte("binary"), 0755)
			}
		}
		return errors.New("missing output")
	}
	if err := runWithCommand([]string{"--targets", "linux/amd64", "--desktop-targets", "windows/amd64"}, runner); err != nil {
		t.Fatal(err)
	}
	if builds != 2 || desktopChecks != 2 {
		t.Fatalf("builds=%d desktop checks=%d", builds, desktopChecks)
	}
	destination := filepath.Join("dist", "v1.2.4")
	checksums, err := os.ReadFile(filepath.Join(destination, "SHA256SUMS"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"lcars_1.2.4_linux_amd64.tar.gz", "lcars-desktop_1.2.4_windows_amd64.zip"} {
		archive, err := os.ReadFile(filepath.Join(destination, name))
		if err != nil || !strings.Contains(string(checksums), fmt.Sprintf("%x  %s\n", sha256.Sum256(archive), name)) {
			t.Fatalf("archive or checksum missing: %s, %v", name, err)
		}
	}
	archive, err := zip.OpenReader(filepath.Join(destination, "lcars-desktop_1.2.4_windows_amd64.zip"))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	for _, name := range []string{"lcars-desktop.exe", "LICENSE", "docs/desktop.md", "web/toolkit/fonts/OFL.txt", "skills/lcars-authoring/SKILL.md"} {
		entry, err := archive.Open(name)
		if err != nil {
			t.Fatalf("desktop archive missing %s: %v", name, err)
		}
		entry.Close()
	}
	var metadata struct {
		Version        string   `json:"version"`
		DesktopTargets []string `json:"desktopTargets"`
	}
	b, err := os.ReadFile(filepath.Join(destination, "release.json"))
	if err != nil || json.Unmarshal(b, &metadata) != nil || metadata.Version != "1.2.4" || !reflect.DeepEqual(metadata.DesktopTargets, []string{"windows/amd64"}) {
		t.Fatalf("bad release metadata: %s, %v", b, err)
	}
}

func TestFailedDesktopReleaseRestoresVersion(t *testing.T) {
	for _, failure := range []string{"test", "build"} {
		t.Run(failure, func(t *testing.T) {
			releaseFixture(t)
			err := runWithCommand([]string{"--targets", "linux/amd64"}, func(dir string, env []string, name string, args ...string) error {
				if dir == "desktop" && args[0] == failure {
					return errors.New("desktop failed")
				}
				for i, arg := range args {
					if arg == "-o" {
						return os.WriteFile(args[i+1], []byte("binary"), 0755)
					}
				}
				return nil
			})
			if err == nil {
				t.Fatal("desktop failure must fail the whole release")
			}
			version, _ := os.ReadFile(versionFile)
			if string(version) != "1.2.3\n" {
				t.Fatalf("version changed on failure: %q", version)
			}
			entries, _ := os.ReadDir("dist")
			if len(entries) != 0 {
				t.Fatal("failed release left archives or staging files")
			}
		})
	}
}

func TestCLIOnlyReleaseSkipsDesktop(t *testing.T) {
	releaseFixture(t)
	if err := runWithCommand([]string{"--targets", "linux/amd64", "--desktop-targets", "none"}, func(dir string, env []string, name string, args ...string) error {
		if dir == "desktop" {
			t.Fatal("CLI-only release invoked desktop tooling")
		}
		for i, arg := range args {
			if arg == "-o" {
				return os.WriteFile(args[i+1], []byte("binary"), 0755)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
