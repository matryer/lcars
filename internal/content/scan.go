package content

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

func ignored(n string) bool {
	return strings.HasPrefix(n, ".") || strings.HasSuffix(n, "~") || strings.HasSuffix(n, ".tmp") || strings.HasSuffix(n, ".swp")
}
func readFile(p string, limit int) ([]byte, time.Time, error) {
	st, e := os.Lstat(p)
	if e != nil {
		return nil, time.Time{}, e
	}
	if !st.Mode().IsRegular() {
		return nil, time.Time{}, fmt.Errorf("%s: regular file required; symlinks refused", p)
	}
	if st.Size() > int64(limit) {
		return nil, time.Time{}, fmt.Errorf("%s exceeds %d byte limit", p, limit)
	}
	f, e := os.Open(p)
	if e != nil {
		return nil, time.Time{}, e
	}
	defer f.Close()
	b, e := readLimited(f, limit)
	return b, st.ModTime().UTC(), e
}
func (s *Store) Scan() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expire()
	rootInfo, rootErr := os.Lstat(s.root)
	if rootErr != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		if rootErr == nil {
			rootErr = fmt.Errorf("data root must remain an ordinary directory")
		}
		s.publish([]string{"Data root unavailable: " + rootErr.Error()})
		return rootErr
	}
	dirs, e := os.ReadDir(s.root)
	if e != nil {
		s.publish([]string{"Data root unavailable: " + e.Error()})
		return e
	}
	_, diag := DiscoverScripts(s.root)
	tabs := map[string]Tab{}
	found := map[string]string{}
	invalid := map[string]string{}
	preserve := map[string]bool{}
	// scanTab reads one section folder. Top-level sections may hold one level
	// of sub-section folders, which become sub-tabs with IDs "section/sub".
	var scanTab func(id, parent string)
	scanTab = func(id, parent string) {
		dir := filepath.Join(s.root, filepath.FromSlash(id))
		t, entries, err := s.loadTab(id, parent, dir)
		tabs[id] = t
		if err != nil {
			preserve[id] = true
			for key, old := range s.tabs {
				if old.Parent == id {
					tabs[key], preserve[key] = old, true
				}
			}
			return
		}
		for _, f := range entries {
			n := f.Name()
			if ignored(n) || n == "_section.json" {
				continue
			}
			if n == "assets" && f.IsDir() {
				continue
			}
			if scriptExtension(n) {
				continue
			}
			if f.IsDir() && f.Type()&os.ModeSymlink == 0 && parent == "" && validID(n) {
				scanTab(id+"/"+n, id)
				continue
			}
			ext := filepath.Ext(n)
			stem := strings.TrimSuffix(n, ext)
			if f.IsDir() || !validID(stem) || (ext != ".html" && ext != ".md" && ext != ".txt") {
				what := "panel entry"
				if f.IsDir() && parent != "" {
					what = "nested folder (sub-sections are one level deep)"
				}
				diag = append(diag, "Warning: unsupported "+what+": "+id+"/"+n)
				continue
			}
			pid := id + "/" + stem
			if _, ok := found[pid]; ok {
				invalid[pid] = "duplicate filename stem across formats"
			}
			found[pid] = filepath.Join(dir, n)
			if f.Type()&os.ModeSymlink != 0 {
				invalid[pid] = "symlink refused"
			}
		}
	}
	for _, d := range dirs {
		name := d.Name()
		if ignored(name) {
			continue
		}
		if d.Type()&os.ModeSymlink != 0 {
			diag = append(diag, "Symlink refused: "+name)
			continue
		}
		if !d.IsDir() || !validID(name) {
			diag = append(diag, "Warning: unsupported tab entry: "+name)
			continue
		}
		scanTab(name, "")
	}
	for id, a := range s.current {
		if _, ok := found[id]; !ok && !preserve[a.Panel.Tab] && !preserve[parentTab(a.Panel.Tab)] {
			s.retire(a.Bundle)
			delete(s.current, id)
		}
	}
	s.tabs = tabs
	ids := make([]string, 0, len(found))
	for id := range found {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		p := found[id]
		old, has := s.current[id]
		reject := func(err error) {
			a := old
			if !has {
				tab, stem := splitPanelID(id)
				a = accepted{Panel: Panel{ID: id, Tab: tab, Title: label(stem), Kind: strings.TrimPrefix(filepath.Ext(p), "."), Size: "normal", Accent: "lilac", Order: 100, Data: Data{Hash: "missing", Missing: true}}, path: p}
			}
			a.Panel.Diagnostics = []string{"Update rejected: " + err.Error()}
			s.current[id] = a
		}
		if err := invalid[id]; err != "" {
			reject(fmt.Errorf("%s: %s", p, err))
			continue
		}
		limit := MaxText
		if filepath.Ext(p) == ".html" {
			limit = MaxHTML
		}
		b, modified, err := readFile(p, limit)
		if err != nil {
			reject(err)
			continue
		}
		if !has && len(s.current) >= MaxPanels {
			diag = append(diag, fmt.Sprintf("%s: exceeds %d panel limit", id, MaxPanels))
			continue
		}
		a := old
		a.Panel.Diagnostics = []string{}
		if !has || old.Bundle == nil || old.Panel.DocHash != Hash(b) {
			a, err = parsePanel(id, p, b, modified)
			if err != nil {
				reject(err)
				continue
			}
			if s.assetBudget(id, a.Bundle) > MaxAssets {
				reject(fmt.Errorf("current assets exceed %d byte limit", MaxAssets))
				continue
			}
			if old.Bundle != nil && old.Bundle.Hash != a.Bundle.Hash {
				s.retire(old.Bundle)
			}
			if has {
				a.Panel.Data = old.Panel.Data
			}
		}
		if a.Panel.Kind == "html" {
			a.Panel.Data, err = loadData(filepath.Join(filepath.Dir(p), "assets", panelStem(id), "data.json"), a.Panel.Data)
			if err != nil {
				a.Panel.Diagnostics = append(a.Panel.Diagnostics, "Data update rejected: "+err.Error())
			}
		}
		s.current[id] = a
	}
	s.publish(diag)
	return nil
}

// loadTab reads a section folder's listing and optional _section.json. A
// listing error keeps the previous tab so its panels are preserved.
func (s *Store) loadTab(id, parent, dir string) (Tab, []os.DirEntry, error) {
	name := id[strings.LastIndexByte(id, '/')+1:]
	fresh := Tab{ID: id, Parent: parent, Title: label(name), Order: 100, Accent: "lilac", Diagnostics: []string{}}
	t := fresh
	if old, ok := s.tabs[id]; ok {
		t = old
		t.Diagnostics = []string{}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Diagnostics = append(t.Diagnostics, "Directory unavailable: "+err.Error())
		return t, nil, err
	}
	b, _, err := readFile(filepath.Join(dir, "_section.json"), MaxText)
	if os.IsNotExist(err) {
		return fresh, entries, nil
	}
	if err == nil {
		var cfg struct {
			Version int    `json:"version"`
			Title   string `json:"title"`
			Order   *int   `json:"order"`
			Accent  string `json:"accent"`
			Theme   string `json:"theme"`
		}
		err = strictJSON(b, &cfg)
		if err == nil && cfg.Version != 1 {
			err = fmt.Errorf("version must be 1")
		}
		if err == nil && cfg.Accent != "" && !palette[cfg.Accent] {
			err = fmt.Errorf("unknown accent")
		}
		if err == nil && cfg.Theme != "" && !Themes[cfg.Theme] {
			err = fmt.Errorf("unknown theme %q (classic, pastel, nebula, tidal, sand, meridian)", cfg.Theme)
		}
		if err == nil && len(cfg.Title) > 256 {
			err = fmt.Errorf("title exceeds 256 characters")
		}
		if err == nil {
			t = fresh
			if cfg.Title != "" {
				t.Title = cfg.Title
			}
			if cfg.Order != nil {
				t.Order = *cfg.Order
			}
			if cfg.Accent != "" {
				t.Accent = cfg.Accent
			}
			t.Theme = cfg.Theme
			return t, entries, nil
		}
	}
	t.Diagnostics = append(t.Diagnostics, "_section.json: "+err.Error())
	return t, entries, nil
}

// splitPanelID separates "section[/sub]/stem" into its tab ID and file stem.
func splitPanelID(id string) (tab, stem string) {
	i := strings.LastIndexByte(id, '/')
	return id[:i], id[i+1:]
}
func panelStem(id string) string { _, stem := splitPanelID(id); return stem }
func parentTab(tab string) string {
	if i := strings.IndexByte(tab, '/'); i >= 0 {
		return tab[:i]
	}
	return ""
}

func loadData(path string, old Data) (Data, error) {
	for _, dir := range []string{filepath.Dir(filepath.Dir(path)), filepath.Dir(path)} {
		st, e := os.Lstat(dir)
		if os.IsNotExist(e) {
			return Data{Hash: "missing", Missing: true}, nil
		}
		if e != nil {
			return old, e
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return old, fmt.Errorf("%s: ordinary data directory required", dir)
		}
	}
	b, mt, e := readFile(path, MaxText)
	if os.IsNotExist(e) {
		return Data{Hash: "missing", Missing: true}, nil
	}
	if e != nil {
		return old, e
	}
	h := Hash(b)
	if old.Hash == h {
		return old, nil
	}
	env, e := ValidateEnvelope(b)
	if e != nil {
		return old, e
	}
	return Data{Hash: h, Envelope: env, UpdatedAt: mt}, nil
}

// ValidateEnvelope is shared by disk ingestion and script stdout publication.
func ValidateEnvelope(b []byte) (*Envelope, error) {
	if len(b) > MaxText || !utf8.Valid(b) {
		return nil, fmt.Errorf("data must be UTF-8 JSON within %d bytes", MaxText)
	}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(b, &fields); e != nil {
		return nil, e
	}
	if v, ok := fields["staleAfterSeconds"]; ok {
		var seconds int
		if json.Unmarshal(v, &seconds) != nil || seconds <= 0 {
			return nil, fmt.Errorf("staleAfterSeconds must be positive when present")
		}
	}
	var env Envelope
	if e := strictJSON(b, &env); e != nil {
		return nil, e
	}
	if env.Version != 1 || env.Data == nil {
		return nil, fmt.Errorf("version 1 and data are required")
	}
	if e := observation(env.UpdatedAt, env.StaleAfterSeconds); e != nil {
		return nil, e
	}
	return &env, nil
}

func (s *Store) assetBudget(replace string, b *Bundle) int {
	seen := map[string]bool{}
	total := 0
	add := func(b *Bundle) {
		if b == nil {
			return
		}
		for _, r := range b.Assets {
			if !seen[r.Hash] {
				seen[r.Hash] = true
				total += len(r.Bytes)
			}
		}
	}
	for id, a := range s.current {
		if id != replace {
			add(a.Bundle)
		}
	}
	add(b)
	return total
}
func (s *Store) retire(b *Bundle) {
	if b == nil {
		return
	}
	s.grace = append(s.grace, retired{b, time.Now().Add(60 * time.Second)})
	s.expire()
}
func (s *Store) expire() {
	total := 0
	for _, g := range s.grace {
		for _, r := range g.bundle.Assets {
			total += len(r.Bytes)
		}
		total += len(g.bundle.HTML)
	}
	for len(s.grace) > 0 && (time.Now().After(s.grace[0].until) || total > MaxAssets) {
		g := s.grace[0]
		total -= len(g.bundle.HTML)
		for _, r := range g.bundle.Assets {
			total -= len(r.Bytes)
		}
		s.grace = s.grace[1:]
	}
}
func snapshotAssets(dir, stem string) (map[string]Resource, time.Time, error) {
	out := map[string]Resource{}
	var latest time.Time
	assetBytes := 0
	seenHashes := map[string]bool{}
	root := filepath.Join(dir, "assets", stem)
	for _, p := range []string{filepath.Join(dir, "assets"), root} {
		st, e := os.Lstat(p)
		if os.IsNotExist(e) {
			return out, latest, nil
		}
		if e != nil {
			return nil, latest, e
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return nil, latest, fmt.Errorf("%s: ordinary asset directory required", p)
		}
	}
	e := filepath.WalkDir(root, func(p string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if p == root {
			return nil
		}
		if ignored(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s: symlink refused", p)
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if rel == "assets/"+stem+"/data.json" {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(p))
		mime, ok := resourceTypes[ext]
		if !ok {
			return fmt.Errorf("%s: unsupported resource type", p)
		}
		b, mt, e := readFile(p, MaxResource)
		if e != nil {
			return e
		}
		if ext == ".svg" {
			if e = passiveSVG(b); e != nil {
				return fmt.Errorf("%s: %w", p, e)
			}
		}
		hash := Hash(b)
		if !seenHashes[hash] {
			seenHashes[hash] = true
			assetBytes += len(b)
		}
		if assetBytes > MaxAssets {
			return fmt.Errorf("asset bundle exceeds %d byte limit", MaxAssets)
		}
		out[rel] = Resource{Bytes: b, MIME: mime, Hash: Hash(b), Modified: mt}
		if mt.After(latest) {
			latest = mt
		}
		return nil
	})
	return out, latest, e
}
