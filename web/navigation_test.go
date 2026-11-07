package web_test

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/matryer/lcars/internal/content"
	"github.com/matryer/lcars/internal/server"
)

// TestSubSectionNavigation covers the sub-tab bar, in-page navigation, the
// first-sub-section redirect and compact times.
func TestSubSectionNavigation(t *testing.T) {
	browser := browserPath(t)
	root := t.TempDir()
	for name, body := range map[string]string{
		"systems/_section.json":         `{"version":1,"title":"Systems","order":1}`,
		"systems/hosts.md":              "# Hosts\n\nFour hosts.",
		"systems/network/_section.json": `{"version":1,"title":"Network"}`,
		"systems/network/latency.md":    "# Latency\n\n12 ms.",
		"projects/_section.json":        `{"version":1,"title":"Projects","order":2}`,
		"projects/garden/plan.md":       "# Plan\n\nGarlic.",
	} {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	store, err := content.New(root)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(server.New(store))
	defer srv.Close()
	deadline, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	allocator, stopAllocator := chromedp.NewExecAllocator(deadline, append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(browser))...)
	defer stopAllocator()
	ctx, stopBrowser := chromedp.NewContext(allocator)
	defer stopBrowser()
	if err = chromedp.Do(ctx, chromedp.EmulateViewport(1200, 900), chromedp.Navigate(srv.URL+"/s/systems")); err != nil {
		t.Fatal(err)
	}
	check := func(what, expression string) {
		t.Helper()
		if _, err := chromedp.Run(ctx, chromedp.Poll[bool](expression, chromedp.WithPollingTimeout(5*time.Second))); err != nil {
			d, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`JSON.stringify([document.querySelector('#section-title').dataset.text, [...document.querySelectorAll('#panels .panel h2')].map(h=>h.dataset.text), [...document.querySelectorAll('#subnav a')].map(a=>a.dataset.tab+(a.getAttribute('aria-current')?'*':'')), [...document.querySelectorAll('#navigation a')].map(a=>a.dataset.tab), location.pathname, document.title, document.querySelector('.panel-time')&&document.querySelector('.panel-time').textContent])`))
			t.Fatalf("%s: %v %s", what, err, d)
		}
	}
	heading := `[...document.querySelectorAll('#panels .panel h2')].map(h=>h.dataset.text).join()`
	check("section page shows its own panels and a sub-tab bar", `document.querySelector('#section-title').dataset.text==='Systems' && `+heading+`==='Hosts' && [...document.querySelectorAll('#subnav a')].map(a=>a.dataset.tab+(a.getAttribute('aria-current')?'*':'')).join()==='systems*,systems/network' && [...document.querySelectorAll('#navigation a')].map(a=>a.dataset.tab).join()==='systems,projects'`)
	check("times are never 'now'", `!!document.querySelector('.panel-time') && /^\d+[mhd]$/.test(document.querySelector('.panel-time').textContent) && document.querySelector('#section-updated').textContent!=='now'`)
	if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`(window.marker=1,document.querySelector('#subnav a[data-tab="systems/network"]').click(),true)`)); err != nil {
		t.Fatal(err)
	}
	check("sub-tab opens in place", `window.marker===1 && location.pathname==='/s/systems/network' && `+heading+`==='Latency' && document.querySelector('#subnav a[aria-current]').dataset.tab==='systems/network' && document.querySelector('#navigation a[aria-current]').dataset.tab==='systems' && document.title==='Network · Systems · LCARS'`)
	if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`(document.querySelector('#navigation a[data-tab="projects"]').click(),true)`)); err != nil {
		t.Fatal(err)
	}
	check("section with only sub-sections opens the first", `window.marker===1 && location.pathname==='/s/projects/garden' && `+heading+`==='Plan' && [...document.querySelectorAll('#subnav a')].map(a=>a.dataset.tab).join()==='projects/garden'`)
	if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`(history.back(),true)`)); err != nil {
		t.Fatal(err)
	}
	check("back returns to the previous sub-section", `window.marker===1 && location.pathname==='/s/systems/network' && `+heading+`==='Latency'`)
}
