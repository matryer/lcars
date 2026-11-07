package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
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
