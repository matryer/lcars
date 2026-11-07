package web_test

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
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
		"systems/load.html":             `<title>Load</title><meta name="lcars:size" content="wide"><p>Wide panel.</p>`,
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
	check("section page shows its own panels and a sub-tab bar", `document.querySelector('#section-title').dataset.text==='Systems' && `+heading+`==='Hosts,Load' && [...document.querySelectorAll('#subnav a')].map(a=>a.dataset.tab+(a.getAttribute('aria-current')?'*':'')).join()==='systems*,systems/network' && [...document.querySelectorAll('#navigation a')].map(a=>a.dataset.tab).join()==='systems,projects'`)
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

	key := func(k string) {
		t.Helper()
		if _, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`(document.activeElement&&document.activeElement.blur(),true)`)); err != nil {
			t.Fatal(err)
		}
		if err := chromedp.Do(ctx, chromedp.KeyEvent(k)); err != nil {
			t.Fatal(err)
		}
	}
	key("2")
	check("number keys open sections", `window.marker===1 && location.pathname==='/s/projects/garden'`)
	key("1")
	check("1 opens the first section", `location.pathname==='/s/systems' && `+heading+`==='Hosts,Load'`)
	key("]")
	check("] steps to the next tab", `location.pathname==='/s/systems/network'`)
	key("]")
	check("] wraps around", `location.pathname==='/s/systems'`)
	key("[")
	check("[ steps back", `location.pathname==='/s/systems/network'`)

	key("1")
	check("back on the section page", `location.pathname==='/s/systems' && document.querySelectorAll('#panels iframe').length===2`)
	// Expand lifts a panel, in place, into a large overlay: the same frame,
	// the rest of the page inert, and the shortcut keys quiet until it closes.
	if err = chromedp.Do(ctx, chromedp.EmulateViewport(1440, 900)); err != nil {
		t.Fatal(err)
	}
	check("three columns", `getComputedStyle(document.querySelector('#panels')).gridTemplateColumns.split(' ').length===3`)
	outside := `['.rail','#subnav','.masthead','footer'].every(s=>document.querySelector(s).inert)`
	for _, which := range []string{"first", "last"} {
		pick := `[...document.querySelectorAll('#panels .panel')].at(` + map[string]string{"first": "0", "last": "-1"}[which] + `)`
		if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`(window.panelRef=`+pick+`,window.frameRef=panelRef.querySelector('iframe'),window.expandRef=panelRef.querySelector('.panel-expand'),expandRef.click(),true)`)); err != nil {
			t.Fatal(err)
		}
		check("expand opens the "+which+" panel in an overlay", `(w=>w.classList.contains('lifted') && getComputedStyle(w).position==='fixed' && w.getAttribute('role')==='dialog' && w.getAttribute('aria-modal')==='true' && expandRef.getAttribute('aria-pressed')==='true' && expandRef.textContent==='Close' && w.getBoundingClientRect().width > innerWidth*0.8 && w.getBoundingClientRect().height > innerHeight*0.75 && !!document.querySelector('#panels > .panel-placeholder') && !!document.querySelector('.expand-backdrop') && `+outside+` && !w.closest('[inert]') && document.activeElement===expandRef && frameRef.isConnected && frameRef===w.querySelector('iframe') && !document.querySelector('.morph-layer'))(panelRef)`)
		if which == "first" {
			key("2")
			check("shortcut keys do nothing while the overlay is open", `location.pathname==='/s/systems' && panelRef.classList.contains('lifted')`)
			if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`(expandRef.focus(),true)`)); err != nil {
				t.Fatal(err)
			}
			if err = chromedp.Do(ctx, chromedp.KeyEvent(kb.Escape)); err != nil {
				t.Fatal(err)
			}
		} else if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`(document.querySelector('.expand-backdrop').click(),true)`)); err != nil {
			t.Fatal(err)
		}
		check(map[string]string{"first": "Escape", "last": "a backdrop click"}[which]+" closes the overlay", `(w=>!w.classList.contains('lifted') && getComputedStyle(w).position!=='fixed' && !w.hasAttribute('role') && !document.querySelector('.panel-placeholder') && !document.querySelector('.expand-backdrop') && !document.querySelector('[inert]:not(.parked)') && expandRef.textContent==='Expand' && expandRef.getAttribute('aria-pressed')==='false' && document.activeElement===expandRef && frameRef.isConnected && frameRef===w.querySelector('iframe') && !document.querySelector('.morph-layer'))(panelRef)`)
	}
	// Navigating away closes an open overlay first.
	if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`(document.querySelector('#panels .panel .panel-expand').click(),true)`)); err != nil {
		t.Fatal(err)
	}
	check("overlay open again", `!!document.querySelector('.panel.lifted')`)
	if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`(history.back(),true)`)); err != nil {
		t.Fatal(err)
	}
	check("navigation closes the overlay", `location.pathname!=='/s/systems' && !document.querySelector('.panel.lifted') && !document.querySelector('.expand-backdrop') && !document.querySelector('[inert]:not(.parked)')`)
}
