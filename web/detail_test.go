package web_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
	"github.com/matryer/lcars/internal/content"
	"github.com/matryer/lcars/internal/server"
)

// TestDetailViewsAndRelativeTime covers in-panel detail views opened by a
// component (and their breadcrumb and Back control on the host frame), the
// <lcars-time> element, the panel time details row and LCARS.navigate.
func TestDetailViewsAndRelativeTime(t *testing.T) {
	browser := browserPath(t)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "overview"), 0700); err != nil {
		t.Fatal(err)
	}
	html := `<!doctype html><title>Today</title>
<p>Last backup <lcars-time id="when" datetime="2026-01-01T00:00:00Z" label="Last backup"></lcars-time></p>
<p>Unknown <lcars-time id="unknown" datetime="nonsense"></lcars-time></p>
<label><input id="keep" type="checkbox"> Keep</label>
<button id="open" type="button" style="width:200px">Open</button>
<button id="go" type="button" style="width:200px">Go</button>
<button id="bad" type="button" style="width:200px">Bad</button>
<script>window.result='pending';document.querySelector('#open').addEventListener('click',()=>{window.result='pending';LCARS.openDetail({title:'Dentist',body:'Bring the form.\n\n<b>not markup</b>',fields:[{label:'Where',value:'High St'},{label:'Starts',time:'2026-01-01T00:00:00Z'}]}).then(v=>{window.result=String(v)})});
document.querySelector('#go').addEventListener('click',()=>LCARS.navigate('elsewhere').then(v=>{window.went=v}));
document.querySelector('#bad').addEventListener('click',()=>LCARS.navigate('no-such-section').then(v=>{window.bad=v}));</script>`
	if err := os.WriteFile(filepath.Join(root, "overview", "today.html"), []byte(html), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "elsewhere"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "elsewhere", "note.md"), []byte("# Elsewhere\n\nArrived."), 0600); err != nil {
		t.Fatal(err)
	}
	// Preloading may load other pages' frames too; this is the panel under test.
	const hostFrame = `[...document.querySelectorAll('iframe:not(.frame-candidate)')].find(f=>f.src.includes('today'))`
	store, err := content.New(root)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(server.New(store))
	defer srv.Close()
	deadline, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	allocator, stopAllocator := chromedp.NewExecAllocator(deadline, append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(browser))...)
	defer stopAllocator()
	ctx, stopBrowser := chromedp.NewContext(allocator)
	defer stopBrowser()
	if err = chromedp.Do(ctx, chromedp.EmulateViewport(1200, 900), chromedp.Navigate(srv.URL+"/s/overview")); err != nil {
		t.Fatal(err)
	}
	if _, err = chromedp.Run(ctx, chromedp.Poll[bool](`!!(`+hostFrame+`)`)); err != nil {
		t.Fatal(err)
	}
	var frameTarget target.ID
	if err = chromedp.Do(ctx, chromedp.Func(func(ctx context.Context, tgt *chromedp.Target) error {
		result, err := cdp.Call(ctx, tgt, target.GetTargets, target.GetTargetsParams{})
		if err != nil {
			return err
		}
		for _, info := range result.TargetInfos {
			if info.Type == "iframe" && strings.Contains(info.URL, "/component/") && strings.Contains(info.URL, "today") {
				frameTarget = info.TargetID
				return nil
			}
		}
		return fmt.Errorf("component iframe target not found")
	})); err != nil {
		t.Fatal(err)
	}
	frameCtx, stopFrame := chromedp.NewContext(ctx, chromedp.WithTargetID(frameTarget))
	defer stopFrame()
	check := func(c context.Context, what, expression string) {
		t.Helper()
		if _, err := chromedp.Run(c, chromedp.Poll[bool](expression, chromedp.WithPollingTimeout(5*time.Second))); err != nil {
			t.Fatalf("%s: %v", what, err)
		}
	}
	run := func(c context.Context, expression string) {
		t.Helper()
		if _, err := chromedp.Run(c, chromedp.Evaluate[bool](`(`+expression+`,true)`)); err != nil {
			t.Fatal(err)
		}
	}
	// Expressions inside the frame. The detail view renders in a shadow root.
	const (
		shadow     = `document.querySelector('.lcars-detail')?.shadowRoot`
		detailOpen = `(()=>{const c=document.querySelector('.lcars-content'),v=document.querySelector('.lcars-detail');return document.documentElement.dataset.lcarsDetail==='open'&&!!v&&c.inert&&getComputedStyle(c).visibility==='hidden'&&getComputedStyle(c).position==='absolute'})()`
		detailGone = `(()=>{const c=document.querySelector('.lcars-content');return !document.querySelector('.lcars-detail')&&!document.documentElement.dataset.lcarsDetail&&!c.inert&&getComputedStyle(c).visibility==='visible'})()`
		// Host expressions for the breadcrumb and Back control on the panel frame.
		crumbShown = `(t=>{const c=document.querySelector('.panel-crumb'),b=document.querySelector('.panel-back');return !!c&&!c.hidden&&!b.hidden&&c.querySelector('.crumb-title').dataset.text===t&&c.previousElementSibling===document.querySelector('.panel-heading h2')&&document.querySelector('.panel-heading h2').dataset.text==='Today'})`
		crumbGone  = `(()=>{const c=document.querySelector('.panel-crumb'),b=document.querySelector('.panel-back');return (!c||c.hidden)&&(!b||b.hidden)})()`
	)
	check(frameCtx, "relative time renders compactly", `/^\d+d$/.test(document.querySelector('#when button').textContent) && document.querySelector('#unknown button').disabled && document.querySelector('#unknown button').textContent==='—'`)
	check(ctx, "the host has no modal dialog", `!document.querySelector('dialog')`)
	// Before any click: navigate needs user activation in the frame.
	if went, err := chromedp.Run(frameCtx, chromedp.Evaluate[bool](`LCARS.navigate('elsewhere')`, chromedp.EvalAwaitPromise)); err != nil || went {
		t.Fatalf("navigate without user activation = %v, %v; want refused", went, err)
	}

	// Real mouse input on an element found by a frame expression. Headless
	// Chrome can drop input aimed at an out-of-process frame that has not
	// painted since it last resized, so retry until the frame reports the press.
	click := func(element string) {
		t.Helper()
		if _, err := chromedp.Run(frameCtx, chromedp.Evaluate[bool](`(window.pressed=null,window.watching||document.addEventListener('pointerdown',e=>{window.pressed=e.composedPath()},true),window.watching=true)`)); err != nil {
			t.Fatal(err)
		}
		for attempt := 0; attempt < 10; attempt++ {
			// Wait until the panel stops moving (slots animate their height), then aim.
			if _, err := chromedp.Run(ctx, chromedp.Poll[bool](`new Promise(done=>{const f=`+hostFrame+`,a=JSON.stringify(f.getBoundingClientRect());setTimeout(()=>done(a===JSON.stringify(f.getBoundingClientRect())),120)})`, chromedp.WithPollingTimeout(3*time.Second))); err != nil {
				t.Fatal(err)
			}
			point, err := chromedp.Run(frameCtx, chromedp.Evaluate[[]float64](`(r=>[r.x+r.width/2,r.y+r.height/2])((window.aim=`+element+`).getBoundingClientRect())`))
			if err != nil {
				t.Fatal(err)
			}
			origin, err := chromedp.Run(ctx, chromedp.Evaluate[[]float64](`(r=>[r.x,r.y])((`+hostFrame+`).getBoundingClientRect())`))
			if err != nil {
				t.Fatal(err)
			}
			if err = chromedp.Do(ctx, chromedp.MouseClickXY(origin[0]+point[0], origin[1]+point[1])); err != nil {
				t.Fatal(err)
			}
			if pressed, _ := chromedp.Run(frameCtx, chromedp.Poll[bool](`!!window.pressed`, chromedp.WithPollingTimeout(300*time.Millisecond))); pressed {
				if hit, _ := chromedp.Run(frameCtx, chromedp.Evaluate[bool](`window.pressed.includes(window.aim)`)); !hit {
					t.Fatalf("a click aimed at %s landed elsewhere", element)
				}
				return
			}
		}
		t.Fatalf("frame never received a click on %s", element)
	}
	q := func(selector string) string { return fmt.Sprintf(`document.querySelector(%q)`, selector) }
	backButton := shadow + `.querySelector('.back')`

	// Local state that must survive a detail view.
	run(frameCtx, `document.querySelector('#keep').checked=true`)
	click(q("#open"))
	check(frameCtx, "a click opens the detail view in the panel", detailOpen+` && (()=>{const r=`+shadow+`;return r.querySelector('h2').textContent==='Dentist'&&r.querySelectorAll('p').length===2&&!r.querySelector('b')&&r.textContent.includes('High St')&&!!r.querySelector('dd time[datetime]')&&/ago$/.test(r.querySelector('dd .ago').textContent)&&r.activeElement===r.querySelector('h2')})()`)
	check(ctx, "the host shows a breadcrumb and Back control", crumbShown+`('Dentist')`)
	click(backButton)
	check(frameCtx, "Back restores the content, its state and focus", detailGone+` && document.querySelector('#keep').checked && document.activeElement===document.querySelector('#open') && window.result==='true'`)
	check(ctx, "going back clears the breadcrumb", crumbGone)

	click(q("#open"))
	check(frameCtx, "the detail reopens", detailOpen)
	if err = chromedp.Do(ctx, chromedp.KeyEvent(kb.Escape)); err != nil {
		t.Fatal(err)
	}
	check(frameCtx, "Escape goes back", detailGone+` && document.activeElement===document.querySelector('#open') && window.result==='true'`)
	check(ctx, "Escape clears the breadcrumb", crumbGone)

	click(q("#open"))
	check(ctx, "the breadcrumb returns", crumbShown+`('Dentist')`)
	run(ctx, `document.querySelector('.panel-back').click()`)
	check(frameCtx, "the host Back control goes back", detailGone+` && document.querySelector('#keep').checked && document.activeElement===document.querySelector('#open') && window.result==='true'`)
	check(ctx, "host Back clears the breadcrumb and focuses the frame", crumbGone+` && document.activeElement===(`+hostFrame+`)`)

	// openModal is an alias, needs no user gesture now that it stays in the
	// panel, and a new detail replaces the open one.
	replaced, err := chromedp.Run(frameCtx, chromedp.Evaluate[bool](`(async()=>{const first=LCARS.openDetail({title:'One'});const second=LCARS.openModal({title:'Two'});const r=await first;return LCARS.openModal===LCARS.openDetail&&r===false&&`+shadow+`.querySelector('h2').textContent==='Two'&&document.querySelectorAll('.lcars-detail').length===1})()`, chromedp.EvalAwaitPromise))
	if err != nil || !replaced {
		t.Fatalf("replacing a detail = %v, %v", replaced, err)
	}
	check(ctx, "the breadcrumb follows the replacement", crumbShown+`('Two')`)
	click(backButton)
	check(frameCtx, "Back from a replaced detail", detailGone)

	click(q("#when button"))
	check(frameCtx, "relative time opens an exact-time detail", detailOpen+` && `+shadow+`.querySelector('h2').textContent==='Last backup' && !!`+shadow+`.querySelector('dd time[datetime="2026-01-01T00:00:00.000Z"]')`)
	check(ctx, "relative time breadcrumb", crumbShown+`('Last backup')`)
	if err = chromedp.Do(ctx, chromedp.KeyEvent(kb.Escape)); err != nil {
		t.Fatal(err)
	}
	check(frameCtx, "Escape returns focus to the relative time", detailGone+` && document.activeElement===document.querySelector('#when button')`)

	// The panel time button toggles an inline row of exact times.
	run(ctx, `document.querySelector('[data-panel-id="overview/today"] .panel-time').click()`)
	check(ctx, "panel time shows inline time details", `(()=>{const b=document.querySelector('[data-panel-id="overview/today"] .panel-time'),row=document.getElementById(b.getAttribute('aria-controls'));return !!row&&!row.hidden&&b.getAttribute('aria-expanded')==='true'&&!b.hasAttribute('aria-haspopup')&&row.closest('.panel-meta')===b.parentElement&&[...row.querySelectorAll('dt')].map(d=>d.textContent).join()==='Updated,Observed'&&!!row.querySelector('dd time[datetime]')&&/^\d+m$/.test(row.querySelector('[data-relative]').textContent)&&!document.querySelector('dialog')})()`)
	run(ctx, `document.querySelector('[data-panel-id="overview/today"] .panel-time').click()`)
	check(ctx, "panel time hides inline time details", `(()=>{const b=document.querySelector('[data-panel-id="overview/today"] .panel-time');return document.getElementById(b.getAttribute('aria-controls')).hidden&&b.getAttribute('aria-expanded')==='false'})()`)

	// navigate: refused for unknown sections; allowed from a click.
	click(q("#bad"))
	check(frameCtx, "unknown section is refused", `window.bad===false`)
	check(ctx, "refused navigation stays put", `location.pathname==='/s/overview'`)
	click(q("#go"))
	check(ctx, "navigate opens the section in place", `location.pathname==='/s/elsewhere' && [...document.querySelectorAll('#panels .panel h2')].map(h=>h.dataset.text).join()==='Elsewhere'`)
}
