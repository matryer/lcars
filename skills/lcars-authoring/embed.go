// Package authoring contains the portable LCARS authoring contract and starter.
package authoring

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

// Files holds the canonical guide and starter served by the application.
//
//go:embed SKILL.md references/guide.md assets/starter.html
var Files embed.FS

// Export writes a complete portable skill to a new directory, never overwriting
// an existing installation. Installation remains the caller's explicit choice.
func Export(destination string) (err error) {
	if err = os.Mkdir(destination, 0755); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(destination)
		}
	}()
	return fs.WalkDir(Files, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		target := filepath.Join(destination, filepath.FromSlash(name))
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		b, err := Files.ReadFile(name)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0644)
	})
}
