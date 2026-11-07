// Package server serves the private shell and immutable isolated components.
package server

import (
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/matryer/lcars/internal/buildinfo"
	"github.com/matryer/lcars/internal/content"
	authoring "github.com/matryer/lcars/skills/lcars-authoring"
	"github.com/matryer/lcars/web"
)

// New serves store. Requests must name localhost, an IP address, or one of
// allowedHosts, which stops DNS-rebinding pages reading private content.
func New(store *content.Store, allowedHosts ...string) http.Handler {
	for i, h := range allowedHosts {
		allowedHosts[i] = strings.ToLower(h)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedHost(r.Host, allowedHosts) {
			http.Error(w, "Unrecognised Host; start lcars with --allowed-host", http.StatusMisdirectedRequest)
			return
		}
		w.Header().Set("Cache-Control", "private, no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", 405)
			return
		}
		p := r.URL.Path
		switch {
		case p == "/favicon.ico":
			w.WriteHeader(http.StatusNoContent)
		case p == "/healthz":
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintln(w, "ok")
		case p == "/api/events":
			events(w, r, store)
		case p == "/api/state":
			st := store.State()
			etag := `"` + st.Revision + `"`
			w.Header().Set("ETag", etag)
			if r.Header.Get("If-None-Match") == etag {
				w.WriteHeader(304)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(st)
		case p == "/api/version":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(buildinfo.Current())
		case p == "/agent-guide":
			serveFile(w, r, authoring.Files, "references/guide.md", "text/markdown; charset=utf-8")
		case p == "/":
			st := store.State()
			if len(st.Tabs) > 0 {
				http.Redirect(w, r, "/s/"+st.Tabs[0].ID, 302)
				return
			}
			shell(w, r)
		case strings.HasPrefix(p, "/s/"):
			// Sections and one level of sub-sections: /s/systems, /s/systems/network.
			if !content.ValidTabID(strings.TrimPrefix(p, "/s/")) {
				http.NotFound(w, r)
				return
			}
			shell(w, r)
		case strings.HasPrefix(p, "/component/"):
			parts := strings.Split(strings.TrimPrefix(p, "/component/"), "/")
			if len(parts) < 3 || len(parts[0]) != 64 {
				http.NotFound(w, r)
				return
			}
			id, ok := content.PanelIDFromSegment(parts[1])
			if !ok {
				http.NotFound(w, r)
				return
			}
			name := strings.Join(parts[2:], "/")
			if path.Clean(name) != name || strings.Contains(name, "\\") {
				http.Error(w, "Invalid resource path", 400)
				return
			}
			res, ok := store.Resource(parts[0], id, name)
			if !ok {
				http.Error(w, "Resource expired or unavailable. Retry latest.", 410)
				return
			}
			w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
			w.Header().Set("Content-Type", res.MIME)
			// 'self' rather than the Host header: proxies may rewrite Host.
			csp := "sandbox; default-src 'none'"
			if name == "index.html" {
				csp = "sandbox allow-scripts; default-src 'none'; script-src 'unsafe-inline' 'self'; style-src 'unsafe-inline' 'self'; img-src 'self'; font-src 'self'; connect-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'"
			}
			w.Header().Set("Content-Security-Policy", csp)
			_, _ = w.Write(res.Bytes)
		case strings.HasPrefix(p, "/toolkit/"):
			name := strings.TrimPrefix(p, "/toolkit/")
			switch name {
			case "shell.js":
				serveBytes(w, r, web.ShellScript(), "text/javascript; charset=utf-8")
			case "shell.css":
				serveBytes(w, r, web.ShellStyles(), "text/css; charset=utf-8")
			case "frame.css", "frame.js":
				serveFile(w, r, web.Files, "toolkit/"+name, "")
			case "Antonio.ttf":
				w.Header().Set("Access-Control-Allow-Origin", "*")
				serveFile(w, r, web.Files, "toolkit/fonts/Antonio.ttf", "font/ttf")
			default:
				http.NotFound(w, r)
			}
		default:
			http.NotFound(w, r)
		}
	})
}
func allowedHost(host string, allowed []string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.ToLower(strings.Trim(host, "[]")), ".")
	return host == "localhost" || net.ParseIP(host) != nil || slices.Contains(allowed, host)
}

// shellPage is shell.html with the build's version written into the footer.
var shellPage = sync.OnceValue(func() []byte {
	b, err := fs.ReadFile(web.Files, "shell.html")
	if err != nil {
		panic(err)
	}
	info := buildinfo.Current()
	label, link := "v"+info.Version, "https://github.com/matryer/lcars"
	title := "LCARS " + label + " · authoring contract " + info.ContractVersion
	if info.Commit != "" {
		short := info.Commit
		if len(short) > 7 {
			short = short[:7]
		}
		link += "/commit/" + url.PathEscape(info.Commit)
		title += " · commit " + short
		if info.Modified {
			title += " (modified)"
		}
	}
	if info.BuiltAt != "" {
		title += " · built " + info.BuiltAt
	}
	return []byte(strings.NewReplacer(
		"__LCARS_VERSION_URL__", html.EscapeString(link),
		"__LCARS_VERSION_TITLE__", html.EscapeString(title),
		"__LCARS_VERSION__", html.EscapeString(label),
	).Replace(string(b)))
})

func shell(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self'; font-src 'self'; frame-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'")
	serveBytes(w, r, shellPage(), "text/html; charset=utf-8")
}
func serveFile(w http.ResponseWriter, r *http.Request, files fs.FS, name, mime string) {
	b, e := fs.ReadFile(files, name)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	if mime == "" {
		switch path.Ext(name) {
		case ".css":
			mime = "text/css; charset=utf-8"
		case ".js":
			mime = "text/javascript; charset=utf-8"
		}
	}
	serveBytes(w, r, b, mime)
}
func serveBytes(w http.ResponseWriter, r *http.Request, b []byte, mime string) {
	w.Header().Set("Content-Type", mime)
	if r.Method != "HEAD" {
		_, _ = w.Write(b)
	}
}
