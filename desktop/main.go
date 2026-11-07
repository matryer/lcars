// lcars-desktop opens the LCARS canvas in a native Wails window.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/matryer/lcars/internal/content"
	"github.com/matryer/lcars/internal/scripts"
	"github.com/matryer/lcars/internal/server"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, "lcars-desktop:", err)
		os.Exit(1)
	}
}

func run(args []string, errOut io.Writer) error {
	flags := flag.NewFlagSet("lcars-desktop", flag.ContinueOnError)
	flags.SetOutput(errOut)
	data := flags.String("data", "", "existing content directory (default: user's config directory/lcars/content)")
	runScripts := flags.Bool("run-scripts", false, "execute trusted adjacent scheduled scripts")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected arguments; use --data DIR to open a content directory")
	}
	root, err := dataDirectory(*data)
	if err != nil {
		return err
	}
	store, err := content.New(root)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	var runner *scripts.Runner
	if *runScripts {
		runner, err = scripts.New(root, store, scripts.Options{Timeout: 30 * time.Second, Concurrency: 4, Log: errOut})
		if err != nil {
			cancel()
			return err
		}
	}
	start := func(fn func(context.Context)) {
		workers.Add(1)
		go func() { defer workers.Done(); fn(ctx) }()
	}
	start(store.Run)
	if runner != nil {
		start(runner.Run)
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			workers.Wait()
			if runner != nil {
				runner.Close()
			}
		})
	}
	defer stop()

	app := application.New(application.Options{
		Name: "LCARS", Description: "Your files. A starship interface.",
		Assets:     application.AssetOptions{Handler: server.NewDesktop(store)},
		Mac:        application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		OnShutdown: stop,
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "LCARS — " + root, Width: 1440, Height: 900,
		MinWidth: 320, MinHeight: 480,
		BackgroundColour: application.NewRGB(0, 0, 0),
		URL:              "/",
	})
	return app.Run()
}

// An explicit directory must already exist. Only the persistent default is
// created, so a typo cannot silently open an empty canvas elsewhere.
func dataDirectory(explicit string) (string, error) {
	if explicit != "" {
		return filepath.Abs(explicit)
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find default content directory: %w", err)
	}
	root := filepath.Join(config, "lcars", "content")
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", fmt.Errorf("create default content directory: %w", err)
	}
	return root, nil
}
