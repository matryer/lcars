package content

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScriptDiscovery(t *testing.T) {
	for _, tc := range []struct {
		name string
		good bool
	}{
		{"weather.5m.py", true}, {"weather.1d.js", true}, {"weather.0s.sh", false},
		{"weather.999999999999999999h.py", false}, {"weather.soon.py", false},
		{"missing.1m.py", false}, {"weather.1.5m.py", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "overview")
			os.Mkdir(dir, 0700)
			os.WriteFile(filepath.Join(dir, "weather.html"), []byte("<title>Weather</title>"), 0600)
			os.WriteFile(filepath.Join(dir, tc.name), []byte("never execute during validation"), 0600)
			jobs, diags := DiscoverScripts(root)
			if tc.good {
				if len(jobs) != 1 || len(diags) != 0 {
					t.Fatalf("jobs=%v diagnostics=%v", jobs, diags)
				}
			} else if len(jobs) != 0 || len(diags) == 0 {
				t.Fatalf("jobs=%v diagnostics=%v", jobs, diags)
			}
		})
	}
}
func TestDuplicateAndDisabledScripts(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "overview")
	os.Mkdir(dir, 0700)
	for _, name := range []string{"weather.html", "weather.5m.py", "weather.1h.js", ".weather.1s.sh", "helper.py"} {
		os.WriteFile(filepath.Join(dir, name), []byte(""), 0600)
	}
	jobs, diags := DiscoverScripts(root)
	if len(jobs) != 0 || len(diags) != 1 {
		t.Fatalf("%v %v", jobs, diags)
	}
	os.Remove(filepath.Join(dir, "weather.1h.js"))
	jobs, diags = DiscoverScripts(root)
	if len(jobs) != 1 || jobs[0].Every != 5*time.Minute || len(diags) != 0 {
		t.Fatalf("%v %v", jobs, diags)
	}
}
func TestStateSubscriptionCoalesces(t *testing.T) {
	root := t.TempDir()
	s, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	events, closeSub := s.Subscribe()
	defer closeSub()
	initial := <-events
	s.Scan()
	select {
	case <-events:
		t.Fatal("unchanged scan notified")
	default:
	}
	os.Mkdir(filepath.Join(root, "overview"), 0700)
	s.Scan()
	os.WriteFile(filepath.Join(root, "overview/a.txt"), []byte("hello"), 0600)
	s.Scan()
	select {
	case latest := <-events:
		if latest == initial || latest != s.State().Revision {
			t.Fatal("not latest revision")
		}
	case <-time.After(time.Second):
		t.Fatal("no notification")
	}
}
func TestEnvelopeValidation(t *testing.T) {
	for _, input := range []string{`{"version":1,"data":null}`, "{\"version\":1,\"data\":{}}\n"} {
		if _, err := ValidateEnvelope([]byte(input)); err != nil {
			t.Fatal(err)
		}
	}
	for _, input := range []string{`{"version":1}`, `{"version":1,"data":{}} {}`, `log\n{"version":1,"data":{}}`, `{"version":1,"data":{},"staleAfterSeconds":0}`, string([]byte{'"', 255, '"'})} {
		if _, err := ValidateEnvelope([]byte(input)); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
}
