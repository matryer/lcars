package web

import (
	"bytes"
	"embed"
	"io/fs"
	"path"
	"slices"
	"sync"
)

// Files contains the shell page and the toolkit it serves at /toolkit/:
// the shell's script and style parts, the component toolkit (frame.js,
// frame.css) and the bundled font.
//
//go:embed shell.html toolkit/*.js toolkit/*.css toolkit/fonts/* toolkit/shell/*.js toolkit/shell/*.css
var Files embed.FS

// ShellScript returns the browser shell: every toolkit/shell/*.js part in
// filename order inside one strict-mode wrapper, so the parts share a single
// scope. The jsdom harness (tests/shell-harness.mjs) builds it the same way.
var ShellScript = built("*.js",
	"/* LCARS shell, bridge and capture contract v1. Built from web/toolkit/shell/*.js in filename order. */\n(() => {\n'use strict';\n",
	"\n})();\n")

// ShellStyles returns the shell stylesheet: every toolkit/shell/*.css part
// in filename order. Later parts win cascade ties, so order is significant.
var ShellStyles = built("*.css",
	"/* LCARS shell styles. Built from web/toolkit/shell/*.css in filename order. */\n", "")

func built(pattern, header, footer string) func() []byte {
	var once sync.Once
	var out []byte
	return func() []byte {
		once.Do(func() {
			names, err := fs.Glob(Files, "toolkit/shell/"+pattern)
			if err != nil {
				panic(err)
			}
			slices.Sort(names)
			var b bytes.Buffer
			b.WriteString(header)
			for _, name := range names {
				part, err := fs.ReadFile(Files, name)
				if err != nil {
					panic(err)
				}
				marker := "// --- shell/" + path.Base(name) + " ---\n"
				if path.Ext(name) == ".css" {
					marker = "/* --- shell/" + path.Base(name) + " --- */\n"
				}
				b.WriteString(marker)
				b.Write(part)
				if !bytes.HasSuffix(part, []byte("\n")) {
					b.WriteByte('\n')
				}
			}
			b.WriteString(footer)
			out = b.Bytes()
		})
		return out
	}
}
