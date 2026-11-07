package capture

import (
	"context"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHashOptionsRequirePanel(t *testing.T) {
	o := Options{URL: "http://example.test", Width: 390, Height: 844, Out: "capture.png", ExpectDataHash: "missing"}
	if err := o.validate(); err == nil || !strings.Contains(err.Error(), "require --panel") {
		t.Fatalf("expected panel requirement, got %v", err)
	}
	o.Panel = "overview/status"
	if err := o.validate(); err != nil {
		t.Fatal(err)
	}
	o.ExpectPanelHash = "wrong"
	if err := o.validate(); err == nil {
		t.Fatal("accepted invalid document hash")
	}
}

func TestMissingBrowserWritesFailureReport(t *testing.T) {
	out := filepath.Join(t.TempDir(), "capture.png")
	_, err := Run(context.Background(), Options{URL: "http://example.test", Browser: "/no/such/lcars-browser", Width: 390, Height: 844, Out: out})
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("got %v", err)
	}
	b, err := os.ReadFile(strings.TrimSuffix(out, ".png") + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		Errors     []string `json:"errors"`
		PNGWritten bool     `json:"pngWritten"`
	}
	if err = json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Errors) != 1 || r.PNGWritten {
		t.Fatalf("wrong failure report: %s", b)
	}
}

func TestBrowserCaptureAndVerificationFailure(t *testing.T) {
	browser, err := browserPath("")
	if err != nil {
		t.Skip(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><body><article data-panel-id="overview/status">Ready</article><script>
window.LCARSCapture=async o=>({panels:[{id:'overview/status',docHash:'test-document',dataHash:'missing'}],errors:o.expectDataHash&&o.expectDataHash!=='missing'?['data hash mismatch']:location.search?['deliberate failure']:[],overflow:[]});
</script></body></html>`))
	}))
	defer srv.Close()
	out := filepath.Join(t.TempDir(), "capture.png")
	opts := Options{URL: srv.URL, Browser: browser, Width: 390, Height: 844, Out: out}
	report, err := Run(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if report["browserVersion"] == nil {
		t.Fatal("missing browser version")
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	config, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != 390 || config.Height != 844 {
		t.Fatalf("wrong viewport PNG: %+v", config)
	}
	opts.URL = srv.URL + "/?failure"
	opts.Panel = "overview/status"
	opts.Out = filepath.Join(t.TempDir(), "failure.png")
	_, err = Run(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "deliberate failure") {
		t.Fatalf("expected verification failure, got %v", err)
	}
	if _, err = os.Stat(opts.Out); err != nil {
		t.Fatalf("failure PNG missing: %v", err)
	}
	opts.URL = srv.URL
	opts.ExpectDataHash = strings.Repeat("0", 64)
	_, err = Run(context.Background(), opts)
	if err == nil || !strings.Contains(err.Error(), "data hash mismatch") {
		t.Fatalf("expected forwarded hash check failure, got %v", err)
	}
}
