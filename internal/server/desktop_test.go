package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matryer/lcars/internal/server"
)

func TestDesktopCanvas(t *testing.T) {
	store, browser, root := fixture(t)
	desktop := server.NewDesktop(store)
	get := func(handler http.Handler, host, path string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest("GET", "http://"+host+path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	for _, host := range []string{"localhost", "wails.localhost"} {
		w := get(desktop, host, "/")
		if w.Code != http.StatusOK || w.Header().Get("Location") != "" {
			t.Fatalf("%s: desktop root must load without a custom-scheme redirect: %d", host, w.Code)
		}
		html := w.Body.String()
		bootstrap := strings.Index(html, `src="/desktop.js"`)
		shell := strings.Index(html, `src="/toolkit/shell.js"`)
		if bootstrap < 0 || shell < bootstrap || !strings.Contains(html, `type="module" src="/wails/runtime.js"`) {
			t.Fatal("desktop runtime and bootstrap must load before the canvas")
		}
		if strings.Contains(w.Header().Get("Content-Security-Policy"), "unsafe-inline") {
			t.Fatal("desktop must retain the shell's strict CSP")
		}
		js := get(desktop, host, "/desktop.js")
		if js.Code != 200 || !strings.Contains(js.Body.String(), "window.EventSource=undefined") || !strings.Contains(js.Body.String(), "/s/overview") {
			t.Fatalf("missing polling or initial section: %s", js.Body.String())
		}
	}
	for _, path := range []string{"/", "/desktop.js", "/api/state"} {
		if w := get(desktop, "attacker.invalid", path); w.Code != http.StatusMisdirectedRequest {
			t.Fatalf("host guard bypass at %s: %d", path, w.Code)
		}
	}
	if w := get(desktop, "localhost", "/api/events"); w.Code != http.StatusNotImplemented {
		t.Fatal("native events endpoint must not open an unflushable stream")
	}
	if w := get(browser, "localhost", "/"); w.Code != http.StatusFound {
		t.Fatal("browser root redirect changed")
	}
	if w := get(browser, "localhost", "/s/overview"); strings.Contains(w.Body.String(), "/wails/") || strings.Contains(w.Body.String(), "/desktop.js") {
		t.Fatal("browser must not load the native runtime")
	}
	if w := get(browser, "localhost", "/desktop.js"); w.Code != http.StatusNotFound {
		t.Fatal("desktop bootstrap exposed in browser mode")
	}
	panel := store.State().Panels[0]
	w := get(desktop, "localhost", panel.URL)
	if w.Code != 200 || strings.Contains(w.Body.String(), "/wails/runtime.js") || !strings.HasPrefix(w.Header().Get("Content-Security-Policy"), "sandbox allow-scripts;") {
		t.Fatal("desktop components must retain their sandbox without the native runtime")
	}
	before := get(desktop, "localhost", "/api/state").Header().Get("ETag")
	if err := os.WriteFile(filepath.Join(root, "overview", "desktop.txt"), []byte("Updated from disk"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.Scan(); err != nil {
		t.Fatal(err)
	}
	w = get(desktop, "localhost", "/api/state")
	if w.Header().Get("ETag") == before || !strings.Contains(w.Body.String(), "overview/desktop") {
		t.Fatal("desktop polling must see published file changes")
	}
}
