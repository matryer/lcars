package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/matryer/lcars/internal/content"
)

func events(w http.ResponseWriter, r *http.Request, store *content.Store) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "private, no-cache, no-transform")
	w.Header().Set("X-Accel-Buffering", "no")
	if r.Method == http.MethodHead {
		return
	}
	revisions, unsubscribe := store.Subscribe()
	defer unsubscribe()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	controller := http.NewResponseController(w)
	write := func(message string) bool {
		// Bound writes to clients that stop reading. Other routes retain normal deadlines.
		_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if _, err := fmt.Fprint(w, message); err != nil {
			return false
		}
		return controller.Flush() == nil
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case revision := <-revisions:
			if !write("id: " + revision + "\nevent: state\ndata: " + revision + "\n\n") {
				return
			}
		case <-heartbeat.C:
			if !write(": heartbeat\n\n") {
				return
			}
		}
	}
}
