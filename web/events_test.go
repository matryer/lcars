package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
	"github.com/matryer/lcars/internal/content"
	"github.com/matryer/lcars/internal/server"
)

func TestLiveEventsAndPollingRecovery(t *testing.T) {
	browser, err := exec.LookPath("chromium")
	if err != nil {
		t.Skip("Chromium unavailable")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "overview/assets/weather")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(root, "overview/weather.html"), []byte(`<title>Weather</title><input id="choice"><output></output><script>window.received=[];LCARS.onData(({data})=>{received.push(data);document.querySelector('output').textContent=data;});</script>`), 0600)
	dataPath := filepath.Join(dir, "data.json")
	publish := func(value int) {
		t.Helper()
		os.WriteFile(dataPath, []byte(fmt.Sprintf(`{"version":1,"data":%d}`, value)), 0600)
	}
	publish(1)
	store, err := content.New(root)
	if err != nil {
		t.Fatal(err)
	}
	var streams atomic.Int32
	var refuse atomic.Bool
	handler := server.New(store)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/events" {
			streams.Add(1)
			if refuse.Load() {
				http.Error(w, "temporary outage", 503)
				return
			}
		}
		handler.ServeHTTP(w, r)
	}))
	defer srv.Close()
	deadline, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	allocator, stop := chromedp.NewExecAllocator(deadline, append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(browser))...)
	defer stop()
	ctx, closeBrowser := chromedp.NewContext(allocator)
	defer closeBrowser()
	// Track data messages without crossing the sandbox origin. Hold one response
	// after it has been read to exercise an event arriving during an older fetch.
	inject := `window.deliveries=[];window.addEventListener('message',e=>{if(e.data?.type==='dataAck')deliveries.push(e.data.hash)});
 const nativeFetch=window.fetch;window.fetch=async(...args)=>{const response=await nativeFetch(...args);if(window.holdState&&args[0]==='/api/state'){window.holdState=false;const body=await response.text();window.held=true;await new Promise(resolve=>window.releaseState=resolve);return new Response(body,{status:response.status,headers:response.headers});}return response;};
 const NativeEventSource=window.EventSource;window.EventSource=class extends NativeEventSource{constructor(...args){super(...args);window.eventStream=this}};`
	if err = chromedp.Do(ctx, chromedp.Func(func(ctx context.Context, tgt *chromedp.Target) error {
		_, err := cdp.Call(ctx, tgt, page.AddScriptToEvaluateOnNewDocument, page.AddScriptToEvaluateOnNewDocumentParams{Source: inject})
		return err
	}), chromedp.Navigate(srv.URL+"/s/overview")); err != nil {
		t.Fatal(err)
	}
	poll := func(expression string) {
		t.Helper()
		if _, err := chromedp.Run(ctx, chromedp.Poll[bool](expression)); err != nil {
			t.Fatal(err)
		}
	}
	evaluate := func(expression string) {
		t.Helper()
		if _, err := chromedp.Run(ctx, chromedp.Evaluate[bool](expression)); err != nil {
			t.Fatal(err)
		}
	}
	poll(`!!document.querySelector('iframe:not(.frame-candidate)')`)
	if streams.Load() == 0 {
		t.Fatal("frontend never opened SSE")
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
		return fmt.Errorf("iframe target missing")
	})); err != nil {
		t.Fatal(err)
	}
	frameCtx, stopFrame := chromedp.NewContext(ctx, chromedp.WithTargetID(frameTarget))
	defer stopFrame()
	if _, err = chromedp.Run(frameCtx, chromedp.Evaluate[bool](`(document.querySelector('#choice').value='keep my selection',true)`)); err != nil {
		t.Fatal(err)
	}
	evaluate(`(window.originalFrame=document.querySelector('iframe'),window.holdState=true,true)`)
	publish(2)
	store.Scan()
	poll(`window.held===true`)
	publish(3)
	store.Scan()
	evaluate(`(window.releaseState(),true)`)
	hash := store.State().Panels[0].Data.Hash
	poll(fmt.Sprintf(`deliveries.includes(%q)`, hash))
	evaluate(`(window.eventStream.close(),true)`)
	refuse.Store(true)
	// Trigger the same handler as a disconnected native stream; retrying the
	// endpoint is exercised by re-opening through visibility lifecycle below.
	evaluate(`(window.eventStream.onerror(new Event('error')),window.eventStream.onerror(new Event('error')),true)`)
	publish(4)
	store.Scan()
	hash = store.State().Panels[0].Data.Hash
	poll(fmt.Sprintf(`deliveries.includes(%q)`, hash))
	refuse.Store(false)
	evaluate(`(document.dispatchEvent(new Event('visibilitychange')),true)`)
	poll(`window.eventStream.readyState===1`)
	publish(5)
	store.Scan()
	hash = store.State().Panels[0].Data.Hash
	poll(fmt.Sprintf(`deliveries.includes(%q)`, hash))
	preserved, err := chromedp.Run(frameCtx, chromedp.Evaluate[bool](`document.querySelector('#choice').value==='keep my selection' && document.querySelector('output').textContent==='5'`))
	if err != nil || !preserved {
		t.Fatalf("panel state lost or data not rendered: %v %v", preserved, err)
	}
	same, err := chromedp.Run(ctx, chromedp.Evaluate[bool](`originalFrame===document.querySelector('iframe')`))
	if err != nil || !same {
		t.Fatalf("frame replaced: %v %v", same, err)
	}
}
