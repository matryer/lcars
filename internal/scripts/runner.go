// Package scripts runs trusted, opt-in adjacent data producers.
package scripts

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/matryer/lcars/internal/content"
)

type Options struct {
	Timeout      time.Duration
	Concurrency  int
	Interpreters map[string]string
	// PassEnv explicitly opts additional service environment variables into scripts.
	PassEnv []string
	Log     io.Writer
}
type Runner struct {
	root  string
	store *content.Store
	opts  Options
	lock  *os.File
	logMu sync.Mutex
}

func New(root string, store *content.Store, opts Options) (*Runner, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.Concurrency == 0 {
		opts.Concurrency = 4
	}
	if opts.Timeout < 0 || opts.Concurrency < 1 {
		return nil, errors.New("script timeout and concurrency must be positive")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = ordinaryDir(root); err != nil {
		return nil, err
	}
	lockPath := filepath.Join(root, ".lcars-scripts.lock")
	if st, e := os.Lstat(lockPath); e == nil && !st.Mode().IsRegular() {
		return nil, errors.New("script lock must be a regular file")
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = lockFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("another script runner owns this content root: %w", err)
	}
	return &Runner{root: root, store: store, opts: opts, lock: f}, nil
}

// Close is called after Run returns. Keep the lock inode to avoid lock races.
func (r *Runner) Close() {
	if r.lock != nil {
		r.lock.Close()
		r.lock = nil
	}
}
func (r *Runner) interpreter(job content.Script) (string, []string, error) {
	name := r.opts.Interpreters[job.Extension]
	if name == "" {
		name = map[string]string{"py": "python3", "js": "node", "sh": "sh", "ps1": "pwsh"}[job.Extension]
	}
	executable, err := exec.LookPath(name)
	if err != nil {
		return "", nil, fmt.Errorf("%s interpreter unavailable; configure -script-%s", job.Extension, job.Extension)
	}
	// Resolve before changing directory. Do not invoke an implicit shell.
	executable, err = filepath.Abs(executable)
	if err != nil {
		return "", nil, err
	}
	args := []string{job.Path}
	if job.Extension == "ps1" {
		args = []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-File", job.Path}
	}
	return executable, args, nil
}
func (r *Runner) environment() []string {
	names := append([]string{"PATH", "HOME", "USERPROFILE", "SystemRoot", "WINDIR", "TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL", "PATHEXT"}, r.opts.PassEnv...)
	values := map[string]string{}
	for _, name := range names {
		if value, ok := os.LookupEnv(name); ok {
			values[name] = value
		}
	}
	values["PYTHONIOENCODING"] = "utf-8"
	values["PYTHONUTF8"] = "1"
	values["PYTHONDONTWRITEBYTECODE"] = "1"
	out := []string{}
	for k, v := range values {
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}

type limitedBuffer struct {
	bytes.Buffer
	limit    int
	overflow bool
	cancel   context.CancelFunc
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - b.Len()
	if len(p) > remaining {
		b.overflow = true
		p = p[:remaining]
		if b.cancel != nil {
			b.cancel()
		}
	}
	_, _ = b.Buffer.Write(p)
	return n, nil
}
func (r *Runner) current(job content.Script) bool {
	jobs, _ := content.DiscoverScripts(r.root)
	for _, j := range jobs {
		if j.ID == job.ID {
			return j == job
		}
	}
	return false
}
func (r *Runner) run(parent context.Context, job content.Script) error {
	if !r.current(job) {
		return errors.New("script changed or disabled")
	}
	executable, args, err := r.interpreter(job)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, r.opts.Timeout)
	defer cancel()
	stdout := &limitedBuffer{limit: content.MaxText, cancel: cancel}
	stderr := &limitedBuffer{limit: 8192}
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = filepath.Dir(job.Path)
	cmd.Env = r.environment()
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = time.Second
	after, cleanup, err := prepareProcess(cmd)
	if err != nil {
		return fmt.Errorf("process setup failed: %w", err)
	}
	defer cleanup()
	if err = cmd.Start(); err != nil {
		return errors.New("could not start script interpreter")
	}
	if err = after(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return errors.New("could not contain script process")
	}
	err = cmd.Wait()
	if r.opts.Log != nil && stderr.Len() > 0 {
		r.logMu.Lock()
		fmt.Fprintf(r.opts.Log, "Script %s stderr (max 8192 bytes):\n%s\n", job.ID, stderr.String())
		r.logMu.Unlock()
	}
	if stdout.overflow {
		return errors.New("stdout exceeds data size limit")
	}
	if ctx.Err() != nil {
		return fmt.Errorf("script interrupted: %w", ctx.Err())
	}
	if err != nil {
		return fmt.Errorf("script failed: %w", err)
	}
	if _, err = content.ValidateEnvelope(stdout.Bytes()); err != nil {
		return errors.New("stdout is not a valid version-1 data envelope")
	}
	if !r.current(job) {
		return errors.New("script changed or disabled")
	}
	if err = parent.Err(); err != nil {
		return err
	}
	if err = r.publish(job, stdout.Bytes()); err != nil {
		return fmt.Errorf("data publication failed: %w", err)
	}
	return r.store.Scan()
}
func ordinaryDir(path string) error {
	st, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("ordinary directory required: %s", path)
	}
	return nil
}
func (r *Runner) publish(job content.Script, b []byte) error {
	parts := strings.Split(job.ID, "/")
	if len(parts) < 2 || len(parts) > 3 {
		return errors.New("invalid panel ID")
	}
	dir := r.root
	if err := ordinaryDir(dir); err != nil {
		return err
	}
	// section[/sub]/assets/stem: only the asset folders are created.
	for i, part := range append(append(append([]string{}, parts[:len(parts)-1]...), "assets"), parts[len(parts)-1]) {
		dir = filepath.Join(dir, part)
		if i < len(parts)-1 {
			if err := ordinaryDir(dir); err != nil {
				return err
			}
			continue
		}
		if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
			return err
		}
		if err := ordinaryDir(dir); err != nil {
			return err
		}
	}
	path := filepath.Join(dir, "data.json")
	if st, err := os.Lstat(path); err == nil {
		if !st.Mode().IsRegular() {
			return errors.New("data target must be a regular file")
		}
		old, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Equal(old, b) {
			return nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.CreateTemp(dir, ".data-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return replaceFile(f.Name(), path)
}

type scheduled struct {
	job    content.Script
	next   time.Time
	cancel context.CancelFunc
	status content.ScriptStatus
}
type result struct {
	entry             *scheduled
	started, finished time.Time
	err               error
}

// Run owns scheduling state; workers only execute and return bounded results.
func (r *Runner) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	entries := map[string]*scheduled{}
	done := make(chan result, r.opts.Concurrency)
	active := 0
	busy := map[string]bool{}
	reconcile := func() {
		jobs, _ := content.DiscoverScripts(r.root)
		current := map[string]content.Script{}
		for _, j := range jobs {
			current[j.ID] = j
		}
		for id, e := range entries {
			j, exists := current[id]
			if !exists || j != e.job {
				if e.cancel != nil {
					e.cancel()
				}
				delete(entries, id)
				r.store.SetScriptStatus(id, nil)
			}
		}
		for _, j := range jobs {
			if _, ok := entries[j.ID]; !ok {
				entries[j.ID] = &scheduled{job: j}
			}
		}
	}
	start := func() {
		ids := []string{}
		for id := range entries {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			e := entries[id]
			if active >= r.opts.Concurrency {
				break
			}
			if busy[id] || e.cancel != nil || time.Now().Before(e.next) {
				continue
			}
			child, cancel := context.WithCancel(ctx)
			e.cancel = cancel
			active++
			busy[id] = true
			started := time.Now()
			e.status.LastAttempt = started
			r.store.SetScriptStatus(id, &e.status)
			go func(e *scheduled) { err := r.run(child, e.job); done <- result{e, started, time.Now(), err} }(e)
		}
	}
	reconcile()
	start()
	for {
		select {
		case <-ctx.Done():
			for _, e := range entries {
				if e.cancel != nil {
					e.cancel()
				}
			}
			for active > 0 {
				<-done
				active--
			}
			return
		case <-ticker.C:
			reconcile()
			start()
		case res := <-done:
			active--
			delete(busy, res.entry.job.ID)
			e := res.entry
			e.cancel()
			e.cancel = nil
			if entries[e.job.ID] == e {
				// Wall-clock due times count machine sleep as elapsed time.
				e.next = res.finished.Round(0).Add(e.job.Every)
				e.status.DurationMS = res.finished.Sub(res.started).Milliseconds()
				e.status.Error = ""
				if res.err != nil {
					e.status.Error = res.err.Error()
				} else {
					e.status.LastSuccess = res.finished
				}
				r.store.SetScriptStatus(e.job.ID, &e.status)
			}
			start()
		}
	}
}
