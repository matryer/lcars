package capture

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/matryer/lcars/internal/content"
	"github.com/matryer/lcars/internal/server"
)

const interactiveDocument = `<!doctype html><title>Interactive fixture</title><label><input type="checkbox" id="selection"> Keep selection</label><output id="value"></output><script>
let deliveries=0;
function report(){parent.postMessage({testReport:true,checked:document.querySelector('#selection').checked,value:document.querySelector('#value').textContent,deliveries},'*')}
addEventListener('message',e=>{if(e.data?.testToggle){document.querySelector('#selection').checked=true;report()}});
LCARS.onData(({data})=>{deliveries++;document.querySelector('#value').textContent=String(data.value);report();if(data.value===3)throw Error('deliberate data callback failure')});
</script>`

func interactiveFixture(t *testing.T) (*content.Store, *httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	asset := filepath.Join(root, "overview/assets/status")
	if err := os.MkdirAll(asset, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "overview/status.html"), []byte(interactiveDocument), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(asset, "data.json"), []byte(`{"version":1,"data":{"value":1}}`), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := content.New(root)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(server.New(store))
	t.Cleanup(srv.Close)
	return store, srv, root
}

func browserContext(t *testing.T) context.Context {
	t.Helper()
	executable, err := browserPath("")
	if err != nil {
		t.Skip(err)
	}
	ctx, deadline := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(deadline)
	allocator, stop := chromedp.NewExecAllocator(ctx, append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(executable))...)
	t.Cleanup(stop)
	browser, cancel := chromedp.NewContext(allocator)
	t.Cleanup(cancel)
	return browser
}

func eval[T any](t *testing.T, ctx context.Context, js string) T {
	t.Helper()
	v, err := chromedp.Run(ctx, chromedp.Evaluate[T](js, chromedp.EvalAwaitPromise))
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func poll(t *testing.T, ctx context.Context, js string) {
	t.Helper()
	if _, err := chromedp.Run(ctx, chromedp.Poll[bool](js, chromedp.WithPollingInterval(25*time.Millisecond))); err != nil {
		t.Fatal(err)
	}
}

func mountInteractive(t *testing.T, ctx context.Context, srv *httptest.Server) {
	t.Helper()
	if err := chromedp.Do(ctx, chromedp.EmulateViewport(1024, 768), chromedp.Navigate(srv.URL+"/s/overview")); err != nil {
		t.Fatal(err)
	}
	poll(t, ctx, `!!document.querySelector('[data-panel-id="overview/status"] iframe:not(.frame-candidate)')`)
	eval[bool](t, ctx, `(()=>{window.__reports=[];window.__mountedFrame=document.querySelector('iframe');addEventListener('message',e=>{if(e.data?.testReport)__reports.push(e.data)});__mountedFrame.contentWindow.postMessage({testToggle:true},'*');return true})()`)
	poll(t, ctx, `window.__reports?.some(x=>x.checked===true&&x.value==='1')`)
}

func publishData(t *testing.T, store *content.Store, root string, value int) {
	t.Helper()
	raw := []byte(fmt.Sprintf(`{"version":1,"data":{"value":%d}}`, value))
	if err := os.WriteFile(filepath.Join(root, "overview/assets/status/data.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.Scan(); err != nil {
		t.Fatal(err)
	}
}

func TestLiveDataPreservesFrameAndInteractionAndRejectedCode(t *testing.T) {
	store, srv, root := interactiveFixture(t)
	ctx := browserContext(t)
	mountInteractive(t, ctx, srv)
	initial := store.State().Panels[0]
	publishData(t, store, root, 2)
	eval[bool](t, ctx, `(()=>{document.dispatchEvent(new Event('visibilitychange'));return true})()`)
	poll(t, ctx, `window.__reports?.some(x=>x.checked===true&&x.value==='2'&&x.deliveries===2)`)
	if !eval[bool](t, ctx, `window.__mountedFrame===document.querySelector('iframe')`) {
		t.Fatal("data update replaced the iframe")
	}
	acceptedData := store.State().Panels[0].Data.Hash
	if err := os.WriteFile(filepath.Join(root, "overview/status.html"), []byte(`<meta name="lcars:size" content="wrong">`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.Scan(); err != nil {
		t.Fatal(err)
	}
	eval[bool](t, ctx, `(()=>{document.dispatchEvent(new Event('visibilitychange'));return true})()`)
	poll(t, ctx, `document.body.textContent.includes('Update rejected')`)
	if !eval[bool](t, ctx, `window.__mountedFrame===document.querySelector('iframe')`) {
		t.Fatal("rejected code replaced the iframe")
	}
	report := eval[Report](t, ctx, `window.LCARSCapture({panel:'overview/status'}).catch(()=>window.__lcarsCaptureReport)`)
	panels := report["panels"].([]any)
	panel := panels[0].(map[string]any)
	if panel["docHash"] != initial.DocHash || panel["dataHash"] != acceptedData {
		t.Fatalf("failed capture lost last displayed hashes: %+v", report)
	}
	if len(report["errors"].([]any)) == 0 {
		t.Fatal("capture silently accepted rejected code")
	}
}

func TestCallbackFailureStopsDeliveryAndReportsLastVerifiedHash(t *testing.T) {
	store, srv, root := interactiveFixture(t)
	ctx := browserContext(t)
	mountInteractive(t, ctx, srv)
	initial := store.State().Panels[0]
	publishData(t, store, root, 3)
	eval[bool](t, ctx, `(()=>{document.dispatchEvent(new Event('visibilitychange'));return true})()`)
	poll(t, ctx, `document.body.textContent.includes('deliberate data callback failure')`)
	// Observe the next accepted state fetch, ensuring a newer update has
	// reached the host before checking that the failed instance stayed stopped.
	eval[bool](t, ctx, `(()=>{const fetchOriginal=window.fetch;window.fetch=async(...args)=>{const response=await fetchOriginal(...args);window.__testStateRevision=response.headers.get('etag');return response};return true})()`)
	publishData(t, store, root, 4)
	revision := store.State().Revision
	eval[bool](t, ctx, `(()=>{document.dispatchEvent(new Event('visibilitychange'));return true})()`)
	poll(t, ctx, `window.__testStateRevision==='"`+revision+`"'`)
	eval[bool](t, ctx, `new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>resolve(true))))`)
	if eval[bool](t, ctx, `window.__reports.some(x=>x.value==='4')`) {
		t.Fatal("delivered data to failed instance")
	}
	report := eval[Report](t, ctx, `window.LCARSCapture({panel:'overview/status'}).catch(()=>window.__lcarsCaptureReport)`)
	panels := report["panels"].([]any)
	panel := panels[0].(map[string]any)
	if panel["dataHash"] != initial.Data.Hash || panel["verified"] != false {
		t.Fatalf("must report last verified hash after callback failure: %+v", report)
	}
	if !strings.Contains(fmt.Sprint(report["errors"]), "failure") {
		t.Fatalf("missing callback error: %+v", report)
	}
}
