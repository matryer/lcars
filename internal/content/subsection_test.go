package content_test

import (
	"strings"
	"testing"

	"github.com/matryer/lcars/internal/content"
)

func TestSubSections(t *testing.T) {
	root := t.TempDir()
	write(t, root, "systems/_section.json", `{"version":1,"title":"Systems","order":5}`)
	write(t, root, "systems/hosts.md", "# Hosts\n\nFour.")
	write(t, root, "systems/network/_section.json", `{"version":1,"title":"Network","order":20,"accent":"blue"}`)
	write(t, root, "systems/network/latency.html", `<title>Latency</title><img src="assets/latency/key.png" alt="Key">`)
	write(t, root, "systems/network/assets/latency/key.png", "\x89PNG\r\n\x1a\n")
	write(t, root, "systems/network/assets/latency/data.json", `{"version":1,"data":{"ms":12}}`)
	write(t, root, "systems/network/latency.5m.py", "print('{}')")
	write(t, root, "systems/storage/volumes.md", "# Volumes")
	write(t, root, "systems/storage/deeper/ignored.md", "# Too deep")
	st := openStore(t, root).State()

	tabs := map[string]content.Tab{}
	for _, tab := range st.Tabs {
		tabs[tab.ID] = tab
	}
	if len(tabs) != 3 || tabs["systems"].Parent != "" || tabs["systems/network"].Parent != "systems" || tabs["systems/network"].Title != "Network" || tabs["systems/network"].Accent != "blue" || tabs["systems/storage"].Title != "storage" {
		t.Fatalf("tabs = %+v", st.Tabs)
	}
	panels := map[string]content.Panel{}
	for _, p := range st.Panels {
		panels[p.ID] = p
	}
	latency, ok := panels["systems/network/latency"]
	if !ok || latency.Tab != "systems/network" || latency.Data.Missing || len(latency.Diagnostics) != 0 {
		t.Fatalf("latency = %+v", latency)
	}
	if !strings.Contains(latency.URL, "/systems~network~latency/index.html") {
		t.Fatalf("component URL = %s", latency.URL)
	}
	if id, ok := content.PanelIDFromSegment("systems~network~latency"); !ok || id != "systems/network/latency" {
		t.Fatalf("segment decode = %q %v", id, ok)
	}
	if panels["systems/hosts"].Tab != "systems" || panels["systems/storage/volumes"].Tab != "systems/storage" {
		t.Fatalf("panels = %+v", st.Panels)
	}
	if _, ok := panels["systems/storage/deeper/ignored"]; ok {
		t.Fatal("folders deeper than one sub-section must not create panels")
	}
	if !strings.Contains(strings.Join(st.Diagnostics, "\n"), "Warning: unsupported nested folder (sub-sections are one level deep): systems/storage/deeper") {
		t.Fatalf("diagnostics = %v", st.Diagnostics)
	}
	scripts, diags := content.DiscoverScripts(root)
	if len(diags) != 0 || len(scripts) != 1 || scripts[0].ID != "systems/network/latency" {
		t.Fatalf("scripts = %+v %v", scripts, diags)
	}
	for id, want := range map[string]bool{"systems": true, "systems/network": true, "systems/network/x": false, "Systems": false, "": false, "systems/": false} {
		if content.ValidTabID(id) != want {
			t.Errorf("ValidTabID(%q) = %v", id, !want)
		}
	}
}
