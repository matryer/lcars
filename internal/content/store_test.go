package content_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/matryer/lcars/internal/content"
)

func write(t *testing.T, root, name, value string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		t.Fatal(err)
	}
}
func openStore(t *testing.T, root string) *content.Store {
	t.Helper()
	store, err := content.New(root)
	if err != nil {
		t.Fatal(err)
	}
	return store
}
func scan(t *testing.T, store *content.Store) {
	t.Helper()
	if err := store.Scan(); err != nil {
		t.Fatal(err)
	}
}
func panel(t *testing.T, store *content.Store, id string) content.Panel {
	t.Helper()
	for _, p := range store.State().Panels {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("panel %q not found in %#v", id, store.State().Panels)
	return content.Panel{}
}
func accepted(t *testing.T, p content.Panel) {
	t.Helper()
	if p.DocHash == "" || p.BundleHash == "" || len(p.Diagnostics) != 0 {
		t.Fatalf("expected accepted panel, got %#v", p)
	}
}

func TestImmutableResourcesCommitWithDocument(t *testing.T) {
	root := t.TempDir()
	doc := `<title>Card</title><script src="assets/card/app.js"></script><p>Reading</p>`
	write(t, root, "overview/card.html", doc)
	write(t, root, "overview/assets/card/app.js", `window.reading = 1;`)
	store := openStore(t, root)
	first := panel(t, store, "overview/card")
	accepted(t, first)
	before := store.State().Revision

	write(t, root, "overview/assets/card/app.js", `window.reading = 2;`)
	scan(t, store)
	unchanged := panel(t, store, first.ID)
	if unchanged.BundleHash != first.BundleHash || store.State().Revision != before {
		t.Fatal("asset-only staging published a revision before document commit")
	}
	resource, ok := store.Resource(first.BundleHash, first.ID, "assets/card/app.js")
	if !ok || string(resource.Bytes) != `window.reading = 1;` {
		t.Fatalf("accepted resource was mutated: %q, found=%v", resource.Bytes, ok)
	}
	// Mutating the caller's resource copy must not mutate accepted bytes.
	resource.Bytes[0] = '!'
	resource, _ = store.Resource(first.BundleHash, first.ID, "assets/card/app.js")
	if string(resource.Bytes) != `window.reading = 1;` {
		t.Fatal("resource lookup leaked mutable storage")
	}

	write(t, root, "overview/card.html", doc+`<!-- commit staged assets -->`)
	scan(t, store)
	next := panel(t, store, first.ID)
	accepted(t, next)
	if next.DocHash == first.DocHash || next.BundleHash == first.BundleHash {
		t.Fatal("document commit did not publish staged resources")
	}
	resource, ok = store.Resource(next.BundleHash, next.ID, "assets/card/app.js")
	if !ok || string(resource.Bytes) != `window.reading = 2;` {
		t.Fatalf("committed resource = %q, found=%v", resource.Bytes, ok)
	}
	// Released resources remain available for the documented grace period.
	resource, ok = store.Resource(first.BundleHash, first.ID, "assets/card/app.js")
	if !ok || string(resource.Bytes) != `window.reading = 1;` {
		t.Fatal("prior bundle disappeared before grace expiry")
	}
}

func TestUnchangedBytesKeepPublicationTimeAndRevision(t *testing.T) {
	root := t.TempDir()
	write(t, root, "overview/briefing.md", "# Briefing\n\nReady.\n")
	path := filepath.Join(root, "overview", "briefing.md")
	original := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(path, original, original); err != nil {
		t.Fatal(err)
	}
	store := openStore(t, root)
	first := panel(t, store, "overview/briefing")
	accepted(t, first)
	revision := store.State().Revision
	if !first.UpdatedAt.Equal(original) {
		t.Fatalf("startup time = %v, want file time %v", first.UpdatedAt, original)
	}
	later := original.Add(time.Hour)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	scan(t, store)
	next := panel(t, store, first.ID)
	if !next.UpdatedAt.Equal(first.UpdatedAt) || next.DocHash != first.DocHash || store.State().Revision != revision {
		t.Fatal("unchanged bytes changed publishing evidence")
	}
}

func TestRejectedDocumentRetainsAcceptedBundle(t *testing.T) {
	root := t.TempDir()
	write(t, root, "overview/card.html", `<title>Accepted</title><p>Original content</p>`)
	store := openStore(t, root)
	first := panel(t, store, "overview/card")
	accepted(t, first)
	write(t, root, "overview/card.html", `<meta name="lcars:size" content="giant"><p>Invalid replacement</p>`)
	scan(t, store)
	rejected := panel(t, store, first.ID)
	if rejected.DocHash != first.DocHash || rejected.BundleHash != first.BundleHash || !rejected.UpdatedAt.Equal(first.UpdatedAt) || rejected.Title != first.Title {
		t.Fatal("rejected document displaced accepted publication")
	}
	if len(rejected.Diagnostics) == 0 {
		t.Fatal("rejection has no visible diagnostic")
	}
	resource, ok := store.Resource(first.BundleHash, first.ID, "index.html")
	if !ok || !bytes.Contains(resource.Bytes, []byte("Original content")) {
		t.Fatal("accepted document is unavailable after rejection")
	}
	write(t, root, "overview/card.html", `<title>Recovered</title><p>Valid replacement</p>`)
	scan(t, store)
	next := panel(t, store, first.ID)
	accepted(t, next)
	if next.DocHash == first.DocHash || next.Title != "Recovered" {
		t.Fatal("valid replacement did not recover from rejection")
	}
}

func TestLiveDataAcceptanceRejectionAndDeletion(t *testing.T) {
	root := t.TempDir()
	write(t, root, "overview/card.html", `<title>Live</title><output>Unavailable</output>`)
	dataPath := "overview/assets/card/data.json"
	initial := `{"version":1,"updatedAt":"2026-11-07T09:00:00Z","staleAfterSeconds":300,"data":{"watts":420}}`
	write(t, root, dataPath, initial)
	store := openStore(t, root)
	first := panel(t, store, "overview/card")
	accepted(t, first)
	if first.Data.Missing || first.Data.Hash != content.Hash([]byte(initial)) || first.Data.Envelope == nil {
		t.Fatalf("initial data not accepted: %#v", first.Data)
	}

	for _, invalid := range []string{`{"version":1,"data":`, `{"version":1,"data":{},"surprise":true}`, `{"version":1}`, `{"version":1,"staleAfterSeconds":300,"data":{}}`, `{"version":1,"updatedAt":"2026-11-07T09:00:00Z","staleAfterSeconds":0,"data":{}}`} {
		write(t, root, dataPath, invalid)
		scan(t, store)
		p := panel(t, store, first.ID)
		if p.Data.Hash != first.Data.Hash || !p.Data.UpdatedAt.Equal(first.Data.UpdatedAt) || p.DocHash != first.DocHash || p.BundleHash != first.BundleHash {
			t.Fatal("invalid data changed accepted document/data evidence")
		}
		if len(p.Diagnostics) == 0 {
			t.Fatalf("invalid data lacks diagnostic: %s", invalid)
		}
	}
	write(t, root, dataPath, `{"version":1,"data":null}`)
	scan(t, store)
	presentNull := panel(t, store, first.ID)
	accepted(t, presentNull)
	if presentNull.Data.Missing || string(presentNull.Data.Envelope.Data) != "null" {
		t.Fatal("present null data confused with a missing file")
	}
	if presentNull.BundleHash != first.BundleHash {
		t.Fatal("data-only update replaced code bundle")
	}
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(dataPath))); err != nil {
		t.Fatal(err)
	}
	scan(t, store)
	missing := panel(t, store, first.ID)
	if !missing.Data.Missing || missing.Data.Hash != "missing" || !missing.Data.UpdatedAt.IsZero() {
		t.Fatalf("deleted data lacks explicit missing state: %#v", missing.Data)
	}
}

func TestDeletionAndUnavailableRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "content")
	write(t, root, "overview/briefing.txt", "Ready")
	store := openStore(t, root)
	first := panel(t, store, "overview/briefing")
	moved := filepath.Join(base, "unavailable")
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	if err := store.Scan(); err == nil {
		t.Fatal("missing root was reported as a successful scan")
	}
	if panel(t, store, first.ID).DocHash != first.DocHash || len(store.State().Diagnostics) == 0 {
		t.Fatal("root loss erased accepted content or omitted diagnostic")
	}
	if err := os.Rename(moved, root); err != nil {
		t.Fatal(err)
	}
	scan(t, store)
	if len(store.State().Diagnostics) != 0 {
		t.Fatal("root recovery retained stale diagnostic")
	}
	if err := os.Remove(filepath.Join(root, "overview", "briefing.txt")); err != nil {
		t.Fatal(err)
	}
	scan(t, store)
	if len(store.State().Panels) != 0 {
		t.Fatal("confirmed deletion retained panel")
	}
}

func TestDuplicateStemsAndLimitsAreValidationErrors(t *testing.T) {
	t.Run("duplicate-stems", func(t *testing.T) {
		root := t.TempDir()
		write(t, root, "overview/card.md", "# Markdown\n")
		write(t, root, "overview/card.html", "<title>HTML</title>")
		var out bytes.Buffer
		if err := content.Validate(root, &out); err == nil {
			t.Fatal("duplicate stem passed validation")
		}
		if !strings.Contains(out.String(), "overview/card") {
			t.Fatalf("validation did not locate duplicate panel: %s", &out)
		}
	})
	for _, tc := range []struct {
		name, path string
		limit      int
	}{
		{"html", "overview/card.html", content.MaxHTML},
		{"text", "overview/card.txt", content.MaxText},
		{"markdown", "overview/card.md", content.MaxText},
		{"resource", "overview/assets/card/image.png", content.MaxResource},
		{"data", "overview/assets/card/data.json", content.MaxText},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if strings.Contains(tc.path, "/assets/") {
				write(t, root, "overview/card.html", "<title>Card</title>")
			}
			write(t, root, tc.path, "")
			if err := os.Truncate(filepath.Join(root, filepath.FromSlash(tc.path)), int64(tc.limit)+1); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := content.Validate(root, &out); err == nil {
				t.Fatalf("oversized %s passed validation", tc.name)
			}
			if !strings.Contains(out.String(), "card") {
				t.Fatalf("validation did not identify oversized file: %s", &out)
			}
		})
	}
}

func TestSectionMetadataRetainsLastAcceptedSettings(t *testing.T) {
	root := t.TempDir()
	write(t, root, "overview/card.txt", "Ready")
	write(t, root, "overview/_section.json", `{"version":1,"title":"My overview","order":7,"accent":"blue"}`)
	store := openStore(t, root)
	first := store.State().Tabs[0]
	if first.Title != "My overview" || first.Order != 7 || first.Accent != "blue" {
		t.Fatalf("metadata not applied: %#v", first)
	}
	write(t, root, "overview/_section.json", `{"version":1,"title":"Bad","unknown":true}`)
	scan(t, store)
	next := store.State().Tabs[0]
	if next.Title != first.Title || next.Order != first.Order || next.Accent != first.Accent || len(next.Diagnostics) == 0 {
		t.Fatalf("bad metadata did not retain prior settings: %#v", next)
	}
}

func TestSymlinksAndCrossPanelResourcesRejected(t *testing.T) {
	t.Run("root", func(t *testing.T) {
		base := t.TempDir()
		actual := filepath.Join(base, "actual")
		if err := os.Mkdir(actual, 0755); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(base, "link")
		if err := os.Symlink(actual, link); err != nil {
			t.Fatal(err)
		}
		if _, err := content.New(link); err == nil {
			t.Fatal("symlink root accepted")
		}
	})
	for _, kind := range []string{"document", "resource"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "outside.txt", "outside")
			var dest string
			if kind == "document" {
				dest = "overview/card.txt"
			} else {
				write(t, root, "overview/card.html", `<title>Card</title><script src="assets/card/app.js"></script>`)
				dest = "overview/assets/card/app.js"
			}
			path := filepath.Join(root, filepath.FromSlash(dest))
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Join(root, "outside.txt"), path); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := content.Validate(root, &out); err == nil {
				t.Fatalf("symlink %s accepted", kind)
			}
		})
	}
	for _, src := range []string{"../other/picture.png", "assets/other/picture.png", "https://example.com/picture.png"} {
		t.Run(src, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "overview/card.html", `<title>Card</title><img alt="Test image" src="`+src+`">`)
			var out bytes.Buffer
			if err := content.Validate(root, &out); err == nil {
				t.Fatalf("out-of-bundle resource accepted: %s", src)
			}
		})
	}
}

func TestValidateReportsExactHashes(t *testing.T) {
	root := t.TempDir()
	doc := `<title>Card</title><p>Ready</p>`
	data := `{"version":1,"data":{"value":3}}`
	write(t, root, "overview/card.html", doc)
	write(t, root, "overview/assets/card/data.json", data)
	var out bytes.Buffer
	if err := content.Validate(root, &out); err != nil {
		t.Fatalf("valid fixture failed: %v\n%s", err, &out)
	}
	for _, want := range []string{"overview/card", content.Hash([]byte(doc)), content.Hash([]byte(data))} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("validation missing %q: %s", want, &out)
		}
	}
}

func TestPanelLimitIsDeterministic(t *testing.T) {
	root := t.TempDir()
	for i := content.MaxPanels; i >= 0; i-- {
		write(t, root, fmt.Sprintf("overview/panel-%03d.txt", i), "Ready")
	}
	store := openStore(t, root)
	state := store.State()
	if len(state.Panels) != content.MaxPanels {
		t.Fatalf("accepted %d panels; limit %d", len(state.Panels), content.MaxPanels)
	}
	if state.Panels[0].ID != "overview/panel-000" || state.Panels[len(state.Panels)-1].ID != fmt.Sprintf("overview/panel-%03d", content.MaxPanels-1) {
		t.Fatal("panel admission is not in bytewise full-ID order")
	}
	if len(state.Diagnostics) == 0 {
		t.Fatal("panel limit rejection has no diagnostic")
	}
}

func TestInvalidHTMLMetadataAndMissingResources(t *testing.T) {
	for name, doc := range map[string]string{
		"size":                      `<meta name="lcars:size" content="giant">`,
		"order":                     `<meta name="lcars:order" content="first">`,
		"accent":                    `<meta name="lcars:accent" content="plaid">`,
		"observation":               `<meta name="lcars:updatedAt" content="yesterday">`,
		"stale-without-observation": `<meta name="lcars:staleAfterSeconds" content="30">`,
		"source-url":                `<meta name="lcars:sourceUrl" content="javascript:alert(1)">`,
		"missing-resource":          `<script src="assets/card/missing.js"></script>`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, "overview/card.html", "<title>Card</title>"+doc)
			p := panel(t, openStore(t, root), "overview/card")
			if p.DocHash != "" || p.BundleHash != "" || len(p.Diagnostics) == 0 {
				t.Fatalf("invalid first publication was accepted: %#v", p)
			}
		})
	}
}

func TestPassiveSVGRejectsActiveContent(t *testing.T) {
	for name, svg := range map[string]string{
		"script":         `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"handler":        `<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><path d="M0 0"/></svg>`,
		"animation":      `<svg xmlns="http://www.w3.org/2000/svg"><animate attributeName="x" values="0;10"/></svg>`,
		"external-image": `<svg xmlns="http://www.w3.org/2000/svg"><image href="https://example.com/a.png"/></svg>`,
	} {
		for _, ext := range []string{"svg", "SVG"} {
			t.Run(name+"."+ext, func(t *testing.T) {
				root := t.TempDir()
				write(t, root, "overview/card.html", `<title>Card</title><img alt="Diagram" src="assets/card/diagram.`+ext+`">`)
				write(t, root, "overview/assets/card/diagram."+ext, svg)
				var out bytes.Buffer
				if err := content.Validate(root, &out); err == nil {
					t.Fatalf("active SVG %s.%s passed validation", name, ext)
				}
			})
		}
	}
}

func TestPassiveSVGAcceptsStaticDiagram(t *testing.T) {
	root := t.TempDir()
	write(t, root, "overview/card.html", `<title>Card</title><img alt="Demand connection diagram" src="assets/card/diagram.svg">`)
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 40" role="img" aria-labelledby="title"><title id="title">Connection</title><defs><clipPath id="bounds"><rect width="100" height="40"/></clipPath></defs><g clip-path="url(#bounds)"><path d="M10 20 H90" stroke="#9999ff" stroke-width="4"/><circle cx="10" cy="20" r="6" fill="#ffcc99"/><text x="50" y="15" text-anchor="middle">Grid</text></g></svg>`
	write(t, root, "overview/assets/card/diagram.svg", svg)
	store := openStore(t, root)
	p := panel(t, store, "overview/card")
	accepted(t, p)
	r, ok := store.Resource(p.BundleHash, p.ID, "assets/card/diagram.svg")
	if !ok || string(r.Bytes) != svg || r.MIME != "image/svg+xml" {
		t.Fatal("passive SVG bytes not published as an image")
	}
}

func TestLiveDataRefusesSymlinkedAncestorAfterCodeAcceptance(t *testing.T) {
	root := t.TempDir()
	write(t, root, "overview/card.html", `<title>Card</title>`)
	write(t, root, "overview/assets/card/data.json", `{"version":1,"data":{"value":1}}`)
	store := openStore(t, root)
	first := panel(t, store, "overview/card")
	accepted(t, first)
	outside := t.TempDir()
	write(t, outside, "card/data.json", `{"version":1,"data":{"value":99}}`)
	assets := filepath.Join(root, "overview", "assets")
	if err := os.RemoveAll(assets); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, assets); err != nil {
		t.Fatal(err)
	}
	scan(t, store)
	next := panel(t, store, first.ID)
	if next.Data.Hash != first.Data.Hash || len(next.Diagnostics) == 0 {
		t.Fatal("data scan followed a symlinked ancestor outside the accepted panel directory")
	}
}

func TestRunningStoreRefusesReplacementRootSymlink(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "content")
	write(t, root, "overview/card.txt", "Original")
	store := openStore(t, root)
	first := panel(t, store, "overview/card")
	outside := filepath.Join(base, "outside")
	write(t, outside, "overview/card.txt", "Unexpected external replacement")
	if err := os.Rename(root, filepath.Join(base, "saved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, root); err != nil {
		t.Fatal(err)
	}
	if err := store.Scan(); err == nil {
		t.Fatal("replacement symlink root was scanned successfully")
	}
	next := panel(t, store, first.ID)
	if next.DocHash != first.DocHash || next.BundleHash != first.BundleHash || len(store.State().Diagnostics) == 0 {
		t.Fatal("replacement root symlink displaced accepted content")
	}
}
