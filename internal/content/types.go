// Package content validates file publications and keeps accepted immutable bundles.
package content

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	MaxPanels   = 200
	MaxHTML     = 1 << 20
	MaxText     = 256 << 10
	MaxResource = 10 << 20
	MaxAssets   = 100 << 20
)

type Tab struct {
	ID          string   `json:"id"`
	Parent      string   `json:"parent,omitempty"`
	Title       string   `json:"title"`
	Order       int      `json:"order"`
	Accent      string   `json:"accent"`
	Theme       string   `json:"theme,omitempty"`
	Diagnostics []string `json:"diagnostics"`
}

// Themes are the shell's page palettes. A section without one gets a stable
// theme chosen from a hash of its ID.
var Themes = map[string]bool{"classic": true, "pastel": true, "nebula": true, "tidal": true, "sand": true, "meridian": true}

type Envelope struct {
	Version           int             `json:"version"`
	UpdatedAt         string          `json:"updatedAt,omitempty"`
	StaleAfterSeconds int             `json:"staleAfterSeconds,omitempty"`
	Data              json.RawMessage `json:"data"`
}
type Data struct {
	Hash      string    `json:"hash"`
	Missing   bool      `json:"missing"`
	Envelope  *Envelope `json:"envelope"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type Source struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}
type Panel struct {
	Script            *ScriptStatus `json:"script,omitempty"`
	Sources           []Source      `json:"sources,omitempty"`
	ID                string        `json:"id"`
	Tab               string        `json:"tab"`
	Title             string        `json:"title"`
	Kind              string        `json:"kind"`
	Size              string        `json:"size"`
	Accent            string        `json:"accent"`
	Order             int           `json:"order"`
	DocHash           string        `json:"docHash"`
	BundleHash        string        `json:"bundleHash"`
	UpdatedAt         time.Time     `json:"updatedAt"`
	ObservedAt        string        `json:"observedAt"`
	StaleAfterSeconds int           `json:"staleAfterSeconds"`
	SourceURL         string        `json:"sourceUrl"`
	Diagnostics       []string      `json:"diagnostics"`
	Data              Data          `json:"data"`
	URL               string        `json:"url"`
}
type State struct {
	Revision    string   `json:"revision"`
	Tabs        []Tab    `json:"tabs"`
	Panels      []Panel  `json:"panels"`
	Diagnostics []string `json:"diagnostics"`
}
type Resource struct {
	Bytes    []byte
	MIME     string
	Hash     string
	Modified time.Time
}
type Bundle struct {
	ID        string
	Hash      string
	HTML      []byte
	Assets    map[string]Resource
	UpdatedAt time.Time
}
type accepted struct {
	Panel  Panel
	Bundle *Bundle
	path   string
}
type retired struct {
	bundle *Bundle
	until  time.Time
}
type Store struct {
	subscribers  map[chan string]struct{}
	scriptStatus map[string]ScriptStatus
	mu           sync.RWMutex
	root         string
	current      map[string]accepted
	tabs         map[string]Tab
	state        State
	grace        []retired
}

func Hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func New(root string) (*Store, error) {
	p, e := filepath.Abs(root)
	if e != nil {
		return nil, e
	}
	st, e := os.Lstat(p)
	if e != nil {
		return nil, e
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("data root must be an ordinary directory: %s", p)
	}
	s := &Store{root: p, current: map[string]accepted{}, tabs: map[string]Tab{}}
	e = s.Scan()
	return s, e
}
func (s *Store) Run(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = s.Scan()
		}
	}
}
func (s *Store) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, _ := json.Marshal(s.state)
	var out State
	_ = json.Unmarshal(b, &out)
	return out
}
func (s *Store) Resource(hash, id, name string) (Resource, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var b *Bundle
	if a, ok := s.current[id]; ok && a.Bundle != nil && a.Bundle.Hash == hash {
		b = a.Bundle
	}
	if b == nil {
		for _, g := range s.grace {
			if time.Now().Before(g.until) && g.bundle.Hash == hash && g.bundle.ID == id {
				b = g.bundle
				break
			}
		}
	}
	if b == nil {
		return Resource{}, false
	}
	if name == "index.html" {
		return Resource{Bytes: append([]byte(nil), b.HTML...), MIME: "text/html; charset=utf-8"}, true
	}
	r, ok := b.Assets[name]
	r.Bytes = append([]byte(nil), r.Bytes...)
	return r, ok
}
func (s *Store) publish(diag []string) {
	st := State{Tabs: []Tab{}, Panels: []Panel{}, Diagnostics: diag}
	for _, t := range s.tabs {
		st.Tabs = append(st.Tabs, t)
	}
	sort.Slice(st.Tabs, func(i, j int) bool {
		a, b := st.Tabs[i], st.Tabs[j]
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		return a.ID < b.ID
	})
	for _, a := range s.current {
		panel := a.Panel
		if status, ok := s.scriptStatus[panel.ID]; ok {
			panel.Script = &status
			if status.Error != "" {
				panel.Diagnostics = append(append([]string{}, panel.Diagnostics...), "Script refresh failed: "+status.Error)
			}
		}
		st.Panels = append(st.Panels, panel)
	}
	sort.Slice(st.Panels, func(i, j int) bool {
		a, b := st.Panels[i], st.Panels[j]
		if a.Order != b.Order {
			return a.Order < b.Order
		}
		return a.ID < b.ID
	})
	b, _ := json.Marshal(st)
	st.Revision = Hash(b)
	changed := s.state.Revision != st.Revision
	s.state = st
	if changed {
		s.notify()
	}
}
func Validate(root string, w io.Writer) error {
	s, e := New(root)
	if e != nil {
		return e
	}
	st := s.State()
	bad := false
	for _, d := range st.Diagnostics {
		fmt.Fprintln(w, d)
		if !strings.HasPrefix(d, "Warning:") {
			bad = true
		}
	}
	for _, t := range st.Tabs {
		for _, d := range t.Diagnostics {
			fmt.Fprintf(w, "%s: %s\n", t.ID, d)
			bad = true
		}
	}
	for _, p := range st.Panels {
		fmt.Fprintf(w, "%s document=%s data=%s\n", p.ID, p.DocHash, p.Data.Hash)
		for _, d := range p.Diagnostics {
			fmt.Fprintf(w, "%s: %s\n", p.ID, d)
			bad = true
		}
	}
	if bad {
		return fmt.Errorf("content validation failed")
	}
	return nil
}
