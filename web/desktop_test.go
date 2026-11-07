package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/matryer/lcars/internal/content"
	"github.com/matryer/lcars/internal/server"
)

// Verify the desktop bootstrap selects the first section before shell startup
// and continues fetching changes with EventSource disabled. This Chromium test
// does not exercise the native webview or the Wails runtime.
func TestDesktopNavigationAndPolling(t *testing.T) {
	browser := browserPath(t)
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "overview"), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "overview", "status.md")
	if err := os.WriteFile(file, []byte("# First publication"), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := content.New(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := server.NewDesktop(store)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/wails/runtime.js" {
			// Chromium has no native runtime; only Wails supplies this module.
			w.Header().Set("Content-Type", "text/javascript")
			return
		}
		handler.ServeHTTP(w, r)
	}))
	defer srv.Close()
	deadline, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	allocator, stopAllocator := chromedp.NewExecAllocator(deadline, append(chromedp.DefaultExecAllocatorOptions[:], chromedp.ExecPath(browser))...)
	defer stopAllocator()
	ctx, stopBrowser := chromedp.NewContext(allocator)
	defer stopBrowser()
	if err := chromedp.Do(ctx, chromedp.Navigate(srv.URL+"/")); err != nil {
		t.Fatal(err)
	}
	if _, err := chromedp.Run(ctx, chromedp.Poll[bool](`location.pathname==='/s/overview' && !window.EventSource && document.querySelector('.panel h2')?.textContent.includes('First publication') && !!document.querySelector('iframe:not(.frame-candidate)')`)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("# Second publication"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.Scan(); err != nil {
		t.Fatal(err)
	}
	if _, err := chromedp.Run(ctx, chromedp.Poll[bool](`document.querySelector('.panel h2')?.textContent.includes('Second publication') && !!document.querySelector('iframe:not(.frame-candidate)')`)); err != nil {
		t.Fatal(err)
	}
}
