package content

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Script is a validated adjacent producer. Revision identifies its source bytes.
type Script struct {
	ID, Path, Extension, Revision string
	Every                         time.Duration
}

var intervalPattern = regexp.MustCompile(`^([1-9][0-9]*)([smhd])$`)

func scriptExtension(name string) bool {
	switch filepath.Ext(name) {
	case ".py", ".js", ".sh", ".ps1":
		return true
	}
	return false
}

// DiscoverScripts only reads files; interpreters are checked by the opt-in runner.
func DiscoverScripts(root string) ([]Script, []string) {
	st, err := os.Lstat(root)
	if err != nil {
		return nil, []string{"Scripts: " + err.Error()}
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return nil, []string{"Scripts: ordinary content root required"}
	}
	jobs := map[string]Script{}
	duplicates := map[string]bool{}
	diags := []string{}
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, []string{"Scripts: " + err.Error()}
	}
	for _, dir := range dirs {
		if ignored(dir.Name()) || !dir.IsDir() || !validID(dir.Name()) {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(root, dir.Name()))
		if err != nil {
			diags = append(diags, "Scripts: "+err.Error())
			continue
		}
		for _, entry := range entries {
			name := entry.Name()
			if ignored(name) || !scriptExtension(name) {
				continue
			}
			stem := strings.TrimSuffix(name, filepath.Ext(name))
			split := strings.LastIndexByte(stem, '.')
			if split < 0 {
				continue
			} // Ordinary helper, not a scheduled producer.
			panel, interval := stem[:split], stem[split+1:]
			path := filepath.Join(root, dir.Name(), name)
			reject := func(message string) { diags = append(diags, dir.Name()+"/"+name+": "+message) }
			parts := intervalPattern.FindStringSubmatch(interval)
			if !validID(panel) || parts == nil {
				reject("expected <panel>.<positive interval s/m/h/d>.<script extension>")
				continue
			}
			units := map[string]time.Duration{"s": time.Second, "m": time.Minute, "h": time.Hour, "d": 24 * time.Hour}
			n, e := strconv.ParseInt(parts[1], 10, 64)
			unit := units[parts[2]]
			if e != nil || n > int64((1<<63-1)/unit) {
				reject("interval exceeds supported duration")
				continue
			}
			b, _, e := readFile(path, MaxHTML)
			if e != nil {
				reject(e.Error())
				continue
			}
			if _, _, e = readFile(filepath.Join(root, dir.Name(), panel+".html"), MaxHTML); e != nil {
				reject("matching regular HTML panel required")
				continue
			}
			id := dir.Name() + "/" + panel
			if _, ok := jobs[id]; ok {
				duplicates[id] = true
			}
			jobs[id] = Script{ID: id, Path: path, Extension: strings.TrimPrefix(filepath.Ext(name), "."), Revision: Hash(b), Every: time.Duration(n) * unit}
		}
	}
	out := []Script{}
	for id, job := range jobs {
		if duplicates[id] {
			diags = append(diags, fmt.Sprintf("%s: duplicate script producers", id))
		} else {
			out = append(out, job)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	sort.Strings(diags)
	return out, diags
}

// ScriptStatus reports execution health without changing source observation time.
type ScriptStatus struct {
	LastAttempt time.Time `json:"lastAttempt"`
	LastSuccess time.Time `json:"lastSuccess"`
	DurationMS  int64     `json:"durationMs"`
	Error       string    `json:"error,omitempty"`
}

func (s *Store) SetScriptStatus(id string, status *ScriptStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scriptStatus == nil {
		s.scriptStatus = map[string]ScriptStatus{}
	}
	if status == nil {
		delete(s.scriptStatus, id)
	} else {
		s.scriptStatus[id] = *status
	}
	s.publish(s.state.Diagnostics)
}
