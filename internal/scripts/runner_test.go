package scripts

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/matryer/lcars/internal/content"
)

func setup(t *testing.T, source string) (*Runner, content.Script, string) {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python unavailable")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "overview")
	os.Mkdir(dir, 0700)
	os.WriteFile(filepath.Join(dir, "weather.html"), []byte("<title>Weather</title>"), 0600)
	path := filepath.Join(dir, "weather.1s.py")
	os.WriteFile(path, []byte(source), 0600)
	store, err := content.New(root)
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(root, store, Options{Interpreters: map[string]string{"py": python}, Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	jobs, diags := content.DiscoverScripts(root)
	if len(jobs) != 1 {
		t.Fatal(diags)
	}
	return r, jobs[0], filepath.Join(dir, "assets/weather/data.json")
}
func TestRunPublishesOnlySuccessfulValidOutput(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		good         bool
	}{
		{"success", `print('{"version":1,"data":{"value":42}}')`, true},
		{"failure", `import sys; print('{"version":1,"data":42}'); sys.exit(1)`, false},
		{"partial", `print('{"version":1,')`, false},
		{"timeout", `import time; time.sleep(60)`, false},
		{"oversized", `print('x'*300000)`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, job, path := setup(t, tc.source)
			if tc.name == "timeout" {
				r.opts.Timeout = 300 * time.Millisecond
			}
			os.MkdirAll(filepath.Dir(path), 0700)
			old := []byte(`{"version":1,"data":"old"}`)
			os.WriteFile(path, old, 0600)
			err := r.run(context.Background(), job)
			if tc.name == "timeout" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected execution deadline, got %v", err)
			}
			got, _ := os.ReadFile(path)
			if tc.good {
				if err != nil || string(got) == string(old) {
					t.Fatalf("%s %v", got, err)
				}
			} else if err == nil || string(got) != string(old) {
				t.Fatalf("failed run changed data: %s %v", got, err)
			}
		})
	}
}
func TestDisabledResultAndExclusiveOwner(t *testing.T) {
	r, job, path := setup(t, `print('{"version":1,"data":42}')`)
	if second, err := New(r.root, r.store, Options{}); err == nil {
		second.Close()
		t.Fatal("second runner acquired root")
	}
	os.Remove(job.Path)
	if err := r.run(context.Background(), job); err == nil {
		t.Fatal("ran removed job")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("published removed job")
	}
}

func TestDisableDuringRunAndChildCleanup(t *testing.T) {
	r, job, path := setup(t, `import subprocess, sys, time
from pathlib import Path
subprocess.Popen([sys.executable, '-c', 'import time; from pathlib import Path; time.sleep(1); Path("escaped").write_text("bad")'])
Path('started').write_text('yes')
time.sleep(60)
`)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- r.run(ctx, job) }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(filepath.Dir(job.Path), "started")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("script never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	os.Rename(job.Path, filepath.Join(filepath.Dir(job.Path), ".weather.1s.py"))
	// Cancel after the startup handshake, so child cleanup does not depend on
	// the interpreter starting within a tiny fixed execution timeout.
	cancel()
	if err := <-done; err == nil {
		t.Fatal("disabled run succeeded")
	}
	time.Sleep(1200 * time.Millisecond)
	for _, p := range []string{path, filepath.Join(filepath.Dir(job.Path), "escaped")} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("unexpected output: %s", p)
		}
	}
}
func TestPublicationRefusesSymlink(t *testing.T) {
	r, job, _ := setup(t, `print('{"version":1,"data":42}')`)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(filepath.Dir(job.Path), "assets")); err != nil {
		t.Skip(err)
	}
	if err := r.run(context.Background(), job); err == nil {
		t.Fatal("published through symlink")
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatal("wrote outside root")
	}
}

func TestSchedulerNoOverlapAndDiscardEditedRun(t *testing.T) {
	r, job, path := setup(t, `import time
from pathlib import Path
Path('started').write_text('yes')
time.sleep(60)
print('{"version":1,"data":"old"}')
`)
	r.opts.Timeout = 30 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); r.Run(ctx) }()
	defer func() { cancel(); <-done }()
	wait := func(condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(15 * time.Second)
		for !condition() {
			if time.Now().After(deadline) {
				t.Fatal("condition not reached")
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	wait(func() bool { _, err := os.Stat(filepath.Join(filepath.Dir(job.Path), "started")); return err == nil })
	source := `from pathlib import Path
import time
lock=Path('running')
if lock.exists():
    Path('overlap').write_text('bad')
lock.write_text('yes')
time.sleep(1.2)
lock.unlink()
print('{"version":1,"data":"new"}', end='\r\n')
`
	if err := os.WriteFile(job.Path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	wait(func() bool {
		b, _ := os.ReadFile(path)
		return strings.TrimSpace(string(b)) == `{"version":1,"data":"new"}`
	})
	if _, err := os.Stat(filepath.Join(filepath.Dir(job.Path), "overlap")); !os.IsNotExist(err) {
		t.Fatal("overlapping jobs")
	}
	status := r.store.State().Panels[0].Script
	wait(func() bool {
		status = r.store.State().Panels[0].Script
		return status != nil && !status.LastSuccess.IsZero()
	})
}
func TestUnchangedOutputPreservesTimestampAndTemporaryFilesCleaned(t *testing.T) {
	r, job, path := setup(t, `print('{"version":1,"data":42}')`)
	if err := r.run(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(path)
	if err := r.run(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("unchanged bytes changed timestamp")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
}
func TestEnvironmentDoesNotInheritSecrets(t *testing.T) {
	t.Setenv("LCARS_TEST_SECRET", "private")
	r, job, path := setup(t, `import os, json; print(json.dumps({'version':1,'data':os.getenv('LCARS_TEST_SECRET')}))`)
	if err := r.run(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if strings.Contains(string(b), "private") {
		t.Fatal("inherited secret")
	}
	r.opts.PassEnv = []string{"LCARS_TEST_SECRET"}
	if err := r.run(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	if !strings.Contains(string(b), "private") {
		t.Fatal("explicit environment missing")
	}
}

func TestSuccessfulButDisabledInvocationCannotPublish(t *testing.T) {
	r, job, path := setup(t, `from pathlib import Path
import time
Path('started').write_text('yes')
while not Path('release').exists(): time.sleep(.01)
print('{"version":1,"data":42}')
`)
	r.opts.Timeout = 30 * time.Second
	done := make(chan error, 1)
	go func() { done <- r.run(context.Background(), job) }()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(filepath.Dir(job.Path), "started")); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("script never started")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := os.Rename(job.Path, filepath.Join(filepath.Dir(job.Path), ".weather.1s.py")); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(filepath.Dir(job.Path), "release"), []byte("go"), 0600)
	if err := <-done; err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("got %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("obsolete invocation published")
	}
}
