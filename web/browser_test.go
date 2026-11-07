package web_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/matryer/lcars/internal/content"
	"github.com/matryer/lcars/internal/server"
	"strings"
)

// TestMountedComponentReflow exercises the allocated frame viewport, including
// a resized Canvas and a focused local control, independently of page size.
func TestMountedComponentReflow(t *testing.T) {
	browser := browserPath(t)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "overview"), 0700); err != nil {
		t.Fatal(err)
	}
	html := `<!doctype html><title>Long expanded component heading with useful information</title><meta name="lcars:size" content="wide"><style>label{display:flex;gap:8px;align-items:center;min-height:44px}canvas{width:100%;height:80px}.test-controls{display:flex;flex-wrap:wrap;gap:16px}.test-controls>*{min-width:0;max-width:100%}</style><label><input type="checkbox" id="choice">Preserve this choice while the component reflows</label><div class="test-controls"><button>Expanded primary action label</button><output>999,999,999 W</output></div><p>VeryLongUnbrokenTranslatedLabelThatMustRemainAvailableWithoutClippingOrChangingTheFontSize</p><canvas aria-label="Responsive test chart" role="img"></canvas><script>const canvas=document.querySelector('canvas');new ResizeObserver(()=>{canvas.width=Math.round(canvas.clientWidth*devicePixelRatio);canvas.height=Math.round(canvas.clientHeight*devicePixelRatio);const context=canvas.getContext('2d');context.fillStyle='#9999ff';context.fillRect(0,0,canvas.width,canvas.height);}).observe(canvas);</script>`
	if err := os.WriteFile(filepath.Join(root, "overview", "reflow.html"), []byte(html), 0600); err != nil {
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
	if err = chromedp.Do(ctx, chromedp.Func(func(ctx context.Context, tgt *chromedp.Target) error {
		_, err := cdp.Call(ctx, tgt, page.AddScriptToEvaluateOnNewDocument, page.AddScriptToEvaluateOnNewDocumentParams{Source: "Object.defineProperty(crypto,'randomUUID',{value:undefined})"})
		return err
	}), chromedp.EmulateViewport(1440, 900), chromedp.Navigate(srv.URL+"/s/overview")); err != nil {
		t.Fatal(err)
	}
	if _, err = chromedp.Run(ctx, chromedp.Poll[bool](`!!document.querySelector('iframe:not(.frame-candidate)')`)); err != nil {
		t.Fatal(err)
	}
	if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`(window.originalFrame=document.querySelector('iframe'),true)`)); err != nil {
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
	if _, err = chromedp.Run(frameCtx, chromedp.Evaluate[bool](`(document.querySelector('#choice').checked=true,document.querySelector('#choice').focus(),true)`)); err != nil {
		t.Fatal(err)
	}
	verify := func() {
		t.Helper()
		ok, err := chromedp.Run(frameCtx, chromedp.Evaluate[bool](`document.querySelector('#choice').checked && document.activeElement===document.querySelector('#choice') && document.documentElement.scrollWidth<=innerWidth+1 && Math.abs(document.querySelector('canvas').width-document.querySelector('canvas').clientWidth*devicePixelRatio)<2`))
		if err != nil || !ok {
			t.Fatalf("component failed reflow/state/Canvas check: ok=%v err=%v", ok, err)
		}
	}
	for _, width := range []int{260, 360, 720, 260} {
		if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](fmt.Sprintf(`(document.querySelector('iframe').style.width='%dpx',true)`, width))); err != nil {
			t.Fatal(err)
		}
		if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`new Promise(resolve=>setTimeout(()=>resolve(true),120))`, chromedp.EvalAwaitPromise)); err != nil {
			t.Fatal(err)
		}
		verify()
	}
	if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`(document.querySelector('iframe').style.width='100%',true)`)); err != nil {
		t.Fatal(err)
	}
	for _, width := range []int64{320, 390, 1024, 1440} {
		if err = chromedp.Do(ctx, chromedp.EmulateViewport(width, 844)); err != nil {
			t.Fatal(err)
		}
		if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`new Promise(resolve=>setTimeout(()=>resolve(true),120))`, chromedp.EvalAwaitPromise)); err != nil {
			t.Fatal(err)
		}
		ok, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`document.documentElement.scrollWidth<=innerWidth+1 && originalFrame===document.querySelector('iframe')`))
		if err != nil || !ok {
			t.Fatalf("page width %d overflowed or replaced local state: %v %v", width, ok, err)
		}
		verify()
	}
	// A 1440px display at 200% browser zoom has a 720px CSS viewport.
	if err = chromedp.Do(ctx, chromedp.EmulateViewport(720, 450, chromedp.EmulateScale(2))); err != nil {
		t.Fatal(err)
	}
	if _, err = chromedp.Run(ctx, chromedp.Evaluate[bool](`new Promise(resolve=>setTimeout(()=>resolve(true),120))`, chromedp.EvalAwaitPromise)); err != nil {
		t.Fatal(err)
	}
	verify()
	ok, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`document.documentElement.scrollWidth<=innerWidth+1 && originalFrame===document.querySelector('iframe')`))
	if err != nil || !ok {
		t.Fatalf("200%% zoom equivalent overflowed or replaced frame: %v %v", ok, err)
	}
}

func browserPath(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "chrome"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	t.Skip("Chrome/Chromium not installed")
	return ""
}

// TestPanelsRenderBehindHostRewritingProxy covers proxies, such as nginx by
// default, that send the upstream address as Host rather than the browser's.
func TestPanelsRenderBehindHostRewritingProxy(t *testing.T) {
	browser := browserPath(t)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "overview"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "overview", "note.md"), []byte("# Note\n\nProxied."), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := content.New(root)
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(server.New(store))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	proxy := httptest.NewServer(&httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) { r.SetURL(target) }})
	defer proxy.Close()
	deadline, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	allocator, stopAllocator := chromedp.NewExecAllocator(deadline, append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(browser))...)
	defer stopAllocator()
	ctx, stopBrowser := chromedp.NewContext(allocator)
	defer stopBrowser()
	if err = chromedp.Do(ctx, chromedp.Navigate(proxy.URL+"/s/overview")); err != nil {
		t.Fatal(err)
	}
	if _, err = chromedp.Run(ctx, chromedp.Poll[bool](`!!document.querySelector('iframe:not(.frame-candidate)')`)); err != nil {
		t.Fatalf("panel never became ready behind proxy: %v", err)
	}
}
