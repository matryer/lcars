package server_test

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEventsInitialChangeAndReconnect(t *testing.T) {
	store, handler, root := fixture(t)
	srv := httptest.NewServer(handler)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	connect := func(last string) *http.Response {
		req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/events", nil)
		req.Header.Set("Last-Event-ID", last)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != 200 || res.Header.Get("Content-Type") != "text/event-stream" {
			res.Body.Close()
			t.Fatalf("not an SSE stream: %d", res.StatusCode)
		}
		if res.Header.Get("X-Accel-Buffering") != "no" || !strings.Contains(res.Header.Get("Cache-Control"), "private") {
			t.Fatal("unsafe buffering/cache headers")
		}
		return res
	}
	read := func(reader *bufio.Reader) string {
		t.Helper()
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				t.Fatal(err)
			}
			if strings.HasPrefix(line, "data: ") {
				return strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			}
		}
	}
	first := connect("")
	reader := bufio.NewReader(first.Body)
	initial := read(reader)
	if initial != store.State().Revision {
		t.Fatal("wrong initial revision")
	}
	os.WriteFile(filepath.Join(root, "overview/new.txt"), []byte("new"), 0600)
	store.Scan()
	if read(reader) != store.State().Revision {
		t.Fatal("missing change")
	}
	first.Body.Close()
	next := connect(initial)
	defer next.Body.Close()
	if read(bufio.NewReader(next.Body)) != store.State().Revision {
		t.Fatal("reconnect did not get current state")
	}
}
