package web

import "embed"

// Files contains the shell and version-one component toolkit.
//
//go:embed *.html *.js *.css fonts/*
var Files embed.FS
