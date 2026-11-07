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

// TestModalsAndRelativeTime covers the text-only host modal requested by a
// component, the <lcars-time> element and the panel time button.
func TestModalsAndRelativeTime(t *testing.T) {
	browser := browserPath(t)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "overview"), 0700); err != nil {
		t.Fatal(err)
	}
	html := `<!doctype html><title>Modal test</title>
<p>Last backup <lcars-time id="when" datetime="2026-01-01T00:00:00Z" label="Last backup"></lcars-time></p>
<p>Unknown <lcars-time id="unknown" datetime="nonsense"></lcars-time></p>
<button id="open" type="button" style="width:200px">Open</button>
<script>window.result='pending';document.querySelector('#open').addEventListener('click',()=>LCARS.openModal({title:'Host vault',body:'Disk nearly full.\n\n<b>not markup</b>',fields:[{label:'Disk',value:'86%'},{label:'Checked',time:'2026-01-01T00:00:00Z'}]}).then(v=>{window.result=String(v)}));</script>`
	if err := os.WriteFile(filepath.Join(root, "overview", "modal.html"), []byte(html), 0600); err != nil {
		t.Fatal(err)
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
	if err = chromedp.Do(ctx, chromedp.EmulateViewport(1200, 900), chromedp.Navigate(srv.URL+"/s/overview")); err != nil {
		t.Fatal(err)
	}
	if _, err = chromedp.Run(ctx, chromedp.Poll[bool](`!!document.querySelector('iframe:not(.frame-candidate)')`)); err != nil {
		t.Fatal(err)
	}
	var frameTarget target.ID
	if err = chromedp.Do(ctx, chromedp.Func(func(ctx context.Context, tgt *chromedp.Target) error {
		result, err := cdp.Call(ctx, tgt, target.GetTargets, target.GetTargetsParams{})
		if err != nil {
			return err
		}
		for _, info := range result.TargetInfos {
			if info.Type == "iframe" && strings.Contains(info.URL, "/component/") {
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
	check(frameCtx, "relative time renders compactly", `/^\d+d$/.test(document.querySelector('#when button').textContent) && document.querySelector('#unknown button').disabled && document.querySelector('#unknown button').textContent==='—'`)

	// Without a user gesture the request is refused rather than thrown.
	refused, err := chromedp.Run(frameCtx, chromedp.Evaluate[string](`LCARS.openModal({title:'Sneaky'}).then(String)`, chromedp.EvalAwaitPromise))
	if err != nil || refused != "false" {
		t.Fatalf("modal without user activation = %q, %v; want refused", refused, err)
	}
	check(ctx, "refused modal stays closed", `!document.querySelector('#modal').open`)

	// Real mouse input, so the frame gets user activation. Headless Chrome can
	// drop input aimed at an out-of-process frame that has not painted since
	// it last resized, so retry until the frame reports the press.
	click := func(selector string) {
		t.Helper()
		if _, err := chromedp.Run(frameCtx, chromedp.Evaluate[bool](`(window.pressed=false,window.watching||document.addEventListener('pointerdown',()=>{window.pressed=true},true),window.watching=true)`)); err != nil {
			t.Fatal(err)
		}
		for attempt := 0; attempt < 10; attempt++ {
			point, err := chromedp.Run(frameCtx, chromedp.Evaluate[[]float64](fmt.Sprintf(`(r=>[r.x+r.width/2,r.y+r.height/2])(document.querySelector(%q).getBoundingClientRect())`, selector)))
			if err != nil {
				t.Fatal(err)
			}
			origin, err := chromedp.Run(ctx, chromedp.Evaluate[[]float64](`(r=>[r.x,r.y])(document.querySelector('iframe').getBoundingClientRect())`))
			if err != nil {
				t.Fatal(err)
			}
			if err = chromedp.Do(ctx, chromedp.MouseClickXY(origin[0]+point[0], origin[1]+point[1])); err != nil {
				t.Fatal(err)
			}
			if pressed, _ := chromedp.Run(frameCtx, chromedp.Poll[bool](`window.pressed`, chromedp.WithPollingTimeout(300*time.Millisecond))); pressed {
				return
			}
		}
		t.Fatalf("frame never received a click on %s", selector)
	}
	click("#open")
	check(ctx, "component modal opens with text-only content", `(()=>{const m=document.querySelector('#modal');return m.open&&document.querySelector('#modal-title').textContent==='Host vault'&&m.querySelectorAll('.modal-body p').length===2&&!m.querySelector('b')&&m.textContent.includes('86%')&&!!m.querySelector('dd time[datetime]')})()`)
	if err = chromedp.Do(ctx, chromedp.KeyEvent(kb.Escape)); err != nil {
		t.Fatal(err)
	}
	check(ctx, "Escape closes the modal", `!document.querySelector('#modal').open && document.activeElement===document.querySelector('iframe')`)
	check(frameCtx, "openModal resolves and focus returns", `window.result==='true' && document.activeElement===document.querySelector('#open')`)

	click("#when button")
	check(ctx, "relative time opens exact-time modal", `document.querySelector('#modal').open && document.querySelector('#modal-title').textContent==='Last backup' && !!document.querySelector('#modal time[datetime]')`)
	if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`(document.querySelector('#modal-close').click(),true)`)); err != nil {
		t.Fatal(err)
	}
	check(ctx, "close button closes", `!document.querySelector('#modal').open`)

	if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`(document.querySelector('.panel-time').click(),true)`)); err != nil {
		t.Fatal(err)
	}
	check(ctx, "panel time opens time details", `document.querySelector('#modal').open && document.querySelector('#modal-title').textContent==='Modal test' && [...document.querySelectorAll('#modal dt')].map(d=>d.textContent).join()==='Updated,Observed'`)
}
