package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/matryer/lcars/internal/content"
	"github.com/matryer/lcars/internal/server"
)

// TestPreloadedSectionsMorph covers the section pool and the morph between
// sections: other sections preload while parked and become ready; opening
// one shows its existing frames without reloading them; ghost frames fly
// during the morph and are gone afterwards; and the mosaic finishes level.
func TestPreloadedSectionsMorph(t *testing.T) {
	browser := browserPath(t)
	root := t.TempDir()
	block := func(title string, height int) string {
		return `<title>` + title + `</title><div style="height:` + strconv.Itoa(height) + `px">` + title + `</div>`
	}
	for name, body := range map[string]string{
		"overview/_section.json":   `{"version":1,"title":"Overview","order":1}`,
		"overview/tall.html":       block("Tall", 520),
		"overview/short.html":      block("Short", 140),
		"overview/medium.html":     block("Medium", 300),
		"overview/note.md":         "# Note\n\nA short note.",
		"work/_section.json":       `{"version":1,"title":"Work","order":2}`,
		"work/plan.html":           block("Plan", 260),
		"work/tasks.md":            "# Tasks\n\n- One\n- Two\n- Three",
		"work/later/_section.json": `{"version":1,"title":"Later"}`,
		"work/later/someday.md":    "# Someday\n\nMaybe.",
		"notes/_section.json":      `{"version":1,"title":"Notes","order":3}`,
		"notes/scratch.md":         "# Scratch\n\nScribbles.",
		"notes/wide.html":          `<title>Wide</title><meta name="lcars:size" content="wide"><div style="height:200px">Wide</div>`,
		"notes/another.html":       block("Another", 380),
		"notes/third.html":         block("Third", 160),
		"notes/fourth.html":        block("Fourth", 240),
		"notes/fifth.md":           "# Fifth\n\nFive.",
		"notes/sixth.html":         block("Sixth", 90),
		"notes/seventh.html":       block("Seventh", 450),
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
	// Count component document loads (each frame load carries a nonce), so a reload would show.
	var loads atomic.Int64
	handler := server.New(store)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/component/") && r.URL.Query().Get("nonce") != "" {
			loads.Add(1)
		}
		handler.ServeHTTP(w, r)
	}))
	defer srv.Close()
	deadline, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	allocator, stopAllocator := chromedp.NewExecAllocator(deadline, append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(browser))...)
	defer stopAllocator()
	ctx, stopBrowser := chromedp.NewContext(allocator)
	defer stopBrowser()
	if err = chromedp.Do(ctx, chromedp.EmulateViewport(1440, 900), chromedp.Navigate(srv.URL+"/s/overview")); err != nil {
		t.Fatal(err)
	}
	check := func(what, expression string) {
		t.Helper()
		if _, err := chromedp.Run(ctx, chromedp.Poll[bool](expression, chromedp.WithPollingTimeout(10*time.Second))); err != nil {
			d, _ := chromedp.Run(ctx, chromedp.Evaluate[string](`JSON.stringify({path:location.pathname,motion:document.documentElement.dataset.motion,sections:[...document.querySelectorAll('.panels')].map(c=>[c.dataset.section,c.id,c.className,c.querySelectorAll('iframe').length,c.querySelectorAll('iframe.frame-candidate').length]),ghosts:document.querySelectorAll('.morph-ghost').length})`))
			t.Fatalf("%s: %v %s", what, err, d)
		}
	}
	check("motion is on", `document.documentElement.dataset.motion==='on'`)
	// Sibling tab first (work/later is not a sibling of overview), then sections in rail order.
	check("other sections preload while parked and become ready", `(()=>{const parked=[...document.querySelectorAll('.panels.parked')];const want=['work','work/later','notes'];return want.every(s=>parked.some(c=>c.dataset.section===s))&&parked.every(c=>c.hasAttribute('inert')&&c.getAttribute('aria-hidden')==='true'&&c.id!=='panels'&&getComputedStyle(c).visibility==='hidden'&&c.getBoundingClientRect().height===0)&&parked.flatMap(c=>[...c.querySelectorAll('iframe')]).length===11&&parked.every(c=>[...c.querySelectorAll('iframe')].every(f=>!f.classList.contains('frame-candidate')))})()`)
	check("only one container is current", `document.querySelectorAll('[id="panels"]').length===1 && document.querySelector('#panels').dataset.section==='overview'`)
	before := loads.Load()
	if before != 15 {
		t.Fatalf("expected 15 frame loads (4 + 3 + 8), got %d", before)
	}
	// Navigate with motion on: ghosts fly, then leave, and the preloaded frames are shown as they are.
	ghosts, err := chromedp.Run(ctx, chromedp.Evaluate[int](`(window.preloaded=[...document.querySelectorAll('.panels[data-section="notes"] iframe')],document.querySelector('#navigation a[data-tab="notes"]').click(),document.querySelectorAll('.morph-layer .morph-ghost').length)`))
	if err != nil {
		t.Fatal(err)
	}
	if ghosts == 0 {
		t.Fatal("no ghost frames during the morph")
	}
	check("new frames wait hidden during the morph", `!document.querySelector('.morph-layer') || document.querySelector('#panels').classList.contains('morphing')`)
	check("ghosts are gone after the morph", `!document.querySelector('.morph-layer') && !document.querySelector('.morph-ghost') && !document.querySelector('.panels.morphing')`)
	check("the preloaded frames are shown, same elements", `location.pathname==='/s/notes' && document.querySelector('#panels').dataset.section==='notes' && (f=>f.length===8 && f.every((frame,i)=>frame===preloaded[i]) && f.every(frame=>frame.isConnected && !frame.closest('[inert]')))([...document.querySelectorAll('#panels iframe')]) && !!document.querySelector('.panels.parked[data-section="overview"]')`)
	if after := loads.Load(); after != before {
		t.Fatalf("navigating to a preloaded section loaded %d frame documents", after-before)
	}
	// The mosaic leaves no visible hole: in every column, panels and tiles run
	// from the top to one shared bottom with only the regular gap between them.
	check("the mosaic finishes level with no holes", `(()=>{
		const c=document.querySelector('#panels'), cols=getComputedStyle(c).gridTemplateColumns.split(' ').length;
		if (cols!==3 || !c.querySelector(':scope > .deco')) return false;
		const box=c.getBoundingClientRect(), w=box.width/cols, items=[...c.children].filter(e=>e.matches('.panel,.deco')).map(e=>e.getBoundingClientRect());
		const bottoms=[];
		for (let i=0;i<cols;i++) {
			const x=box.left+w*i+w/2, column=items.filter(r=>r.left<=x&&r.right>=x).sort((a,b)=>a.top-b.top);
			if (!column.length || column[0].top-box.top>4) return false;
			for (let j=1;j<column.length;j++) if (column[j].top-column[j-1].bottom>32+8 || column[j].top<column[j-1].bottom-1) return false;
			bottoms.push(column.at(-1).bottom);
		}
		return Math.max(...bottoms)-Math.min(...bottoms)<=4;
	})()`)
	// Back returns to the parked overview, again without reloading.
	if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`(history.back(),true)`)); err != nil {
		t.Fatal(err)
	}
	check("back shows the parked section", `location.pathname==='/s/overview' && document.querySelectorAll('#panels iframe').length===4 && !document.querySelector('.morph-layer')`)
	if after := loads.Load(); after != before {
		t.Fatalf("returning to a parked section loaded %d frame documents", after-before)
	}
}
