// release builds portable archives locally. It never commits, tags, or publishes.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/matryer/lcars/internal/buildinfo"
	authoring "github.com/matryer/lcars/skills/lcars-authoring"
)

const versionFile = "internal/buildinfo/VERSION"

var semver = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

func nextVersion(current, bump string) (string, error) {
	if !semver.MatchString(current) {
		return "", fmt.Errorf("invalid version %q: expected MAJOR.MINOR.PATCH", current)
	}
	if bump == "" || bump == "none" {
		return current, nil
	}
	parts := strings.Split(current, ".")
	index := map[string]int{"major": 0, "minor": 1, "patch": 2}
	i, ok := index[bump]
	if !ok {
		return "", errors.New("bump must be patch, minor, major, or none")
	}
	n, err := strconv.ParseUint(parts[i], 10, 63)
	if err != nil {
		return "", err
	}
	parts[i] = strconv.FormatUint(n+1, 10)
	for j := i + 1; j < len(parts); j++ {
		parts[j] = "0"
	}
	return strings.Join(parts, "."), nil
}

type releaseFile struct {
	name string
	data []byte
	mode fs.FileMode
}

func writeArchive(path string, files []releaseFile) (err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	// Stable archive headers: no build machine paths or local timestamps.
	if strings.HasSuffix(path, ".zip") {
		w := zip.NewWriter(f)
		for _, file := range files {
			h := &zip.FileHeader{Name: file.name, Method: zip.Deflate}
			h.SetMode(file.mode)
			entry, err := w.CreateHeader(h)
			if err != nil {
				return err
			}
			if _, err = entry.Write(file.data); err != nil {
				return err
			}
		}
		return w.Close()
	}
	gz := gzip.NewWriter(f)
	w := tar.NewWriter(gz)
	for _, file := range files {
		if err = w.WriteHeader(&tar.Header{Name: file.name, Mode: int64(file.mode), Size: int64(len(file.data))}); err != nil {
			return err
		}
		if _, err = w.Write(file.data); err != nil {
			return err
		}
	}
	return errors.Join(w.Close(), gz.Close())
}

func command(env []string, name string, args ...string) error {
	if name == "go" {
		name = filepath.Join(runtime.GOROOT(), "bin", "go")
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
	}
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd.Run()
}

func main() {
	if err := run(os.Args[1:]); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}
func run(args []string) (err error) {
	flags := flag.NewFlagSet("release", flag.ContinueOnError)
	bump := flags.String("bump", "patch", "bump patch, minor, or major before packaging; none keeps the current version")
	versionOnly := flags.Bool("version-only", false, "only bump VERSION; do not build archives")
	output := flags.String("out", "dist", "parent directory for versioned release folders")
	targets := flags.String("targets", "linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64", "comma-separated OS/architecture pairs")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected release arguments")
	}
	raw, err := os.ReadFile(versionFile)
	if err != nil {
		return fmt.Errorf("run from the repository root: %w", err)
	}
	current := strings.TrimSpace(string(raw))
	version, err := nextVersion(current, *bump)
	if err != nil {
		return err
	}
	if *versionOnly {
		if *bump == "" || *bump == "none" {
			return errors.New("--version-only requires --bump")
		}
		if err = os.WriteFile(versionFile, []byte(version+"\n"), 0644); err != nil {
			return err
		}
		fmt.Println("Version", current, "→", version)
		return nil
	}
	supported := map[string]bool{"linux/amd64": true, "linux/arm64": true, "darwin/amd64": true, "darwin/arm64": true, "windows/amd64": true, "windows/arm64": true}
	requested := strings.Split(*targets, ",")
	seen := map[string]bool{}
	for _, target := range requested {
		if !supported[target] || seen[target] {
			return fmt.Errorf("unsupported or duplicate target %q", target)
		}
		seen[target] = true
	}
	destination := filepath.Join(*output, "v"+version)
	if _, err = os.Lstat(destination); !os.IsNotExist(err) {
		return fmt.Errorf("release destination must be new: %s", destination)
	}
	// Checks run before touching VERSION or creating artifacts.
	if err = command(nil, "go", "test", "-race", "./..."); err != nil {
		return err
	}
	if err = command(nil, "go", "vet", "./..."); err != nil {
		return err
	}
	if err = os.MkdirAll(*output, 0755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(*output, ".release-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	commitBytes, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return err
	}
	commit := strings.TrimSpace(string(commitBytes))
	statusBytes, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return err
	}
	modified := len(statusBytes) > 0 || current != version
	builtAt := time.Now().UTC().Format(time.RFC3339)
	// Embedding the candidate version keeps VERSION as the only version source.
	if err = os.WriteFile(versionFile, []byte(version+"\n"), 0644); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.WriteFile(versionFile, raw, 0644))
		}
	}()
	common := []releaseFile{}
	for _, name := range []string{"README.md", "web/toolkit/fonts/OFL.txt"} {
		b, e := os.ReadFile(name)
		if e != nil {
			return e
		}
		common = append(common, releaseFile{name, b, 0644})
	}
	err = fs.WalkDir(authoring.Files, ".", func(name string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		b, e := authoring.Files.ReadFile(name)
		if e != nil {
			return e
		}
		common = append(common, releaseFile{"skills/lcars-authoring/" + name, b, 0644})
		return nil
	})
	if err != nil {
		return err
	}
	var checksums strings.Builder
	for _, target := range requested {
		pair := strings.Split(target, "/")
		binary := "lcars"
		ext := ".tar.gz"
		if pair[0] == "windows" {
			binary += ".exe"
			ext = ".zip"
		}
		binaryPath := filepath.Join(stage, binary)
		ldflags := "-s -w -X github.com/matryer/lcars/internal/buildinfo.commit=" + commit + " -X github.com/matryer/lcars/internal/buildinfo.builtAt=" + builtAt
		if err = command([]string{"CGO_ENABLED=0", "GOOS=" + pair[0], "GOARCH=" + pair[1]}, "go", "build", "-trimpath", "-ldflags", ldflags, "-o", binaryPath, "./cmd/lcars"); err != nil {
			return err
		}
		b, e := os.ReadFile(binaryPath)
		if e != nil {
			return e
		}
		if err = os.Remove(binaryPath); err != nil {
			return err
		}
		files := append([]releaseFile{{binary, b, 0755}}, common...)
		name := "lcars_" + version + "_" + pair[0] + "_" + pair[1] + ext
		if err = writeArchive(filepath.Join(stage, name), files); err != nil {
			return err
		}
		archive, e := os.ReadFile(filepath.Join(stage, name))
		if e != nil {
			return e
		}
		fmt.Fprintf(&checksums, "%x  %s\n", sha256.Sum256(archive), name)
		fmt.Println("Built", name)
	}
	metadata, err := json.MarshalIndent(map[string]any{"version": version, "contractVersion": buildinfo.ContractVersion, "protocolVersion": buildinfo.ProtocolVersion, "commit": commit, "builtAt": builtAt, "modified": modified, "targets": requested}, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(stage, "release.json"), append(metadata, '\n'), 0644); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(stage, "SHA256SUMS"), []byte(checksums.String()), 0644); err != nil {
		return err
	}
	if err = os.Rename(stage, destination); err != nil {
		return err
	}
	fmt.Printf("Release %s ready in %s. Review and commit VERSION, then tag v%s when ready to publish.\n", version, destination, version)
	return nil
}
