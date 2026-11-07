package content

import (
	"testing"
	"time"
)

func TestGraceExpiryDoesNotExposeExpiredBytes(t *testing.T) {
	s := &Store{current: map[string]accepted{}}
	expired := &Bundle{ID: "overview/card", Hash: "expired", HTML: []byte("old")}
	live := &Bundle{ID: "overview/card", Hash: "live", HTML: []byte("recent")}
	s.grace = []retired{
		{bundle: expired, until: time.Now().Add(-time.Second)},
		{bundle: live, until: time.Now().Add(time.Minute)},
	}
	// Lookups enforce expiry even before the next scanner tick.
	if _, ok := s.Resource("expired", "overview/card", "index.html"); ok {
		t.Fatal("expired bundle remained readable")
	}
	if _, ok := s.Resource("live", "overview/another", "index.html"); ok {
		t.Fatal("grace bundle was exposed under another panel ID")
	}
	s.expire()
	if len(s.grace) != 1 || s.grace[0].bundle.Hash != "live" {
		t.Fatal("expiry did not remove only expired resources")
	}
	if r, ok := s.Resource("live", "overview/card", "index.html"); !ok || string(r.Bytes) != "recent" {
		t.Fatal("unexpired grace bundle unavailable")
	}
}

func TestGraceBudgetEvictsOldestBeforeNewest(t *testing.T) {
	// The cache counts resource bytes, including the HTML document. Reuse one
	// allocation here; each retired bundle still represents its own stored bytes.
	bytes := make([]byte, MaxAssets/2+1)
	s := &Store{current: map[string]accepted{}}
	s.retire(&Bundle{ID: "overview/card", Hash: "first", HTML: bytes})
	s.retire(&Bundle{ID: "overview/card", Hash: "second", HTML: bytes})
	if len(s.grace) != 1 || s.grace[0].bundle.Hash != "second" {
		t.Fatal("grace cache did not evict the oldest bundle at its byte limit")
	}
}

func TestCurrentAssetBudgetDeduplicatesAcceptedContent(t *testing.T) {
	resource := Resource{Hash: "same-bytes", Bytes: []byte("shared")}
	unique := Resource{Hash: "unique", Bytes: []byte("other")}
	s := &Store{current: map[string]accepted{
		"overview/a": {Bundle: &Bundle{Assets: map[string]Resource{"a.png": resource}}},
		"overview/b": {Bundle: &Bundle{Assets: map[string]Resource{"b.png": resource}}},
	}}
	candidate := &Bundle{Assets: map[string]Resource{"c.png": unique}}
	if got, want := s.assetBudget("overview/c", candidate), len(resource.Bytes)+len(unique.Bytes); got != want {
		t.Fatalf("unique resource budget = %d, want %d", got, want)
	}
	// Replacing an accepted panel must release its former allocation first.
	s.current["overview/b"] = accepted{Bundle: &Bundle{Assets: map[string]Resource{"b.png": unique}}}
	if got, want := s.assetBudget("overview/b", &Bundle{}), len(resource.Bytes); got != want {
		t.Fatalf("replacement budget = %d, want %d", got, want)
	}
}
