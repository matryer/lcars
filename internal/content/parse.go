package content

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"golang.org/x/net/html"
)

var idPattern = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)
var palette = map[string]bool{"apricot": true, "orange": true, "lilac": true, "blue": true, "peach": true, "critical": true}
var resourceTypes = map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp", ".svg": "image/svg+xml", ".js": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8", ".json": "application/json", ".woff2": "font/woff2", ".ttf": "font/ttf"}

func validID(s string) bool { return idPattern.MatchString(s) }

// ComponentSegment encodes a panel ID as one URL path segment, so a panel's
// relative asset references resolve beneath it at any section depth. "~"
// cannot appear in IDs.
func ComponentSegment(id string) string { return strings.ReplaceAll(id, "/", "~") }

// PanelIDFromSegment reverses ComponentSegment, rejecting malformed IDs.
func PanelIDFromSegment(segment string) (string, bool) {
	parts := strings.Split(segment, "~")
	if len(parts) < 2 || len(parts) > 3 {
		return "", false
	}
	for _, part := range parts {
		if !validID(part) {
			return "", false
		}
	}
	return strings.Join(parts, "/"), true
}

// ValidTabID reports whether id names a section or a one-level sub-section.
func ValidTabID(id string) bool {
	parts := strings.Split(id, "/")
	if len(parts) > 2 {
		return false
	}
	for _, part := range parts {
		if !validID(part) {
			return false
		}
	}
	return true
}
func label(s string) string { return strings.ReplaceAll(s, "-", " ") }
func readLimited(r io.Reader, n int) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, int64(n+1)))
	if e == nil && len(b) > n {
		e = fmt.Errorf("file exceeds %d byte limit", n)
	}
	return b, e
}
func strictJSON(b []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return e
	}
	if e := d.Decode(new(any)); e != io.EOF {
		return fmt.Errorf("expected exactly one JSON value")
	}
	return nil
}
func observation(ts string, seconds int) error {
	if seconds < 0 {
		return fmt.Errorf("staleAfterSeconds must be positive")
	}
	if seconds > 0 && ts == "" {
		return fmt.Errorf("staleAfterSeconds requires updatedAt")
	}
	if ts != "" {
		if _, e := time.Parse(time.RFC3339, ts); e != nil {
			return fmt.Errorf("updatedAt must be RFC3339")
		}
	}
	return nil
}
func textOf(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.WriteString(textOf(c))
	}
	return b.String()
}
func attribute(n *html.Node, k string) string {
	for _, a := range n.Attr {
		if a.Key == k {
			return a.Val
		}
	}
	return ""
}
func each(n *html.Node, f func(*html.Node) error) error {
	if e := f(n); e != nil {
		return e
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if e := each(c, f); e != nil {
			return e
		}
	}
	return nil
}
func parsePanel(id, filename string, raw []byte, modified time.Time) (accepted, error) {
	tab, stem := splitPanelID(id)
	kind := strings.TrimPrefix(filepath.Ext(filename), ".")
	p := Panel{ID: id, Tab: tab, Title: label(stem), Kind: kind, Size: "normal", Accent: "lilac", Order: 100, DocHash: Hash(raw), Diagnostics: []string{}, Data: Data{Hash: "missing", Missing: true}}
	assets, mt, e := snapshotAssets(filepath.Dir(filename), stem)
	if e != nil {
		return accepted{}, e
	}
	if mt.After(modified) {
		modified = mt
	}
	var doc *html.Node
	if kind == "html" {
		doc, e = html.Parse(bytes.NewReader(raw))
		if e != nil {
			return accepted{}, e
		}
		seen := map[string]bool{}
		e = each(doc, func(n *html.Node) error {
			if n.Type != html.ElementNode {
				return nil
			}
			if n.Data == "title" && strings.TrimSpace(textOf(n)) != "" {
				p.Title = strings.TrimSpace(textOf(n))
			}
			if n.Data != "meta" {
				return nil
			}
			name := attribute(n, "name")
			if !strings.HasPrefix(name, "lcars:") {
				return nil
			}
			if seen[name] {
				return fmt.Errorf("duplicate metadata: %s", name)
			}
			seen[name] = true
			v := attribute(n, "content")
			switch name {
			case "lcars:size":
				if v != "normal" && v != "wide" && v != "full" {
					return fmt.Errorf("invalid lcars:size")
				}
				p.Size = v
			case "lcars:order":
				p.Order, e = strconv.Atoi(v)
				return e
			case "lcars:accent":
				if !palette[v] {
					return fmt.Errorf("invalid lcars:accent")
				}
				p.Accent = v
			case "lcars:updatedAt":
				p.ObservedAt = v
			case "lcars:staleAfterSeconds":
				p.StaleAfterSeconds, e = strconv.Atoi(v)
				if e != nil || p.StaleAfterSeconds <= 0 {
					return fmt.Errorf("lcars:staleAfterSeconds must be positive")
				}
			case "lcars:sourceUrl":
				u, err := url.Parse(v)
				if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
					return fmt.Errorf("sourceUrl must be HTTP(S)")
				}
				p.SourceURL = v
			default:
				return fmt.Errorf("unrecognized metadata: %s", name)
			}
			return nil
		})
		if e != nil {
			return accepted{}, e
		}
		if e = observation(p.ObservedAt, p.StaleAfterSeconds); e != nil {
			return accepted{}, e
		}
	} else {
		var rendered bytes.Buffer
		if kind == "md" {
			if e = goldmark.Convert(raw, &rendered); e != nil {
				return accepted{}, e
			}
		} else {
			rendered.WriteString("<pre class=lcars-plain>")
			rendered.WriteString(html.EscapeString(string(raw)))
			rendered.WriteString("</pre>")
		}
		doc, e = html.Parse(&rendered)
		if e != nil {
			return accepted{}, e
		}
		var first *html.Node
		_ = each(doc, func(n *html.Node) error {
			if n.Type == html.ElementNode && n.Data == "h1" && first == nil {
				first = n
			}
			return nil
		})
		if kind == "md" && first != nil {
			p.Title = textOf(first)
			first.Parent.RemoveChild(first)
		}
	}
	if len(p.Title) > 1024 {
		return accepted{}, fmt.Errorf("title exceeds 1024 bytes")
	}
	if kind == "md" {
		_ = each(doc, func(n *html.Node) error {
			if n.Type == html.ElementNode && n.Data == "a" {
				u, e := url.Parse(attribute(n, "href"))
				if e == nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https") {
					p.Sources = append(p.Sources, Source{Title: textOf(n), URL: u.String()})
				}
			}
			return nil
		})
	}
	if e = validateReferences(doc, assets, stem, kind != "html"); e != nil {
		return accepted{}, e
	}
	for name, r := range assets {
		if strings.HasSuffix(name, ".css") {
			if e = validateCSS(string(r.Bytes), name, assets, stem); e != nil {
				return accepted{}, e
			}
		}
	}
	// Hash includes original bytes and sorted resource names/hashes, never file times.
	var digest strings.Builder
	digest.WriteString(id + "\x00" + p.DocHash)
	names := make([]string, 0, len(assets))
	for n := range assets {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		digest.WriteString("\x00" + n + "\x00" + assets[n].Hash)
	}
	bundleHash := Hash([]byte(digest.String()))
	p.BundleHash = bundleHash
	p.UpdatedAt = modified
	p.URL = "/component/" + bundleHash + "/" + ComponentSegment(id) + "/index.html"
	var head, body *html.Node
	_ = each(doc, func(n *html.Node) error {
		if n.Type == html.ElementNode && n.Data == "head" {
			head = n
		}
		if n.Type == html.ElementNode && n.Data == "body" {
			body = n
		}
		return nil
	})
	style := &html.Node{Type: html.ElementNode, Data: "link", Attr: []html.Attribute{{Key: "rel", Val: "stylesheet"}, {Key: "href", Val: "/toolkit/frame.css"}}}
	script := &html.Node{Type: html.ElementNode, Data: "script", Attr: []html.Attribute{{Key: "src", Val: "/toolkit/frame.js"}}}
	head.InsertBefore(script, head.FirstChild)
	head.InsertBefore(style, head.FirstChild)
	wrapper := &html.Node{Type: html.ElementNode, Data: "div", Attr: []html.Attribute{{Key: "class", Val: "lcars-content"}}}
	for body.FirstChild != nil {
		c := body.FirstChild
		body.RemoveChild(c)
		wrapper.AppendChild(c)
	}
	body.AppendChild(wrapper)
	var normalized bytes.Buffer
	if e = html.Render(&normalized, doc); e != nil {
		return accepted{}, e
	}
	return accepted{Panel: p, Bundle: &Bundle{ID: id, Hash: bundleHash, HTML: normalized.Bytes(), Assets: assets, UpdatedAt: modified}, path: filename}, nil
}
func localReference(ref, base, stem string, assets map[string]Resource) error {
	if ref == "" || strings.HasPrefix(ref, "#") {
		return nil
	}
	u, e := url.Parse(ref)
	if e != nil {
		return e
	}
	if u.IsAbs() || u.Host != "" || strings.HasPrefix(ref, "/") || strings.Contains(ref, "\\") {
		return fmt.Errorf("external or absolute resource refused: %s", ref)
	}
	clean := path.Clean(path.Join(path.Dir(base), u.Path))
	if !strings.HasPrefix(clean, "assets/"+stem+"/") {
		return fmt.Errorf("resource must be in assets/%s/: %s", stem, ref)
	}
	if _, ok := assets[clean]; !ok {
		return fmt.Errorf("missing required asset: %s", clean)
	}
	return nil
}

var cssURL = regexp.MustCompile(`(?i)url\(\s*['"]?([^'"\)]+)['"]?\s*\)`)

func validateCSS(css, base string, assets map[string]Resource, stem string) error {
	if strings.Contains(strings.ToLower(css), "@import") {
		return fmt.Errorf("CSS @import unsupported; use a local stylesheet link")
	}
	for _, m := range cssURL.FindAllStringSubmatch(css, -1) {
		if e := localReference(strings.TrimSpace(m[1]), base, stem, assets); e != nil {
			return e
		}
	}
	return nil
}
func validateReferences(doc *html.Node, assets map[string]Resource, stem string, prose bool) error {
	return each(doc, func(n *html.Node) error {
		if n.Type != html.ElementNode {
			return nil
		}
		switch n.Data {
		case "base", "iframe", "object", "embed", "audio", "video", "source":
			return fmt.Errorf("unsupported element: %s", n.Data)
		}
		for _, a := range n.Attr {
			if a.Key == "style" {
				if e := validateCSS(a.Val, "index.html", assets, stem); e != nil {
					return e
				}
			}
			if a.Key == "srcset" {
				return fmt.Errorf("srcset unsupported; use responsive CSS and src")
			}
			if a.Key == "src" || (n.Data == "link" && a.Key == "href") || (n.Data == "use" && (a.Key == "href" || a.Key == "xlink:href")) {
				if e := localReference(a.Val, "index.html", stem, assets); e != nil {
					return e
				}
			}
		}
		if n.Data == "script" && (attribute(n, "type") == "module") {
			return fmt.Errorf("use classic scripts, not modules")
		}
		if n.Data == "style" {
			if e := validateCSS(textOf(n), "index.html", assets, stem); e != nil {
				return e
			}
		}
		if n.Data == "a" {
			ref := attribute(n, "href")
			u, e := url.Parse(ref)
			if e != nil || (u.Scheme != "" && u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "mailto") {
				return fmt.Errorf("unsafe link")
			}
			if prose {
				n.Attr = append(n.Attr, html.Attribute{Key: "target", Val: "_blank"}, html.Attribute{Key: "rel", Val: "noopener noreferrer"})
			}
		}
		if prose && n.Data == "img" {
			if strings.TrimSpace(attribute(n, "alt")) == "" {
				return fmt.Errorf("local images require descriptive alt text")
			}
			ext := strings.ToLower(path.Ext(attribute(n, "src")))
			if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" && ext != ".svg" {
				return fmt.Errorf("unsupported prose image type")
			}
		}
		return nil
	})
}

var svgTags = strings.Fields("svg g path rect circle ellipse line polyline polygon text tspan title desc defs clipPath mask linearGradient radialGradient stop")
var svgAttrs = strings.Fields("xmlns viewBox width height x y x1 x2 y1 y2 cx cy r rx ry d points fill fill-opacity fill-rule stroke stroke-width stroke-opacity stroke-linecap stroke-linejoin stroke-dasharray opacity transform font-family font-size font-weight text-anchor dominant-baseline id class role aria-label aria-labelledby preserveAspectRatio offset stop-color stop-opacity gradientUnits gradientTransform clip-path mask")

func passiveSVG(b []byte) error {
	tags := map[string]bool{}
	attrs := map[string]bool{}
	for _, v := range svgTags {
		tags[v] = true
	}
	for _, v := range svgAttrs {
		attrs[v] = true
	}
	d := xml.NewDecoder(bytes.NewReader(b))
	root := false
	for {
		t, e := d.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		switch x := t.(type) {
		case xml.StartElement:
			if !root {
				if x.Name.Local != "svg" {
					return fmt.Errorf("SVG root required")
				}
				root = true
			}
			if !tags[x.Name.Local] {
				return fmt.Errorf("SVG element refused: %s", x.Name.Local)
			}
			for _, a := range x.Attr {
				if !attrs[a.Name.Local] && a.Name.Space != "xmlns" {
					return fmt.Errorf("SVG attribute refused: %s", a.Name.Local)
				}
				if strings.Contains(strings.ToLower(a.Value), "url(") {
					m := cssURL.FindStringSubmatch(a.Value)
					if len(m) < 2 || !strings.HasPrefix(m[1], "#") {
						return fmt.Errorf("SVG external reference refused")
					}
				}
			}
		case xml.Directive:
			return fmt.Errorf("SVG directives refused")
		case xml.ProcInst:
			if x.Target != "xml" {
				return fmt.Errorf("SVG processing instructions refused")
			}
		}
	}
	if !root {
		return fmt.Errorf("empty SVG")
	}
	return nil
}
