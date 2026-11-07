package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/matryer/lcars/internal/content"
	"github.com/matryer/lcars/internal/server"
)

func fixture(t *testing.T) (*content.Store, http.Handler, string) {
	t.Helper()
	root := t.TempDir()
	tab := filepath.Join(root, "overview")
	if err := os.MkdirAll(filepath.Join(tab, "assets", "status"), 0700); err != nil {
		t.Fatal(err)
	}
	doc := `<!doctype html><html><head><title>House status</title></head><body><script src="assets/status/status.js"></script><output>Normal</output></body></html>`
	for p, b := range map[string]string{"status.html": doc, "assets/status/status.js": "window.fixtureRevision = 1;"} {
		if err := os.WriteFile(filepath.Join(tab, p), []byte(b), 0600); err != nil {
			t.Fatal(err)
		}
	}
	store, err := content.New(root)
	if err != nil {
		t.Fatal(err)
	}
	st := store.State()
	if len(st.Panels) != 1 || st.Panels[0].BundleHash == "" {
		t.Fatalf("fixture rejected: %+v", st)
	}
	return store, server.New(store), root
}

func request(handler http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080"+path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func TestStateETagAndPrivateCaching(t *testing.T) {
	_, handler, _ := fixture(t)
	response := request(handler, "/api/state", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("state status: %d %s", response.Code, response.Body.String())
	}
	etag := response.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing state ETag")
	}
	if !strings.Contains(response.Header().Get("Cache-Control"), "private") {
		t.Fatal("state must use private cache policy")
	}
	var state content.State
	if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Panels) != 1 || state.Panels[0].Title != "House status" {
		t.Fatalf("wrong state: %+v", state)
	}
	response = request(handler, "/api/state", map[string]string{"If-None-Match": etag})
	if response.Code != http.StatusNotModified || response.Body.Len() != 0 {
		t.Fatalf("unchanged state response: %d %s", response.Code, response.Body.String())
	}
}

func TestComponentIsolationAndImmutableResources(t *testing.T) {
	store, handler, root := fixture(t)
	panel := store.State().Panels[0]
	docURL := panel.URL
	response := request(handler, docURL+"?nonce=fixture", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("component status: %d %s", response.Code, response.Body.String())
	}
	csp := response.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"sandbox allow-scripts", "connect-src 'none'", "object-src 'none'", "frame-src 'none'"} {
		if !strings.Contains(csp, directive) {
			t.Fatalf("missing CSP %q: %s", directive, csp)
		}
	}
	body := response.Body.String()
	helper, author := strings.Index(body, "/toolkit/frame.js"), strings.Index(body, "assets/status/status.js")
	if helper < 0 || author < 0 || helper > author {
		t.Fatal("helper must initialize before author scripts")
	}
	if !strings.Contains(body, "lcars-content") {
		t.Fatal("missing intrinsic content wrapper")
	}
	if !strings.Contains(response.Header().Get("Cache-Control"), "private") {
		t.Fatal("component must remain private")
	}
	resourceURL := strings.TrimSuffix(docURL, "index.html") + "assets/status/status.js"
	response = request(handler, resourceURL, nil)
	if response.Code != 200 || response.Body.String() != "window.fixtureRevision = 1;" {
		t.Fatalf("resource: %d %q", response.Code, response.Body.String())
	}
	if csp := response.Header().Get("Content-Security-Policy"); !strings.HasPrefix(csp, "sandbox; default-src 'none'") {
		t.Fatalf("asset opened directly must be sandboxed: %q", csp)
	}
	if err := os.WriteFile(filepath.Join(root, "overview/assets/status/status.js"), []byte("window.fixtureRevision = 2;"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.Scan(); err != nil {
		t.Fatal(err)
	}
	response = request(handler, resourceURL, nil)
	if response.Body.String() != "window.fixtureRevision = 1;" {
		t.Fatal("asset-only edits mutated accepted bundle")
	}
	docPath := filepath.Join(root, "overview/status.html")
	doc, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(docPath, append(doc, []byte("<!-- publish -->")...), 0600); err != nil {
		t.Fatal(err)
	}
	if err = store.Scan(); err != nil {
		t.Fatal(err)
	}
	updated := store.State().Panels[0]
	if updated.BundleHash == panel.BundleHash {
		t.Fatal("document commit did not snapshot staged asset")
	}
	response = request(handler, strings.TrimSuffix(updated.URL, "index.html")+"assets/status/status.js", nil)
	if response.Code != 200 || response.Body.String() != "window.fixtureRevision = 2;" {
		t.Fatalf("new resource: %d %q", response.Code, response.Body.String())
	}
	response = request(handler, resourceURL, nil)
	if response.Code != 200 || response.Body.String() != "window.fixtureRevision = 1;" {
		t.Fatal("old resource unavailable during grace period")
	}
}

func TestRoutesAndCanonicalGuide(t *testing.T) {
	_, handler, _ := fixture(t)
	response := request(handler, "/", nil)
	if response.Code < 300 || response.Code >= 400 || response.Header().Get("Location") != "/s/overview" {
		t.Fatalf("root redirect: %d %s", response.Code, response.Header().Get("Location"))
	}
	for _, path := range []string{"/healthz", "/s/overview", "/s/deleted", "/toolkit/shell.css", "/toolkit/frame.js"} {
		response = request(handler, path, nil)
		if response.Code != http.StatusOK {
			t.Errorf("%s returned %d", path, response.Code)
		}
	}
	response = request(handler, "/agent-guide", nil)
	if response.Code != 200 {
		t.Fatalf("guide status: %d", response.Code)
	}
	canonical, err := os.ReadFile("../../skills/lcars-authoring/references/guide.md")
	if err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(response.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(canonical) {
		t.Fatal("served guide differs from canonical skill reference")
	}
	for _, path := range []string{"/api/screenshot", "/toolkit/../PLAN.md", "/component/no-such-bundle/overview/status/index.html", "/component/no-such-bundle/overview/status/assets/status/../../../../go.mod"} {
		response = request(handler, path, nil)
		if response.Code == http.StatusOK {
			t.Errorf("unexpected exposed route: %s", path)
		}
	}
}

func TestVersionDiscovery(t *testing.T) {
	_, handler, _ := fixture(t)
	response := request(handler, "/api/version", nil)
	var info struct {
		Version  string `json:"version"`
		Contract string `json:"contractVersion"`
		Protocol int    `json:"protocolVersion"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || info.Version == "" || info.Contract != "1.0" || info.Protocol != 1 {
		t.Fatalf("bad version discovery: %s", response.Body.String())
	}
}

func TestRejectsUnknownHosts(t *testing.T) {
	store, _, _ := fixture(t)
	handler := server.New(store, "lcars.example.ts.net")
	for host, want := range map[string]int{"127.0.0.1:8080": 200, "[::1]:8080": 200, "localhost:8080": 200, "100.64.0.1": 200, "lcars.example.ts.net": 200, "LCARS.example.ts.net:443": 200, "attacker.example:8080": 421, "localhost.attacker.example": 421} {
		req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
		req.Host = host
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		if response.Code != want {
			t.Errorf("Host %s returned %d, want %d", host, response.Code, want)
		}
	}
}

func TestSubSectionRoutesAndComponents(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "systems", "network", "assets", "latency")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "systems", "network", "latency.html"), []byte(`<title>Latency</title><script src="assets/latency/app.js"></script>`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("window.ok=1"), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := content.New(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(store)
	for path, want := range map[string]int{"/s/systems": 200, "/s/systems/network": 200, "/s/systems/network/deeper": 404, "/s/Systems": 404} {
		if got := request(handler, path, nil).Code; got != want {
			t.Errorf("%s = %d, want %d", path, got, want)
		}
	}
	url := store.State().Panels[0].URL
	if got := request(handler, url, nil); got.Code != 200 || !strings.Contains(got.Body.String(), "Latency") {
		t.Fatalf("component %s = %d", url, got.Code)
	}
	if got := request(handler, strings.Replace(url, "index.html", "assets/latency/app.js", 1), nil); got.Code != 200 || got.Body.String() != "window.ok=1" {
		t.Fatalf("component asset = %d %q", got.Code, got.Body.String())
	}
	if got := request(handler, strings.Replace(url, "systems~network~latency", "systems~network~latency~extra", 1), nil).Code; got != 404 {
		t.Fatalf("malformed component ID = %d, want 404", got)
	}
}
